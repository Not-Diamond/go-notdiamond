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
	models.isModels()     // Actually call the method
}

func TestCustomInvalidType_isModels(t *testing.T) {
	invalidType := CustomInvalidType{}
	var _ Models = invalidType // Verify CustomInvalidType implements Models interface
	invalidType.isModels()     // Actually call the method
}

func TestClientType(t *testing.T) {
	if ClientTypeAzure != "azure" {
		t.Errorf("Expected ClientTypeAzure to be 'azure', got %s", ClientTypeAzure)
	}
	if ClientTypeOpenai != "openai" {
		t.Errorf("Expected ClientTypeOpenai to be 'openai', got %s", ClientTypeOpenai)
	}
}

func TestModelLimits(t *testing.T) {
	limits := ModelLimits{
		MaxNoOfCalls:    100,
		MaxRecoveryTime: 5 * time.Minute,
	}

	if limits.MaxNoOfCalls != 100 {
		t.Errorf("Expected MaxNoOfCalls to be 100, got %d", limits.MaxNoOfCalls)
	}
	if limits.MaxRecoveryTime != 5*time.Minute {
		t.Errorf("Expected MaxRecoveryTime to be 5 minutes, got %v", limits.MaxRecoveryTime)
	}
}

func TestRollingAverageLatency(t *testing.T) {
	ral := &RollingAverageLatency{
		AvgLatencyThreshold: 1.5,
		NoOfCalls:           10,
		RecoveryTime:        30 * time.Second,
	}

	if ral.AvgLatencyThreshold != 1.5 {
		t.Errorf("Expected AvgLatencyThreshold to be 1.5, got %f", ral.AvgLatencyThreshold)
	}
	if ral.NoOfCalls != 10 {
		t.Errorf("Expected NoOfCalls to be 10, got %d", ral.NoOfCalls)
	}
	if ral.RecoveryTime != 30*time.Second {
		t.Errorf("Expected RecoveryTime to be 30 seconds, got %v", ral.RecoveryTime)
	}
}

func TestModelLatency(t *testing.T) {
	ml := ModelLatency{
		"model1": &RollingAverageLatency{
			AvgLatencyThreshold: 1.0,
			NoOfCalls:           5,
			RecoveryTime:        time.Minute,
		},
	}

	if len(ml) != 1 {
		t.Errorf("Expected ModelLatency to have 1 entry, got %d", len(ml))
	}

	if ml["model1"].AvgLatencyThreshold != 1.0 {
		t.Errorf("Expected AvgLatencyThreshold to be 1.0, got %f", ml["model1"].AvgLatencyThreshold)
	}
}

func TestConfig_Complete(t *testing.T) {
	config := Config{
		Clients: []http.Request{{}},
		Models: WeightedModels{
			"model1": 0.7,
			"model2": 0.3,
		},
		MaxRetries: map[string]int{
			"model1": 3,
		},
		Timeout: map[string]float64{
			"model1": 30.0,
		},
		ModelMessages: map[string][]Message{
			"model1": {
				{"role": "system"},
			},
		},
		Backoff: map[string]float64{
			"model1": 1.0,
		},
		StatusCodeRetry: []int{500, 429},
		ModelLatency: ModelLatency{
			"model1": &RollingAverageLatency{
				AvgLatencyThreshold: 1.0,
				NoOfCalls:           5,
				RecoveryTime:        time.Minute,
			},
		},
	}

	// Test Models interface with WeightedModels
	if _, ok := config.Models.(WeightedModels); !ok {
		t.Error("Expected Models to be WeightedModels")
	}

	// Test Message type
	msg := config.ModelMessages["model1"][0]
	if msg["role"] != "system" {
		t.Errorf("Expected message role to be 'system', got %s", msg["role"])
	}
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
