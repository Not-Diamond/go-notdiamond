package metric

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/database"
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

// setupTempDB sets the innerDB.DataFolder to a unique temporary directory for the test.
func setupTempDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	absDir, err := filepath.Abs(tmpDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	database.DataFolder = absDir
}

func TestMetricsTracker_RecordAndHealth(t *testing.T) {
	setupTempDB(t)
	currentModel := "openai/gpt-4o"
	metrics, err := NewTracker(":memory:" + t.Name())
	if err != nil {
		slog.Error("Failed to open database connection", "error", err.Error())
	}
	// Record several latencies. For example, 1, 2, 3, and 4 seconds.
	latencies := []float64{1.0, 2.0, 3.0, 4.0} // average = 2.5 seconds
	for _, l := range latencies {
		err := metrics.RecordLatency(currentModel, l, "s")
		if err != nil {
			t.Errorf("recordLatency error: %v", err)
		}
	}

	// Use thresholds: average_latency threshold = 3.0 sec, no_of_calls = 10, recovery_time = 10 minutes.
	config := model.Config{
		ModelLatency: model.ModelLatency{
			currentModel: &model.RollingAverageLatency{
				NoOfCalls:           10,
				RecoveryTime:        10 * time.Minute,
				AvgLatencyThreshold: 3.0,
			},
		},
	}

	err = metrics.CheckModelHealth(currentModel, "s", config)
	if err != nil {
		t.Errorf("Expected model %q to be healthy (avg=2.5 < threshold=3.0)", currentModel)
	}

	// Record two high latency calls (e.g. 10 seconds each), which should push the average above the threshold.
	highLatencies := []float64{10.0, 10.0}
	for _, l := range highLatencies {
		err := metrics.RecordLatency(currentModel, l, "s")
		if err != nil {
			t.Errorf("recordLatency error: %v", err)
		}
	}

	// Use thresholds: average_latency threshold = 3.0 sec, no_of_calls = 10, recovery_time = 10 minutes.
	config = model.Config{
		ModelLatency: model.ModelLatency{
			currentModel: &model.RollingAverageLatency{
				NoOfCalls:           10,
				RecoveryTime:        10 * time.Minute,
				AvgLatencyThreshold: 3.0,
			},
		},
	}
	err = metrics.CheckModelHealth(currentModel, "s", config)
	if err != nil {
		t.Errorf("Expected model %q to be unhealthy (average latency too high)", currentModel)
	}
}

// TestNewMetricsTracker verifies that a new metrics tracker is created and that the model_metrics table exists.
func TestNewMetricsTracker(t *testing.T) {
	setupTempDB(t)
	mt, err := NewTracker("test_metrics_new")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.Close()

	cols, err := mt.db.GetColumns("model_metrics")
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

	mt, err := NewTracker("test_metrics_record")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.Close()

	if err := mt.RecordLatency("model_record", 123.45, "s"); err != nil {
		t.Fatalf("recordLatency() failed: %v", err)
	}

	// executeQuery the table to verify a record exists.
	rows, err := mt.db.Query("SELECT latency FROM model_metrics WHERE model = ?", "model_record")
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

	mt, err := NewTracker("test_metrics_no_records")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.Close()

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"nonexistent_model": &model.RollingAverageLatency{
				AvgLatencyThreshold: 100,
				NoOfCalls:           5,
				RecoveryTime:        time.Minute,
			},
		},
	}
	err = mt.CheckModelHealth("nonexistent_model", "s", config)
	if err != nil {
		t.Errorf("Expected model to be healthy when no records exist")
	}
}

