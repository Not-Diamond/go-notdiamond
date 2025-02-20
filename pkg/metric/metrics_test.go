package metric

import (
	"context"
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

func TestErrorTracking(t *testing.T) {
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
	config := model.Config{
		ModelErrorTracking: model.ModelErrorTracking{
			modelName: &model.RollingErrorTracking{
				StatusConfigs: map[int]*model.StatusCodeConfig{
					401: {
						ErrorThresholdPercentage: 80,
						NoOfCalls:                5,
						RecoveryTime:             1 * time.Minute,
					},
				},
			},
		},
	}

	// Test recording error codes
	for i := 0; i < 5; i++ {
		if err := tracker.RecordErrorCode(modelName, 401); err != nil {
			t.Errorf("RecordErrorCode() error = %v", err)
		}
	}

	// Test health check after recording errors
	healthy, err := tracker.CheckModelErrorHealth(modelName, config)
	if err == nil {
		t.Error("Expected an error indicating the model is unhealthy")
	}
	if healthy {
		t.Error("Expected model to be unhealthy after 5 401 errors")
	}

	// Test recovery time
	if err := tracker.RecordErrorRecoveryTime(modelName, config, 401); err != nil {
		t.Errorf("RecordErrorRecoveryTime() error = %v", err)
	}

	// Check recovery time - should be in recovery
	err = tracker.CheckErrorRecoveryTime(modelName, config)
	if err == nil {
		t.Error("Expected an error indicating the model is in recovery")
	} else if !strings.Contains(err.Error(), "still in error recovery period") {
		t.Errorf("Expected 'still in error recovery period' error message, got: %v", err)
	}

	// Fast forward time by 2 minutes
	mr.FastForward(2 * time.Minute)

	// Check recovery time - should be recovered
	if err := tracker.CheckErrorRecoveryTime(modelName, config); err != nil {
		t.Errorf("CheckErrorRecoveryTime() error = %v", err)
	}

	// Test health check after recovery
	healthy, err = tracker.CheckModelErrorHealth(modelName, config)
	if err != nil {
		t.Errorf("CheckModelErrorHealth() error = %v", err)
	}
	if !healthy {
		t.Error("Expected model to be healthy after recovery period")
	}
}

func TestErrorTrackingWithMultipleStatusCodes(t *testing.T) {
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
	config := model.Config{
		ModelErrorTracking: model.ModelErrorTracking{
			modelName: &model.RollingErrorTracking{
				StatusConfigs: map[int]*model.StatusCodeConfig{
					401: {
						ErrorThresholdPercentage: 80,
						NoOfCalls:                5,
						RecoveryTime:             1 * time.Minute,
					},
					500: {
						ErrorThresholdPercentage: 60,
						NoOfCalls:                3,
						RecoveryTime:             30 * time.Second,
					},
				},
			},
		},
	}

	// Test 401 errors (not enough to trigger recovery)
	for i := 0; i < 3; i++ {
		if err := tracker.RecordErrorCode(modelName, 401); err != nil {
			t.Errorf("RecordErrorCode() error = %v", err)
		}
	}

	// Check health - should still be healthy
	healthy, err := tracker.CheckModelErrorHealth(modelName, config)
	if err != nil {
		t.Errorf("CheckModelErrorHealth() error = %v", err)
	}
	if !healthy {
		t.Error("Expected model to be healthy with only 3 401 errors")
	}

	// Test 500 errors (enough to trigger recovery)
	for i := 0; i < 3; i++ {
		if err := tracker.RecordErrorCode(modelName, 500); err != nil {
			t.Errorf("RecordErrorCode() error = %v", err)
		}
	}

	// Check health - should be unhealthy due to 500 errors
	healthy, err = tracker.CheckModelErrorHealth(modelName, config)
	if err == nil {
		t.Error("Expected an error indicating the model is unhealthy")
	}
	if healthy {
		t.Error("Expected model to be unhealthy after 3 500 errors")
	}

	// Fast forward time by 1 minute
	mr.FastForward(1 * time.Minute)

	// Check health - should be healthy again
	healthy, err = tracker.CheckModelErrorHealth(modelName, config)
	if err != nil {
		t.Errorf("CheckModelErrorHealth() error = %v", err)
	}
	if !healthy {
		t.Error("Expected model to be healthy after recovery period")
	}
}

func TestErrorTrackingEdgeCases(t *testing.T) {
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
	config := model.Config{
		ModelErrorTracking: model.ModelErrorTracking{
			modelName: &model.RollingErrorTracking{
				StatusConfigs: map[int]*model.StatusCodeConfig{
					401: {
						ErrorThresholdPercentage: 80,
						NoOfCalls:                5,
						RecoveryTime:             1 * time.Minute,
					},
				},
			},
		},
	}

	// Test recording error code for non-configured status code
	if err := tracker.RecordErrorCode(modelName, 404); err != nil {
		t.Errorf("RecordErrorCode() error = %v", err)
	}

	// Check health - should be healthy since 404 is not configured
	healthy, err := tracker.CheckModelErrorHealth(modelName, config)
	if err != nil {
		t.Errorf("CheckModelErrorHealth() error = %v", err)
	}
	if !healthy {
		t.Error("Expected model to be healthy with unconfigured status code")
	}

	// Test recording recovery time for non-configured status code
	err = tracker.RecordErrorRecoveryTime(modelName, config, 404)
	if err == nil {
		t.Error("Expected an error when recording recovery time for unconfigured status code")
	}

	// Test health check for non-existent model
	healthy, err = tracker.CheckModelErrorHealth("non-existent-model", config)
	if err != nil {
		t.Errorf("CheckModelErrorHealth() error = %v", err)
	}
	if !healthy {
		t.Error("Expected non-existent model to be considered healthy")
	}
}

func TestCheckModelOverallHealth(t *testing.T) {
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
	config := model.Config{
		ModelLatency: model.ModelLatency{
			modelName: &model.RollingAverageLatency{
				AvgLatencyThreshold: 1.0,
				NoOfCalls:           2,
				RecoveryTime:        time.Second,
			},
		},
		ModelErrorTracking: model.ModelErrorTracking{
			modelName: &model.RollingErrorTracking{
				StatusConfigs: map[int]*model.StatusCodeConfig{
					401: {
						ErrorThresholdPercentage: 80,
						NoOfCalls:                2,
						RecoveryTime:             time.Second,
					},
				},
			},
		},
	}

	clearAllData := func() {
		ctx := context.Background()
		// Clear all data for the model
		if err := tracker.client.ClearAllModelData(ctx, modelName); err != nil {
			t.Errorf("Failed to clear model data: %v", err)
		}
		// Fast forward time to clear any remaining recovery periods
		mr.FastForward(2 * time.Second)
	}

	tests := []struct {
		name           string
		setupFunc      func()
		expectedHealth bool
		expectError    bool
		errorContains  string
	}{
		{
			name: "both healthy",
			setupFunc: func() {
				clearAllData()
				// Record good latencies
				tracker.RecordLatency(modelName, 0.5, "success")
				tracker.RecordLatency(modelName, 0.7, "success")
				// Record some errors but below threshold
				tracker.RecordErrorCode(modelName, 401)
				tracker.RecordErrorCode(modelName, 200)
			},
			expectedHealth: true,
			expectError:    false,
		},
		{
			name: "latency unhealthy",
			setupFunc: func() {
				clearAllData()
				// Record bad latencies
				tracker.RecordLatency(modelName, 1.5, "success")
				tracker.RecordLatency(modelName, 1.7, "success")
			},
			expectedHealth: false,
			expectError:    true,
			errorContains:  "unhealthy: average latency",
		},
		{
			name: "errors unhealthy",
			setupFunc: func() {
				clearAllData()
				// Record good latencies
				tracker.RecordLatency(modelName, 0.5, "success")
				tracker.RecordLatency(modelName, 0.7, "success")
				// Record errors above threshold
				tracker.RecordErrorCode(modelName, 401)
				tracker.RecordErrorCode(modelName, 401)
			},
			expectedHealth: false,
			expectError:    true,
			errorContains:  "unhealthy: status code 401",
		},
		{
			name: "both unhealthy",
			setupFunc: func() {
				clearAllData()
				// Record bad latencies
				tracker.RecordLatency(modelName, 1.5, "success")
				tracker.RecordLatency(modelName, 1.7, "success")
				// Record errors above threshold
				tracker.RecordErrorCode(modelName, 401)
				tracker.RecordErrorCode(modelName, 401)
			},
			expectedHealth: false,
			expectError:    true,
			errorContains:  "unhealthy: average latency",
		},
		{
			name: "no data is healthy",
			setupFunc: func() {
				clearAllData()
			},
			expectedHealth: true,
			expectError:    false,
		},
		{
			name: "in latency recovery",
			setupFunc: func() {
				clearAllData()
				// Record bad latencies and set recovery
				tracker.RecordLatency(modelName, 1.5, "success")
				tracker.RecordLatency(modelName, 1.7, "success")
				tracker.RecordRecoveryTime(modelName, config)
			},
			expectedHealth: false,
			expectError:    true,
			errorContains:  "still in recovery period",
		},
		{
			name: "in error recovery",
			setupFunc: func() {
				clearAllData()
				// Record errors and set recovery
				tracker.RecordErrorCode(modelName, 401)
				tracker.RecordErrorCode(modelName, 401)
				tracker.RecordErrorRecoveryTime(modelName, config, 401)
			},
			expectedHealth: false,
			expectError:    true,
			errorContains:  "still in error recovery period",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFunc()

			healthy, err := tracker.CheckModelOverallHealth(modelName, config)
			if tt.expectError {
				if err == nil {
					t.Error("Expected an error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %v", tt.errorContains, err)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if healthy != tt.expectedHealth {
				t.Errorf("Expected health = %v, got %v", tt.expectedHealth, healthy)
			}
		})
	}
}
