package notdiamond

import (
	"fmt"
	"github.com/pkg/errors"
	"time"
)

// metricsTracker manages a SQLite database that records call latencies per model.
type metricsTracker struct {
	db    *database
	debug bool
}

// newMetricsTracker initializes the SQLite database (stored in the file given by dbPath)
// and creates the table if needed.
func newMetricsTracker(dbPath string) (*metricsTracker, error) {
	db, err := openDB(dbPath, false)
	if err != nil {
		return nil, err
	}

	// Create the table if it does not exist.
	err = db.makeTables(true, "model_metrics", Message{
		"reqID":   "TEXT",
		"model":   "TEXT",
		"latency": "REAL",
		"status":  "TEXT",
	})
	if err != nil {
		return nil, err
	}

	return &metricsTracker{db: db}, nil
}

// recordLatency records a call's latency for a given model.
func (mt *metricsTracker) recordLatency(reqID string, model string, latency float64, status string) error {
	err := mt.db.execQuery("INSERT INTO model_metrics(timestamp, reqID, model, latency, status) VALUES(?, ?, ?, ?, ?)",
		time.Now().UTC(), reqID, model, latency, status)
	return err
}

// recordRecoveryTime records the recovery time for a given model.
func (mt *metricsTracker) recordRecoveryTime(model string) error {
	return mt.db.setJSON("keystore", model, time.Now().UTC())
}

func (mt *metricsTracker) checkRecoveryTime(model string, config *Config) error {
	var latestTime time.Time

	err := mt.db.getJSON("keystore", model, &latestTime)
	if err != nil {

	}

	if time.Since(latestTime) < config.ModelLatency[model].RecoveryTime {
		return errors.Errorf("Model hasn't recover yet, skipping")
	}

	return nil
}

// checkModelHealth returns true if the model is healthy (i.e. its average latency over the last
// noOfCalls does not exceed avgLatency threshold). If the model is unhealthy, it is considered “blacklisted”
// until recoveryTime has elapsed since the last call that exceeded the threshold.
func (mt *metricsTracker) checkModelHealth(reqID, model string, status string, config *Config) error {
	if config.ModelLatency[model].NoOfCalls > 10 {
		config.ModelLatency[model].NoOfCalls = 10 // Enforce maximum
	}
	if config.ModelLatency[model].RecoveryTime > time.Hour {
		config.ModelLatency[model].RecoveryTime = time.Hour // Enforce maximum
	}

	//err := mt.checkRecoveryTime(model, config)
	//if err != nil {
	//	return err
	//}
	// executeQuery the most recent noOfCalls calls for the model.
	raw_query := `
	SELECT timestamp, latency FROM model_metrics
	WHERE model = '%s' and status LIKE '%%' || '%s' || '%%'
	ORDER BY timestamp DESC
	LIMIT %d;
	`
	query := fmt.Sprintf(raw_query, model, status, config.ModelLatency[model].NoOfCalls)
	rows, err := mt.db.executeQuery(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Create a new Statistics instance to accumulate data.
	stats := NewStatistics()

	for rows.Next() {
		var ts string
		var latency float64
		if err := rows.Scan(&ts, &latency); err != nil {
			return err
		}
		// Parse the timestamp.
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			// Fallback: try RFC3339
			t, err = time.Parse(time.RFC3339, ts)
			if err != nil {
				return errors.Wrap(err, "CheckModelHealth parse time")
			}
		}
		stats.Add(t, latency)
	}

	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "getting rows error")
	}

	//average, err := stats.MovingAverage(config.ModelLatency[model].NoOfCalls)
	//if err != nil {
	//	return errors.Wrap(err, "CheckModelHealth moving average")
	//}
	//if average > config.ModelLatency[model].AvgLatencyThreshold {
	//	err = mt.recordRecoveryTime(model)
	//	if err == nil {
	//		logger.Infof("Recovery time for model: %s started", model)
	//	}
	//	return errors.Errorf("Average latency for model %s is below threshold: %f. Resetting time for recovery", model, average)
	//}
	//
	//if err := rows.Err(); err != nil {
	//	return err
	//}
	//return nil
	// If there are not enough data points, assume model is healthy.
	if len(stats.Data) < config.ModelLatency[model].NoOfCalls {
		return nil
	}

	// Our query returned data in descending order (most recent first).
	// Reverse the slice so that data is in ascending order (oldest first)
	for i, j := 0, len(stats.Data)-1; i < j; i, j = i+1, j-1 {
		stats.Data[i], stats.Data[j] = stats.Data[j], stats.Data[i]
	}

	// Compute the moving average with a window equal to config.NoOfCalls.
	movingAverages, err := stats.MovingAverage(config.ModelLatency[model].NoOfCalls)
	if err != nil {
		return err
	}
	// The last element is the moving average for the most recent window.
	latestMovingAvg := movingAverages[len(movingAverages)-1]

	// If the latest moving average exceeds the threshold, check the recovery time.
	if latestMovingAvg > config.ModelLatency[model].AvgLatencyThreshold {
		// Use the timestamp of the most recent record (now at the end after reversal)
		latestTimestamp := stats.Data[len(stats.Data)-1].Timestamp
		if time.Since(latestTimestamp) < config.ModelLatency[model].RecoveryTime {
			// High moving average and recent data: model is unhealthy.
			return errors.Errorf("High moving average and recent data: model is unhealthy.")
		}
	}
	// Otherwise, the model is healthy.
	return nil
}

// close closes the underlying database connection.
func (mt *metricsTracker) close() error {
	return mt.db.closeConnection()
}

// drop closes the underlying database connection.
func (mt *metricsTracker) drop() error {
	return mt.db.dropDB()
}