// TestCheckModelHealth_UnderThreshold verifies that a model with low recorded latencies is considered healthy.
func TestCheckModelHealth_UnderThreshold(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_under")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.Close()

	// Insert two records with low latency.
	if err := mt.RecordLatency("model_under", 50, "s"); err != nil {
		t.Fatalf("recordLatency() failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps.
	if err := mt.RecordLatency("model_under", 50, "s"); err != nil {
		t.Fatalf("recordLatency() failed: %v", err)
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_under": &model.RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}
	err = mt.CheckModelHealth("model_under", "s", config)
	if err != nil {
		t.Errorf("Expected model to be healthy with average latency below threshold")
	}
}

// TestCheckModelHealth_InsufficientData verifies that if there are fewer data points than required for the window,
// the model is considered healthy.
func TestCheckModelHealth_InsufficientData(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_insufficient")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	now := time.Now().UTC()
	// Insert only 2 records while we expect a window (NoOfCalls) of 5.
	ts1 := now.Add(-1 * time.Minute).Format(time.RFC3339Nano)
	ts2 := now.Add(-30 * time.Second).Format(time.RFC3339Nano)
	err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts1, "model_insufficient", 200.0)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts2, "model_insufficient", 150.0)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_insufficient": &model.RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}

	err = mt.CheckModelHealth("model_insufficient", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy with insufficient data, got unhealthy")
	}
}

// TestCheckModelHealth_MovingAverage_Healthy provides enough low-latency records so that
// the moving average is below the threshold.
func TestCheckModelHealth_MovingAverage_Healthy(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_healthy")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	now := time.Now().UTC()
	// Insert 5 records with low latencies.
	latencies := []float64{50, 60, 55, 65, 60} // average ~58
	for i, latency := range latencies {
		ts := now.Add(time.Duration(-5+i) * time.Minute).Format(time.RFC3339Nano)
		err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts, "model_healthy", latency)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_healthy": &model.RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}

	err = mt.CheckModelHealth("model_healthy", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy, got unhealthy")
	}
}

// TestCheckModelHealth_MovingAverage_Unhealthy provides enough high-latency records so that
// the moving average exceeds the threshold and the latest record is recent.
func TestCheckModelHealth_MovingAverage_Unhealthy(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_unhealthy")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	now := time.Now().UTC()
	// Insert 5 records with high latencies.
	latencies := []float64{200, 210, 220, 230, 240} // average ~220
	for i, latency := range latencies {
		ts := now.Add(time.Duration(-5+i) * time.Minute).Format(time.RFC3339Nano)
		err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts, "model_unhealthy", latency)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_unhealthy": &model.RollingAverageLatency{
				AvgLatencyThreshold: 150.0, // threshold is lower than the moving average of ~220
				NoOfCalls:           5,
				RecoveryTime:        10 * time.Minute, // recent data: no recovery
			},
		},
	}

	err = mt.CheckModelHealth("model_unhealthy", "s", config)
	if err != nil {
		t.Errorf("expected model to be unhealthy due to high moving average, got healthy")
	}
}

// TestCheckModelHealth_MovingAverage_Recovered provides high-latency records where the most recent
// record is old (beyond the recovery time), so the model should be considered healthy.
func TestCheckModelHealth_MovingAverage_Recovered(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_recovered")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	now := time.Now().UTC()
	// Insert 5 records with high latencies, but with timestamps older than the recovery time.
	latencies := []float64{200, 210, 220, 230, 240}
	// Make the most recent record 10 minutes ago.
	for i, latency := range latencies {
		ts := now.Add(time.Duration(-10+i) * time.Minute).Format(time.RFC3339Nano)
		err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts, "model_recovered", latency)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_recovered": &model.RollingAverageLatency{
				AvgLatencyThreshold: 150.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute, // recovery period is short; data is old.
			},
		},
	}

	err = mt.CheckModelHealth("model_recovered", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy due to recovery time elapsed, got unhealthy")
	}
}

// TestCheckModelHealth_MaxNoOfCalls verifies that if config.NoOfCalls is set higher than 10,
// it is capped to 10.
func TestCheckModelHealth_MaxNoOfCalls(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_maxcalls")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	now := time.Now().UTC()
	// Insert 12 records, each with a latency of 100.
	for i := 0; i < 12; i++ {
		ts := now.Add(time.Duration(-12+i) * time.Minute).Format(time.RFC3339Nano)
		err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", ts, "model_maxcalls", 100.0)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_maxcalls": &model.RollingAverageLatency{
				AvgLatencyThreshold: 150.0,
				NoOfCalls:           15, // Intend to use 15, but should be capped to 10.
				RecoveryTime:        1 * time.Minute,
			},
		},
	}

	err = mt.CheckModelHealth("model_maxcalls", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy with capped NoOfCalls, got unhealthy")
	}
}

