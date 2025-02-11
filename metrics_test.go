package notdiamond

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func must(t *testing.T, err error) {
	if err != nil {
		t.Error(err)
	}
}

// setupTempDB sets the innerDB.DataFolder to a unique temporary directory for the test.
func setupTempDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	absDir, err := filepath.Abs(tmpDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	dataFolder = absDir
}

func TestMetricsTracker_RecordAndHealth(t *testing.T) {
	setupTempDB(t)
	model := "openai/gpt-4o"
	metrics, err := newMetricsTracker(":memory:" + t.Name())
	if err != nil {
		logger.Fatalf("Failed to open database connection: %v", err)
	}
	// Record several latencies. For example, 1, 2, 3, and 4 seconds.
	latencies := []float64{1.0, 2.0, 3.0, 4.0} // average = 2.5 seconds
	for _, l := range latencies {
		err := metrics.recordLatency(model, l, "s")
		if err != nil {
			t.Errorf("recordLatency error: %v", err)
		}
	}

	// Use thresholds: average_latency threshold = 3.0 sec, no_of_calls = 10, recovery_time = 10 minutes.
	config := &Config{
		ModelLatency: ModelLatency{
			model: &RollingAverageLatency{
				NoOfCalls:           10,
				RecoveryTime:        10 * time.Minute,
				AvgLatencyThreshold: 3.0,
			},
		},
	}

	err = metrics.checkModelHealth(model, "s", config)
	if err != nil {
		t.Errorf("Expected model %q to be healthy (avg=2.5 < threshold=3.0)", model)
	}

	// Record two high latency calls (e.g. 10 seconds each), which should push the average above the threshold.
	highLatencies := []float64{10.0, 10.0}
	for _, l := range highLatencies {
		err := metrics.recordLatency(model, l, "s")
		if err != nil {
			t.Errorf("recordLatency error: %v", err)
		}
	}

	// Use thresholds: average_latency threshold = 3.0 sec, no_of_calls = 10, recovery_time = 10 minutes.
	config = &Config{
		ModelLatency: ModelLatency{
			model: &RollingAverageLatency{
				NoOfCalls:           10,
				RecoveryTime:        10 * time.Minute,
				AvgLatencyThreshold: 3.0,
			},
		},
	}
	err = metrics.checkModelHealth(model, "s", config)
	if err != nil {
		t.Errorf("Expected model %q to be unhealthy (average latency too high)", model)
	}
}

