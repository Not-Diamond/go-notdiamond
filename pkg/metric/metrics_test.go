package metric

import (
	"strings"
	"testing"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/alicebob/miniredis/v2"
)

func setupTestRedis(t *testing.T) (string, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}

	return mr.Addr(), func() {
		mr.Close()
	}
}

func TestNewTracker(t *testing.T) {
	redisAddr, cleanup := setupTestRedis(t)
	defer cleanup()

	tracker, err := NewTracker(redisAddr)
	if err != nil {
		t.Fatalf("NewTracker() error = %v", err)
	}
	defer tracker.Close()

	if tracker.client == nil {
		t.Error("Expected non-nil Redis client")
	}
}

func TestRecordAndCheckLatency(t *testing.T) {
	redisAddr, cleanup := setupTestRedis(t)
	defer cleanup()

	tracker, err := NewTracker(redisAddr)
	if err != nil {
		t.Fatalf("NewTracker() error = %v", err)
	}
	defer tracker.Close()

	modelName := "test-model"
	latencyConfig := &model.RollingAverageLatency{
		AvgLatencyThreshold: 1.0,
		NoOfCalls:           2,
		RecoveryTime:        time.Second,
	}
	config := model.Config{
		ModelLatency: model.ModelLatency{
			modelName: latencyConfig,
		},
	}

	// Record some latencies
	if err := tracker.RecordLatency(modelName, 0.5, "success"); err != nil {
		t.Errorf("RecordLatency() error = %v", err)
	}
	if err := tracker.RecordLatency(modelName, 0.7, "success"); err != nil {
		t.Errorf("RecordLatency() error = %v", err)
	}

	// Check health - should be healthy as average is below threshold
	healthy, err := tracker.CheckModelHealth(modelName, config)
	if err != nil {
		t.Errorf("CheckModelHealth() error = %v", err)
	}
	if !healthy {
		t.Error("Expected model to be healthy")
	}

	// Record high latency
	if err := tracker.RecordLatency(modelName, 2.0, "success"); err != nil {
		t.Errorf("RecordLatency() error = %v", err)
	}

	// Check health - should be unhealthy as average is above threshold
	healthy, err = tracker.CheckModelHealth(modelName, config)
	if err == nil {
		t.Error("Expected an error indicating the model is unhealthy")
	} else if !strings.Contains(err.Error(), "unhealthy") {
		t.Errorf("Expected unhealthy error message, got: %v", err)
	}
	if healthy {
		t.Error("Expected model to be unhealthy")
	}
}

func TestRecoveryTime(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	tracker, err := NewTracker(mr.Addr())
	if err != nil {
		t.Fatalf("NewTracker() error = %v", err)
	}
	defer tracker.Close()

	modelName := "test-model"
	latencyConfig := &model.RollingAverageLatency{
		RecoveryTime: time.Second,
	}
	config := model.Config{
		ModelLatency: model.ModelLatency{
			modelName: latencyConfig,
		},
	}

	// Record recovery time
	if err := tracker.RecordRecoveryTime(modelName, config); err != nil {
		t.Errorf("RecordRecoveryTime() error = %v", err)
	}

	// Check recovery time - should be in recovery
	err = tracker.CheckRecoveryTime(modelName, config)
	if err == nil {
		t.Error("Expected an error indicating the model is in recovery")
	} else if !strings.Contains(err.Error(), "still in recovery period") {
		t.Errorf("Expected 'still in recovery period' error message, got: %v", err)
	}

	// Fast forward time by 2 seconds
	mr.FastForward(2 * time.Second)

	// Check recovery time - should be recovered
	if err := tracker.CheckRecoveryTime(modelName, config); err != nil {
		t.Errorf("CheckRecoveryTime() error = %v", err)
	}
}

func TestCheckModelHealth_NoConfig(t *testing.T) {
	redisAddr, cleanup := setupTestRedis(t)
	defer cleanup()

	tracker, err := NewTracker(redisAddr)
	if err != nil {
		t.Fatalf("NewTracker() error = %v", err)
	}
	defer tracker.Close()

	modelName := "test-model"
	config := model.Config{
		ModelLatency: model.ModelLatency{}, // Empty config
	}

	// Model should be considered healthy when no config exists
	healthy, err := tracker.CheckModelHealth(modelName, config)
	if err != nil {
		t.Errorf("CheckModelHealth() error = %v", err)
	}
	if !healthy {
		t.Error("Expected model to be healthy when no config exists")
	}
}

func TestCheckModelHealth_NoData(t *testing.T) {
	redisAddr, cleanup := setupTestRedis(t)
	defer cleanup()

	tracker, err := NewTracker(redisAddr)
	if err != nil {
		t.Fatalf("NewTracker() error = %v", err)
	}
	defer tracker.Close()

	modelName := "test-model"
	latencyConfig := &model.RollingAverageLatency{
		AvgLatencyThreshold: 1.0,
		NoOfCalls:           2,
		RecoveryTime:        time.Second,
	}
	config := model.Config{
		ModelLatency: model.ModelLatency{
			modelName: latencyConfig,
		},
	}

	// Model should be considered healthy when no data exists
	healthy, err := tracker.CheckModelHealth(modelName, config)
	if err != nil {
		t.Errorf("CheckModelHealth() error = %v", err)
	}
	if !healthy {
		t.Error("Expected model to be healthy when no data exists")
	}
}