// TestCheckModelHealth_RecoveryTimeClamped verifies that a RecoveryTime greater than 1 hour is clamped.
func TestCheckModelHealth_RecoveryTimeClamped(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_recoveryclamped")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	// Insert a record with a timestamp 90 minutes ago.
	oldTime := time.Now().Add(-90 * time.Minute).UTC().Format(time.RFC3339Nano)
	err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", oldTime, "model_clamped", 200.0)
	if err != nil {
		t.Fatalf("manual insert failed: %v", err)
	}

	// Set RecoveryTime to 2 hours; CheckModelHealth should clamp it to 1 hour.
	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_clamped": &model.RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        2 * time.Hour, // Should be clamped to 1 hour.
			},
		},
	}
	err = mt.CheckModelHealth("model_clamped", "s", config)
	if err != nil {
		t.Errorf("expected model to be healthy since record is older than clamped recovery time, got unhealthy")
	}

	// Now insert a recent high-latency record.
	recentTime := time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339Nano)
	err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", recentTime, "model_clamped", 200.0)
	if err != nil {
		t.Fatalf("manual insert failed: %v", err)
	}
	err = mt.CheckModelHealth("model_clamped", "s", config)
	if err != nil {
		t.Errorf("expected model to be unhealthy due to recent high latency record, got healthy")
	}
}

// TestCheckModelHealth_InvalidTimestamp simulates an invalid timestamp and expects an error.
func TestCheckModelHealth_InvalidTimestamp(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_invalid_ts")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	for i := 0; i < 5; i++ {
		// Insert a record with an invalid timestamp.
		err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency, status) VALUES(?, ?, ?, ?)", "invalid-timestamp", "model_invalid", 100.0, "s")
		if err != nil {
			t.Fatalf("manual insert with invalid timestamp failed: %v", err)
		}
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_invalid": &model.RollingAverageLatency{
				AvgLatencyThreshold: 150.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}

	err = mt.CheckModelHealth("model_invalid", "s", config)
	if err != nil {
		t.Errorf("expected error when checking health with invalid timestamp, got nil")
	}
}

// TestCheckModelHealth_OverThreshold_NotRecovered verifies that a recent high-latency record makes the model unhealthy.
func TestCheckModelHealth_OverThreshold_NotRecovered(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_over_not_recovered")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.Close()

	// Insert a record with high latency (current timestamp).
	if err := mt.RecordLatency("model_over", 200, "s"); err != nil {
		t.Fatalf("recordLatency() failed: %v", err)
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_over": &model.RollingAverageLatency{
				AvgLatencyThreshold: 150.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}
	err = mt.CheckModelHealth("model_over", "s", config)
	if err != nil {
		t.Errorf("Expected model to be unhealthy due to high latency and insufficient recovery time")
	}
}

// TestCheckModelHealth_OverThreshold_Recovered verifies that a record older than the recovery time makes the model healthy.
func TestCheckModelHealth_OverThreshold_Recovered(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_over_recovered")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	defer mt.Close()

	// Manually insert a record with a timestamp older than the recovery time.
	oldTime := time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)
	err = mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)", oldTime, "model_recovered", 200)
	if err != nil {
		t.Fatalf("Manual insert failed: %v", err)
	}

	config := model.Config{
		ModelLatency: model.ModelLatency{
			"model_recovered": &model.RollingAverageLatency{
				AvgLatencyThreshold: 100.0,
				NoOfCalls:           5,
				RecoveryTime:        1 * time.Minute,
			},
		},
	}
	err = mt.CheckModelHealth("model_recovered", "s", config)
	if err != nil {
		t.Errorf("Expected model to be healthy since recovery time has elapsed")
	}
}