// TestNewMetricsTracker verifies that a new metrics tracker is created and that the model_metrics table exists.
func TestNewMetricsTracker(t *testing.T) {
	setupTempDB(t)
	mt, err := newMetricsTracker("test_metrics_new")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.close()

	cols, err := mt.db.getColumns("model_metrics")
	if err != nil {
		t.Fatalf("Failed to get columns for model_metrics: %v", err)
	}
	// When using makeTables(true, ...), expect columns: id, timestamp, plus the columns we specified ("model" and "latency").
	expectedCols := []string{"id", "timestamp", "model", "latency"}
	for _, col := range expectedCols {
		found := false
		for _, c := range cols {
			if c == col {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected column %q in model_metrics, not found in %v", col, cols)
		}
	}
}

// TestRecordLatency verifies that recordLatency inserts a record into the model_metrics table.
func TestRecordLatency(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_record")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.close()

	if err := mt.recordLatency("model_record", 123.45, "s"); err != nil {
		t.Fatalf("recordLatency() failed: %v", err)
	}

	// executeQuery the table to verify a record exists.
	rows, err := mt.db.executeQuery("SELECT latency FROM model_metrics WHERE model = ?", "model_record")
	if err != nil {
		t.Fatalf("executeQuery() failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count == 0 {
		t.Errorf("Expected at least one record for model_record, got none")
	}
}

// TestCheckModelHealth_NoRecords verifies that checkModelHealth returns healthy when no records exist.
func TestCheckModelHealth_NoRecords(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_no_records")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.close()

	config := &Config{
		ModelLatency: ModelLatency{
			"nonexistent_model": &RollingAverageLatency{
				AvgLatencyThreshold: 100,
				NoOfCalls:           5,
				RecoveryTime:        time.Minute,
			},
		},
	}
	err = mt.checkModelHealth("nonexistent_model", "s", config)
	if err != nil {
		t.Errorf("Expected model to be healthy when no records exist")
	}
}

// TestCheckModelHealth_UnderThreshold verifies that a model with low recorded latencies is considered healthy.
func TestCheckModelHealth_UnderThreshold(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_under")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.close()

	// Insert two records with low latency.
	if err := mt.recordLatency("model_under", 50, "s"); err != nil {
		t.Fatalf("recordLatency() failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps.
	if err := mt.recordLatency("model_under", 50, "s"); err != nil {
		t.Fatalf("recordLatency() failed: %v", err)
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_under": &RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}
	err = mt.checkModelHealth("model_under", "s", config)
	if err != nil {
		t.Errorf("Expected model to be healthy with average latency below threshold")
	}
}

// TestCheckModelHealth_InsufficientData verifies that if there are fewer data points than required for the window,
// the model is considered healthy.
func TestCheckModelHealth_InsufficientData(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_insufficient")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.close()

	now := time.Now().UTC()
	// Insert only 2 records while we expect a window (NoOfCalls) of 5.
	ts1 := now.Add(-1 * time.Minute).Format(time.RFC3339Nano)
	ts2 := now.Add(-30 * time.Second).Format(time.RFC3339Nano)
	err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts1, "model_insufficient", 200.0)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts2, "model_insufficient", 150.0)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_insufficient": &RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}

	err = mt.checkModelHealth("model_insufficient", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy with insufficient data, got unhealthy")
	}
}

// TestCheckModelHealth_MovingAverage_Healthy provides enough low-latency records so that
// the moving average is below the threshold.
func TestCheckModelHealth_MovingAverage_Healthy(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_healthy")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.close()

	now := time.Now().UTC()
	// Insert 5 records with low latencies.
	latencies := []float64{50, 60, 55, 65, 60} // average ~58
	for i, latency := range latencies {
		ts := now.Add(time.Duration(-5+i) * time.Minute).Format(time.RFC3339Nano)
		err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts, "model_healthy", latency)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_healthy": &RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}

	err = mt.checkModelHealth("model_healthy", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy, got unhealthy")
	}
}

// TestCheckModelHealth_MovingAverage_Unhealthy provides enough high-latency records so that
// the moving average exceeds the threshold and the latest record is recent.
func TestCheckModelHealth_MovingAverage_Unhealthy(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_unhealthy")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.close()

	now := time.Now().UTC()
	// Insert 5 records with high latencies.
	latencies := []float64{200, 210, 220, 230, 240} // average ~220
	for i, latency := range latencies {
		ts := now.Add(time.Duration(-5+i) * time.Minute).Format(time.RFC3339Nano)
		err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts, "model_unhealthy", latency)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_unhealthy": &RollingAverageLatency{
				AvgLatencyThreshold: 150.0, // threshold is lower than the moving average of ~220
				NoOfCalls:           5,
				RecoveryTime:        10 * time.Minute, // recent data: no recovery
			},
		},
	}

	err = mt.checkModelHealth("model_unhealthy", "s", config)
	if err != nil {
		t.Errorf("expected model to be unhealthy due to high moving average, got healthy")
	}
}

// TestCheckModelHealth_MovingAverage_Recovered provides high-latency records where the most recent
// record is old (beyond the recovery time), so the model should be considered healthy.
func TestCheckModelHealth_MovingAverage_Recovered(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_recovered")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.close()

	now := time.Now().UTC()
	// Insert 5 records with high latencies, but with timestamps older than the recovery time.
	latencies := []float64{200, 210, 220, 230, 240}
	// Make the most recent record 10 minutes ago.
	for i, latency := range latencies {
		ts := now.Add(time.Duration(-10+i) * time.Minute).Format(time.RFC3339Nano)
		err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts, "model_recovered", latency)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_recovered": &RollingAverageLatency{
				AvgLatencyThreshold: 150.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute, // recovery period is short; data is old.
			},
		},
	}

	err = mt.checkModelHealth("model_recovered", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy due to recovery time elapsed, got unhealthy")
	}
}

// TestCheckModelHealth_MaxNoOfCalls verifies that if config.NoOfCalls is set higher than 10,
// it is capped to 10.
func TestCheckModelHealth_MaxNoOfCalls(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_maxcalls")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.close()

	now := time.Now().UTC()
	// Insert 12 records, each with a latency of 100.
	for i := 0; i < 12; i++ {
		ts := now.Add(time.Duration(-12+i) * time.Minute).Format(time.RFC3339Nano)
		err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts, "model_maxcalls", 100.0)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_maxcalls": &RollingAverageLatency{
				AvgLatencyThreshold: 150.0,
				NoOfCalls:           15, // Intend to use 15, but should be capped to 10.
				RecoveryTime:        1 * time.Minute,
			},
		},
	}

	err = mt.checkModelHealth("model_maxcalls", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy with capped NoOfCalls, got unhealthy")
	}
}

