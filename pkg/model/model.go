package model

import (
	"net/http"
	"time"
)

type Models interface {
	isModels()
}

type Message map[string]string

type OrderedModels []string

func (OrderedModels) isModels() {}

type WeightedModels map[string]float64

func (WeightedModels) isModels() {}

type clientType string

const (
	ClientTypeAzure  clientType = "azure"
	ClientTypeOpenai clientType = "openai"
)

type RollingAverageLatency struct {
	AvgLatencyThreshold float64
	NoOfCalls           int
	RecoveryTime        time.Duration
}

type ModelLatency map[string]*RollingAverageLatency

type CustomInvalidType struct{}

func (CustomInvalidType) isModels() {}

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