// TestCloseMetricsTracker verifies that after closing the metrics tracker, operations fail.
func TestCloseMetricsTracker(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_close")
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}

	if err := mt.Close(); err != nil {
		t.Fatalf("closeConnection() failed: %v", err)
	}

	// Attempting to record latency after closeConnection should fail.
	err = mt.RecordLatency("model_close", 100, "s")
	if err == nil {
		t.Errorf("Expected error when calling recordLatency after closeConnection, got nil")
	}
}

// TestDropMetricsTracker verifies that dropDB closes the database and removes the underlying file.
func TestDropMetricsTracker(t *testing.T) {
	setupTempDB(t)

	dbPath := "test_metrics_drop"
	mt, err := NewTracker(dbPath)
	if err != nil {
		t.Fatalf("newMetricsTracker() failed: %v", err)
	}
	dbFile := mt.db.Schema

	if err := mt.Drop(); err != nil {
		t.Fatalf("dropDB() failed: %v", err)
	}
	// Verify that the database file no longer exists.
	if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
		t.Errorf("Expected database file %q to be removed after dropDB(), but it exists", dbFile)
	}
}

// TestCheckModelHealth_LatencyFallback verifies that a model is marked unhealthy
// when its average latency exceeds the threshold and remains in recovery for the specified duration.
func TestCheckModelHealth_LatencyFallback(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_latency_fallback")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	currentModel := "test/model"
	var config model.Config
	config.ModelLatency = make(model.ModelLatency)
	config.ModelLatency[currentModel] = &model.RollingAverageLatency{
		AvgLatencyThreshold: 0.5,             // 500ms threshold
		NoOfCalls:           5,               // Need 5 calls to calculate average
		RecoveryTime:        1 * time.Second, // Short recovery for testing
	}

	// Record 5 high-latency calls
	for i := 0; i < 5; i++ {
		err := mt.RecordLatency(currentModel, 1.0, "success") // 1 second latency
		if err != nil {
			t.Fatalf("RecordLatency failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps
	}

	// First check should mark the model as unhealthy
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err == nil {
		t.Error("Expected model to be marked as unhealthy")
	} else if !strings.Contains(err.Error(), "is unhealthy") {
		t.Errorf("Expected unhealthy error message, got: %v", err)
	}

	// Immediate recheck should show model in recovery period
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err == nil {
		t.Error("Expected model to be in recovery period")
	} else if !strings.Contains(err.Error(), "recovery period") {
		t.Errorf("Expected recovery period error message, got: %v", err)
	}

	// Wait for recovery time to pass
	time.Sleep(2 * time.Second)

	// Model should now be allowed to try again
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err != nil {
		t.Errorf("Expected model to be recovered, got error: %v", err)
	}
}

// TestCheckModelHealth_RecoveryAndReset tests that:
// 1. Model becomes unhealthy after high latencies
// 2. Model stays in recovery for the specified duration
// 3. After recovery, old latency data is cleared
// 4. Model becomes healthy and can become unhealthy again with new high latencies
func TestCheckModelHealth_RecoveryAndReset(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_recovery_reset")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	currentModel := "test/model"
	var config model.Config
	config.ModelLatency = make(model.ModelLatency)
	config.ModelLatency[currentModel] = &model.RollingAverageLatency{
		AvgLatencyThreshold: 0.5,                    // 500ms threshold
		NoOfCalls:           5,                      // Need 5 calls to calculate average
		RecoveryTime:        100 * time.Millisecond, // Shorter recovery time for testing
	}

	// Step 1: Record 5 high-latency calls to make model unhealthy
	for i := 0; i < 5; i++ {
		err := mt.RecordLatency(currentModel, 1.0, "success") // 1 second latency
		if err != nil {
			t.Fatalf("RecordLatency failed: %v", err)
		}
		time.Sleep(5 * time.Millisecond) // Shorter delay between records
	}

	// Check that model is marked unhealthy
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err == nil || !strings.Contains(err.Error(), "is unhealthy") {
		t.Fatalf("Expected model to be marked as unhealthy, got error: %v", err)
	}

	// Step 2: Verify model stays in recovery
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err == nil || !strings.Contains(err.Error(), "recovery period") {
		t.Fatalf("Expected model to be in recovery period, got error: %v", err)
	}

	// Step 3: Wait for recovery time to pass
	time.Sleep(150 * time.Millisecond) // Wait slightly longer than recovery time

	// Step 4: Verify model is healthy and latency data was cleared
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err != nil {
		t.Errorf("Expected model to be healthy after recovery, got error: %v", err)
	}

	// Verify latency data was cleared by checking the count
	rows, err := mt.db.Query("SELECT COUNT(*) FROM model_metrics WHERE model = ?", currentModel)
	if err != nil {
		t.Fatalf("Failed to query metrics count: %v", err)
	}
	defer rows.Close()

	var count int
	if rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			t.Fatalf("Failed to scan count: %v", err)
		}
	}
	if count != 0 {
		t.Errorf("Expected latency data to be cleared (count = 0), got count = %d", count)
	}

	// Step 5: Record new high latencies and verify model can become unhealthy again
	for i := 0; i < 5; i++ {
		err := mt.RecordLatency(currentModel, 1.0, "success")
		if err != nil {
			t.Fatalf("RecordLatency failed: %v", err)
		}
		time.Sleep(5 * time.Millisecond) // Shorter delay between records
	}

	err = mt.CheckModelHealth(currentModel, "success", config)
	if err == nil || !strings.Contains(err.Error(), "is unhealthy") {
		t.Fatalf("Expected model to become unhealthy again with new high latencies, got error: %v", err)
	}
}

