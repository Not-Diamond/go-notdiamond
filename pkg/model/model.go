package model

import (
	"net/http"
	"time"
)

// Models is a type that can be used to represent a list of models.
type Models interface {
	isModels()
}

// Message is a map of strings.
type Message map[string]string

// OrderedModels is a type that can be used to represent a list of models.
type OrderedModels []string

func (OrderedModels) isModels() {}

// WeightedModels is a type that can be used to represent a list of models.
type WeightedModels map[string]float64

func (WeightedModels) isModels() {}

// ClientType is a type that can be used to represent a client type.
type clientType string

const (
	ClientTypeAzure  clientType = "azure"
	ClientTypeOpenai clientType = "openai"
)

// RollingAverageLatency is a type that can be used to represent a rolling average latency.
type RollingAverageLatency struct {
	AvgLatencyThreshold float64
	NoOfCalls           int
	RecoveryTime        time.Duration
}

// ModelLatency is a type that can be used to represent a model latency.
type ModelLatency map[string]*RollingAverageLatency

// CustomInvalidType is a type that can be used to represent a custom invalid type.
type CustomInvalidType struct{}

func (CustomInvalidType) isModels() {}

// Config is the configuration for the NotDiamond client.
type Config struct {
	Clients         []http.Request
	Models          Models
	MaxRetries      map[string]int
	Timeout         map[string]float64
	ModelMessages   map[string][]Message
	Backoff         map[string]float64
	StatusCodeRetry interface{}
	ModelLatency    ModelLatency
}
