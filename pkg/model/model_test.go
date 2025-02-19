package model

import (
	"net/http"
	"testing"
	"time"
)

func TestOrderedModels_isModels(t *testing.T) {
	models := OrderedModels{"model1", "model2"}
	var _ Models = models // Verify OrderedModels implements Models interface
	models.isModels()     // Actually call the method
}

func TestWeightedModels_isModels(t *testing.T) {
	models := WeightedModels{"model1": 0.5, "model2": 0.5}
	var _ Models = models // Verify WeightedModels implements Models interface
}

func TestCustomInvalidType_isModels(t *testing.T) {
	invalidType := CustomInvalidType{}
	var _ Models = invalidType // Verify CustomInvalidType implements Models interface
}

func TestConfig_Validation(t *testing.T) {
	// This is a basic structure test to ensure Config can hold all required fields
	config := Config{
		Clients: []http.Request{
			{},
		},
		Models: OrderedModels{"model1", "model2"},
		MaxRetries: map[string]int{
			"model1": 3,
		},
		Timeout: map[string]float64{
			"model1": 30.0,
		},
		ModelMessages: map[string][]Message{
			"model1": {
				{"role": "system", "content": "You are a helpful assistant"},
			},
		},
		Backoff: map[string]float64{
			"model1": 0.1,
		},
		StatusCodeRetry: []int{429, 500},
		ModelLatency: ModelLatency{
			"model1": &RollingAverageLatency{
				AvgLatencyThreshold: 3.5,
				NoOfCalls:           5,
				RecoveryTime:        time.Minute,
			},
		},
	}

	// Verify that all fields are accessible and of the correct type
	if len(config.Clients) != 1 {
		t.Error("Expected 1 client")
	}

	if _, ok := config.Models.(OrderedModels); !ok {
		t.Error("Expected Models to be OrderedModels")
	}

	if retries, ok := config.MaxRetries["model1"]; !ok || retries != 3 {
		t.Error("Expected MaxRetries to contain model1 with value 3")
	}

	if timeout, ok := config.Timeout["model1"]; !ok || timeout != 30.0 {
		t.Error("Expected Timeout to contain model1 with value 30.0")
	}

	if messages, ok := config.ModelMessages["model1"]; !ok || len(messages) != 1 {
		t.Error("Expected ModelMessages to contain model1 with 1 message")
	}

	if backoff, ok := config.Backoff["model1"]; !ok || backoff != 0.1 {
		t.Error("Expected Backoff to contain model1 with value 0.1")
	}

	if latency, ok := config.ModelLatency["model1"]; !ok || latency.NoOfCalls != 5 {
		t.Error("Expected ModelLatency to contain model1 with NoOfCalls 5")
	}
}

func TestMessage_Usage(t *testing.T) {
	message := Message{
		"role":    "system",
		"content": "You are a helpful assistant",
	}

	if message["role"] != "system" {
		t.Error("Expected role to be 'system'")
	}

	if message["content"] != "You are a helpful assistant" {
		t.Error("Expected correct content")
	}

	// Test map behavior
	message["new_field"] = "test"
	if message["new_field"] != "test" {
		t.Error("Expected to be able to add new fields")
	}

	delete(message, "new_field")
	if _, exists := message["new_field"]; exists {
		t.Error("Expected to be able to delete fields")
	}
}

func TestRollingAverageLatency_Usage(t *testing.T) {
	latency := &RollingAverageLatency{
		AvgLatencyThreshold: 3.5,
		NoOfCalls:           5,
		RecoveryTime:        time.Minute,
	}

	if latency.AvgLatencyThreshold != 3.5 {
		t.Error("Expected AvgLatencyThreshold to be 3.5")
	}

	if latency.NoOfCalls != 5 {
		t.Error("Expected NoOfCalls to be 5")
	}

	if latency.RecoveryTime != time.Minute {
		t.Error("Expected RecoveryTime to be 1 minute")
	}
}

func TestStatusCodeConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *StatusCodeConfig
		expected struct {
			threshold    float64
			calls        int
			recoveryTime time.Duration
		}
	}{
		{
			name: "basic config",
			config: &StatusCodeConfig{
				ErrorThresholdPercentage: 80,
				NoOfCalls:                5,
				RecoveryTime:             1 * time.Minute,
			},
			expected: struct {
				threshold    float64
				calls        int
				recoveryTime time.Duration
			}{
				threshold:    80,
				calls:        5,
				recoveryTime: 1 * time.Minute,
			},
		},
		{
			name:   "zero values",
			config: &StatusCodeConfig{},
			expected: struct {
				threshold    float64
				calls        int
				recoveryTime time.Duration
			}{
				threshold:    0,
				calls:        0,
				recoveryTime: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.ErrorThresholdPercentage != tt.expected.threshold {
				t.Errorf("ErrorThresholdPercentage = %v, want %v", tt.config.ErrorThresholdPercentage, tt.expected.threshold)
			}
			if tt.config.NoOfCalls != tt.expected.calls {
				t.Errorf("NoOfCalls = %v, want %v", tt.config.NoOfCalls, tt.expected.calls)
			}
			if tt.config.RecoveryTime != tt.expected.recoveryTime {
				t.Errorf("RecoveryTime = %v, want %v", tt.config.RecoveryTime, tt.expected.recoveryTime)
			}
		})
	}
}

func TestRollingErrorTracking(t *testing.T) {
	tests := []struct {
		name     string
		tracking *RollingErrorTracking
		wantLen  int
	}{
		{
			name: "empty config",
			tracking: &RollingErrorTracking{
				StatusConfigs: make(map[int]*StatusCodeConfig),
			},
			wantLen: 0,
		},
		{
			name: "single status code",
			tracking: &RollingErrorTracking{
				StatusConfigs: map[int]*StatusCodeConfig{
					401: {
						ErrorThresholdPercentage: 80,
						NoOfCalls:                5,
						RecoveryTime:             1 * time.Minute,
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "multiple status codes",
			tracking: &RollingErrorTracking{
				StatusConfigs: map[int]*StatusCodeConfig{
					401: {
						ErrorThresholdPercentage: 80,
						NoOfCalls:                5,
						RecoveryTime:             1 * time.Minute,
					},
					500: {
						ErrorThresholdPercentage: 70,
						NoOfCalls:                5,
						RecoveryTime:             1 * time.Minute,
					},
				},
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.tracking.StatusConfigs) != tt.wantLen {
				t.Errorf("StatusConfigs length = %v, want %v", len(tt.tracking.StatusConfigs), tt.wantLen)
			}
		})
	}
}

func TestModelErrorTracking(t *testing.T) {
	tests := []struct {
		name     string
		tracking ModelErrorTracking
		wantLen  int
	}{
		{
			name:     "empty tracking",
			tracking: make(ModelErrorTracking),
			wantLen:  0,
		},
		{
			name: "single model",
			tracking: ModelErrorTracking{
				"openai/gpt-4": &RollingErrorTracking{
					StatusConfigs: map[int]*StatusCodeConfig{
						401: {
							ErrorThresholdPercentage: 80,
							NoOfCalls:                5,
							RecoveryTime:             1 * time.Minute,
						},
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "multiple models",
			tracking: ModelErrorTracking{
				"openai/gpt-4": &RollingErrorTracking{
					StatusConfigs: map[int]*StatusCodeConfig{
						401: {
							ErrorThresholdPercentage: 80,
							NoOfCalls:                5,
							RecoveryTime:             1 * time.Minute,
						},
					},
				},
				"azure/gpt-4": &RollingErrorTracking{
					StatusConfigs: map[int]*StatusCodeConfig{
						500: {
							ErrorThresholdPercentage: 70,
							NoOfCalls:                5,
							RecoveryTime:             1 * time.Minute,
						},
					},
				},
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.tracking) != tt.wantLen {
				t.Errorf("ModelErrorTracking length = %v, want %v", len(tt.tracking), tt.wantLen)
			}
		})
	}
}
