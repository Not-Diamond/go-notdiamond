package metric

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/Not-Diamond/go-notdiamond/pkg/redis"
	"github.com/pkg/errors"
)

// Tracker manages metrics for model calls using Redis
type Tracker struct {
	client *redis.Client
}

// NewTracker initializes a new Redis client for tracking metrics
func NewTracker(redisAddr string) (*Tracker, error) {
	cfg := redis.Config{
		Addr:     redisAddr,
		Password: "", // Add password if needed
		DB:       0,  // Default DB
	}

	client, err := redis.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %v", err)
	}

	return &Tracker{client: client}, nil
}

// Close closes the Redis connection
func (mt *Tracker) Close() error {
	return mt.client.Close()
}

// RecordLatency records a call's latency for a given model
func (mt *Tracker) RecordLatency(model string, latency float64, status string) error {
	ctx := context.Background()
	err := mt.client.RecordLatency(ctx, model, latency, status)
	if err != nil {
		return fmt.Errorf("RecordLatency failed: %v", err)
	}
	return nil
}

// RecordRecoveryTime records the recovery time for a given model
func (mt *Tracker) RecordRecoveryTime(model string, config model.Config) error {
	ctx := context.Background()
	duration := config.ModelLatency[model].RecoveryTime
	return mt.client.SetRecoveryTime(ctx, model, duration)
}

// CheckRecoveryTime checks if the model has recovered from a previous unhealthy state
func (mt *Tracker) CheckRecoveryTime(model string, config model.Config) error {
	ctx := context.Background()

	inRecovery, err := mt.client.CheckRecoveryTime(ctx, model)
	if err != nil {
		return fmt.Errorf("failed to check recovery time: %v", err)
	}

	if inRecovery {
		return errors.Errorf("Model %s is still in recovery period", model)
	}

	// Clean up old latency data when recovery period ends
	age := 24 * time.Hour // Keep last 24 hours of data
	if err := mt.client.CleanupOldLatencies(ctx, model, age); err != nil {
		slog.Error("Failed to cleanup old latency data", "error", err)
	}

	return nil
}

// CheckModelHealth returns true if the model is healthy based on its average latency and recovery time
func (mt *Tracker) CheckModelHealth(model string, config model.Config) (bool, error) {
	ctx := context.Background()

	latencyConfig, ok := config.ModelLatency[model]
	if !ok {
		return true, nil // No latency config means model is considered healthy
	}

	// First check if the model is in recovery period
	inRecovery, err := mt.client.CheckRecoveryTime(ctx, model)
	if err != nil {
		return false, fmt.Errorf("failed to check recovery time: %v", err)
	}
	if inRecovery {
		return false, fmt.Errorf("model %s is still in recovery period", model)
	}

	// Get the latency entries first to check if we have enough data
	entries, err := mt.client.GetLatencyEntries(ctx, model, int64(latencyConfig.NoOfCalls))
	if err != nil {
		return false, fmt.Errorf("failed to get latency entries: %v", err)
	}

	// If we don't have enough data points yet, consider the model healthy
	if len(entries) < latencyConfig.NoOfCalls {
		slog.Info("Not enough data points yet",
			"model", model,
			"current", len(entries),
			"required", latencyConfig.NoOfCalls)
		return true, nil
	}

	// Calculate average from the entries we already have
	var totalLatency float64
	for _, latency := range entries {
		totalLatency += latency
	}
	avgLatency := totalLatency / float64(len(entries))

	// If average latency is above threshold, set recovery time and mark as unhealthy
	if avgLatency > latencyConfig.AvgLatencyThreshold {
		if err := mt.RecordRecoveryTime(model, config); err != nil {
			slog.Error("Failed to record recovery time", "error", err)
		}
		return false, fmt.Errorf("model %s is unhealthy: average latency %.2fs exceeds threshold %.2fs (over last %d calls)",
			model, avgLatency, latencyConfig.AvgLatencyThreshold, len(entries))
	}

	return true, nil
}
