package notdiamond

import (
	"database/sql"
	"time"
)

// metricsTracker manages a SQLite database that records call latencies per model.
type metricsTracker struct {
	db *database
}

// newMetricsTracker initializes the SQLite database (stored in the file given by dbPath)
// and creates the table if needed.
func newMetricsTracker(dbPath string) (*metricsTracker, error) {
	db, err := openDB(dbPath, false)
	if err != nil {
		return nil, err
	}

	// Create the table if it does not exist.
	err = db.makeTables(true, "model_metrics", map[string]string{
		"model":   "TEXT",
		"latency": "REAL",
	})
	if err != nil {
		return nil, err
	}

	return &metricsTracker{db: db}, nil
}

// recordLatency records a call's latency for a given model.
func (mt *metricsTracker) recordLatency(model string, latency float64) error {
	err := mt.db.execQuery("INSERT INTO model_metrics(timestamp, model, latency) VALUES(?, ?, ?)",
		time.Now().UTC(), model, latency)
	return err
}

// checkModelHealth returns true if the model is healthy (i.e. its average latency over the last
// noOfCalls does not exceed avgLatency threshold). If the model is unhealthy, it is considered “blacklisted”
// until recoveryTime has elapsed since the last call that exceeded the threshold.
func (mt *metricsTracker) checkModelHealth(model string, config *Config) (bool, error) {
	if config.NoOfCalls > 10 {
		config.NoOfCalls = 10 // Enforce maximum
	}
	if config.RecoveryTime > time.Hour {
		config.RecoveryTime = time.Hour // Enforce maximum
	}

	// executeQuery the most recent noOfCalls calls for the model.
	query := `
	SELECT timestamp, latency FROM model_metrics
	WHERE model = ?
	ORDER BY timestamp DESC
	LIMIT ?;
	`
	rows, err := mt.db.executeQuery(query, model, config.NoOfCalls)
	if err != nil {
		return false, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			// logger error but don't return it.
			errorLog("Failed to close rows:", err)
		}
	}(rows)

	var totalLatency float64
	var count int
	var latestTime time.Time

	for rows.Next() {
		var ts string
		var latency float64
		if err := rows.Scan(&ts, &latency); err != nil {
			return false, err
		}
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			// Fallback: try to parse with time.RFC3339
			t, err = time.Parse(time.RFC3339, ts)
			if err != nil {
				return false, err
			}
		}
		if count == 0 {
			latestTime = t
		}
		totalLatency += latency
		count++
	}

	// If there are not enough records, consider the model healthy.
	if count == 0 {
		return true, nil
	}

	average := totalLatency / float64(count)
	if average > config.AvgLatencyThreshold {
		// Check if recovery_time has elapsed since the most recent call.
		if time.Since(latestTime) < config.RecoveryTime {
			// Model is unhealthy.
			return false, nil
		}
		// Recovery time elapsed: consider the model healthy.
	}
	return true, nil
}

// close closes the underlying database connection.
func (mt *metricsTracker) close() error {
	return mt.db.closeConnection()
}

// drop closes the underlying database connection.
func (mt *metricsTracker) drop() error {
	return mt.db.dropDB()
}