// TestCheckModelHealth_RecoveryTimeClearing tests that:
// 1. Recovery time is properly recorded when model becomes unhealthy
// 2. Recovery time is cleared after recovery period
func TestCheckModelHealth_RecoveryTimeClearing(t *testing.T) {
	setupTempDB(t)

	mt, err := NewTracker("test_metrics_recovery_clearing")
	if err != nil {
		t.Fatalf("NewMetricsTracker failed: %v", err)
	}
	defer mt.Close()

	currentModel := "test/model"
	var config model.Config
	config.ModelLatency = make(model.ModelLatency)
	config.ModelLatency[currentModel] = &model.RollingAverageLatency{
		AvgLatencyThreshold: 0.5,
		NoOfCalls:           5,
		RecoveryTime:        100 * time.Millisecond,
	}

	// Make model unhealthy to record recovery time
	for i := 0; i < 5; i++ {
		err := mt.RecordLatency(currentModel, 1.0, "success")
		if err != nil {
			t.Fatalf("RecordLatency failed: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// First check should mark the model as unhealthy and record recovery time
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err == nil || !strings.Contains(err.Error(), "is unhealthy") {
		t.Fatalf("Expected model to be marked as unhealthy, got error: %v", err)
	}

	// Verify we're in recovery period
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err == nil || !strings.Contains(err.Error(), "recovery period") {
		t.Fatalf("Expected model to be in recovery period, got error: %v", err)
	}

	// Now verify recovery time is recorded
	var recoveryTime time.Time
	err = mt.db.GetJSON("keystore", currentModel, &recoveryTime)
	if err != nil {
		t.Errorf("Expected recovery time to be recorded, got error: %v", err)
	}

	// Wait for recovery period
	time.Sleep(150 * time.Millisecond)

	// Check model health to trigger recovery time clearing
	err = mt.CheckModelHealth(currentModel, "success", config)
	if err != nil {
		t.Errorf("Expected model to be healthy after recovery, got error: %v", err)
	}

	// Add a small delay to ensure database operations complete
	time.Sleep(50 * time.Millisecond)

	// Verify recovery time was cleared
	err = mt.db.GetJSON("keystore", currentModel, &recoveryTime)
	if err == nil {
		t.Error("Expected recovery time to be cleared, but it still exists")
	}
}
