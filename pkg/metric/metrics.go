package metric

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/database"
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/Not-Diamond/go-notdiamond/pkg/statistic"
	"github.com/pkg/errors"
)

// metricsTracker manages a SQLite database that records call latencies per model.
type Tracker struct {
	db database.Instance
}

// NewTracker initializes the SQLite database (stored in the file given by dbPath)
// and creates the table if needed.
func NewTracker(dbPath string) (*Tracker, error) {
	db, err := database.Open(dbPath, false)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode and set busy timeout
	err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %v", err)
	}

	err = db.Exec("PRAGMA busy_timeout=5000") // 5 second timeout
	if err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %v", err)
	}

	// Create the table if it does not exist.
	err = db.CreateTables(true, "model_metrics", model.Message{
		"model":   "TEXT",
		"latency": "REAL",
		"status":  "TEXT",
	})
	if err != nil {
		return nil, err
	}

	return &Tracker{db: *db}, nil
}

// recordLatency records a call's latency for a given model.
func (mt *Tracker) RecordLatency(model string, latency float64, status string) error {
	err := mt.db.Exec("INSERT INTO model_metrics(timestamp, model, latency, status) VALUES(?, ?, ?, ?)",
		time.Now().UTC(), model, latency, status)
	if err != nil {
		return fmt.Errorf("RecordLatency failed: %v", err)
	}
	return nil
}

// recordRecoveryTime records the recovery time for a given model.
func (mt *Tracker) RecordRecoveryTime(model string) error {
	return mt.db.SetJSON("keystore", model, time.Now().UTC())
}

// CheckRecoveryTime checks if the model has recovered from a previous unhealthy state.
func (mt *Tracker) CheckRecoveryTime(model string, config model.Config) error {
	var latestTime time.Time

	err := mt.db.GetJSON("keystore", model, &latestTime)
	if err != nil {
		// If no recovery time is recorded, the model is considered healthy
		return nil
	}

	if time.Since(latestTime) < config.ModelLatency[model].RecoveryTime {
		return errors.Errorf("Model %s is still in recovery period for %v",
			model, config.ModelLatency[model].RecoveryTime-time.Since(latestTime))
	}

	// Clear the recovery time since the recovery period has elapsed
	err = mt.db.Exec("DELETE FROM keystore WHERE key = ?", model)
	if err != nil {
		slog.Error("Failed to clear recovery time", "error", err)
		return nil
	}

	// Clear old latency data so the model starts fresh
	err = mt.db.Exec("DELETE FROM model_metrics WHERE model = ?", model)
	if err != nil {
		slog.Error("Failed to clear old latency data", "error", err)
		return nil
	}

	slog.Info(fmt.Sprintf("Model %s has recovered and latency data has been reset", model))
	return nil
}

// checkModelHealth returns true if the model is healthy (i.e. its average latency over the last
// noOfCalls does not exceed avgLatency threshold). If the model is unhealthy, it is considered "blacklisted"
// until recoveryTime has elapsed since the last call that exceeded the threshold.
func (mt *Tracker) CheckModelHealth(model string, status string, config model.Config) error {
	// Skip health check if ModelLatency is not configured for this model
	if _, exists := config.ModelLatency[model]; !exists {
		return nil
	}

	// Enforce maximum number of calls
	maxLimit := config.ModelLimits.MaxNoOfCalls
	if maxLimit == 0 {
		maxLimit = 10000
	}
	maxRecoveryTime := config.ModelLimits.MaxRecoveryTime
	if maxRecoveryTime == 0 {
		maxRecoveryTime = time.Hour
	}

	if config.ModelLatency[model].NoOfCalls > maxLimit {
		slog.Info("🚨 Enforcing maximum number of calls", "model", model, "noOfCalls", config.ModelLatency[model].NoOfCalls, "maxLimit", maxLimit)
		config.ModelLatency[model].NoOfCalls = maxLimit // Enforce maximum
	}
	if config.ModelLatency[model].RecoveryTime > maxRecoveryTime {
		slog.Info("🚨 Enforcing maximum recovery time", "model", model, "recoveryTime", config.ModelLatency[model].RecoveryTime, "maxRecoveryTime", maxRecoveryTime)
		config.ModelLatency[model].RecoveryTime = maxRecoveryTime // Enforce maximum
	}

	// First check if we're still in recovery period
	err := mt.CheckRecoveryTime(model, config)
	if err != nil {
		return err // Still in recovery period
	}

	// If we're not in recovery, check the latency average
	raw_query := `
	SELECT timestamp, latency FROM model_metrics
	WHERE model = '%s' and status LIKE '%%' || '%s' || '%%'
	ORDER BY timestamp DESC
	LIMIT %d;
	`
	query := fmt.Sprintf(raw_query, model, status, config.ModelLatency[model].NoOfCalls)
	rows, err := mt.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Create a new Statistics instance to accumulate data.
	stats := statistic.NewStatistics()

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

	// If there are not enough data points, assume model is healthy.
	if len(stats.Data) < config.ModelLatency[model].NoOfCalls {
		slog.Info("🛈 Not enough data points for model", "model", model, "noOfCalls", config.ModelLatency[model].NoOfCalls)
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
		// Record the recovery time when we first detect the model is unhealthy
		err := mt.RecordRecoveryTime(model)
		if err != nil {
			slog.Error("Failed to record recovery time", "error", err)
		}

		// High moving average: model is unhealthy
		return errors.Errorf("Model %s is unhealthy: moving average %.2f exceeds threshold %.2f",
			model, latestMovingAvg, config.ModelLatency[model].AvgLatencyThreshold)
	}

	// Otherwise, the model is healthy.
	slog.Info(fmt.Sprintf("✓ Model %s is healthy, moving average: %.2f is below threshold: %.2f", model, latestMovingAvg, config.ModelLatency[model].AvgLatencyThreshold))
	return nil
}

// close closes the underlying database connection.
func (mt *Tracker) Close() error {
	return mt.db.CloseConnection()
}

// drop closes the underlying database connection.
func (mt *Tracker) Drop() error {
	return mt.db.Drop()
}