// TestCheckModelHealth_RecoveryTimeClamped verifies that a RecoveryTime greater than 1 hour is clamped.
func TestCheckModelHealth_RecoveryTimeClamped(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_recoveryclamped")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.close()

	// Insert a record with a timestamp 90 minutes ago.
	oldTime := time.Now().Add(-90 * time.Minute).UTC().Format(time.RFC3339Nano)
	err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", oldTime, "model_clamped", 200.0)
	if err != nil {
		t.Fatalf("manual insert failed: %v", err)
	}

	// Set RecoveryTime to 2 hours; CheckModelHealth should clamp it to 1 hour.
	config := &Config{
		ModelLatency: ModelLatency{
			"model_clamped": &RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        2 * time.Hour, // Should be clamped to 1 hour.
			},
		},
	}
	err = mt.checkModelHealth("model_clamped", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy since record is older than clamped recovery time, got unhealthy")
	}

	// Now insert a recent high-latency record.
	recentTime := time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339Nano)
	err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", recentTime, "model_clamped", 200.0)
	if err != nil {
		t.Fatalf("manual insert failed: %v", err)
	}
	err = mt.checkModelHealth("model_clamped", "s", config)
	if err != nil {
		t.Errorf("expected model to be unhealthy due to recent high latency record, got healthy")
	}
}

// TestCheckModelHealth_InvalidTimestamp simulates an invalid timestamp and expects an error.
func TestCheckModelHealth_InvalidTimestamp(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_invalid_ts")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.close()

	for i := 0; i < 5; i++ {
		// Insert a record with an invalid timestamp.
		err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency, status) VALUES(?, ?, ?, ?)", "invalid-timestamp", "model_invalid", 100.0, "s")
		if err != nil {
			t.Fatalf("manual insert with invalid timestamp failed: %v", err)
		}
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_invalid": &RollingAverageLatency{
				AvgLatencyThreshold: 150.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}

	err = mt.checkModelHealth("model_invalid", "s", config)
	if err != nil {
		t.Errorf("expected error when checking health with invalid timestamp, got nil")
	}
}

// TestCheckModelHealth_OverThreshold_NotRecovered verifies that a recent high-latency record makes the model unhealthy.
func TestCheckModelHealth_OverThreshold_NotRecovered(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_over_not_recovered")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.close()

	// Insert a record with high latency (current timestamp).
	if err := mt.recordLatency("model_over", 200, "s"); err != nil {
		t.Fatalf("recordLatency() failed: %v", err)
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_over": &RollingAverageLatency{
				AvgLatencyThreshold: 150.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}
	err = mt.checkModelHealth("model_over", "s", config)
	if err != nil {
		t.Errorf("Expected model to be unhealthy due to high latency and insufficient recovery time")
	}
}

// TestCheckModelHealth_OverThreshold_Recovered verifies that a record older than the recovery time makes the model healthy.
func TestCheckModelHealth_OverThreshold_Recovered(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_over_recovered")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.close()

	// Manually insert a record with a timestamp older than the recovery time.
	oldTime := time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)
	err = mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", oldTime, "model_recovered", 200)
	if err != nil {
		t.Fatalf("Manual insert failed: %v", err)
	}

	config := &Config{
		ModelLatency: ModelLatency{
			"model_recovered": &RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}
	err = mt.checkModelHealth("model_recovered", "s", config)
	if err != nil {
		t.Errorf("Expected model to be healthy since recovery time has elapsed")
	}
}

// TestCloseMetricsTracker verifies that after closing the metrics tracker, operations fail.
func TestCloseMetricsTracker(t *testing.T) {
	setupTempDB(t)

	mt, err := newMetricsTracker("test_metrics_close")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}

	if err := mt.close(); err != nil {
		t.Fatalf("closeConnection() failed: %v", err)
	}

	// Attempting to record latency after closeConnection should fail.
	err = mt.recordLatency("model_close", 100, "s")
	if err == nil {
		t.Errorf("Expected error when calling recordLatency after closeConnection, got nil")
	}
}

// TestDropMetricsTracker verifies that dropDB closes the database and removes the underlying file.
func TestDropMetricsTracker(t *testing.T) {
	setupTempDB(t)

	dbPath := "test_metrics_drop"
	mt, err := newMetricsTracker(dbPath)
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	dbFile := mt.db.Schema

	if err := mt.drop(); err != nil {
		t.Fatalf("dropDB() failed: %v", err)
	}
	// Verify that the database file no longer exists.
	if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
		t.Errorf("Expected database file %q to be removed after dropDB(), but it exists", dbFile)
	}
}
