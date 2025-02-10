package notdiamond

import (
	"net/http"
	"time"
)

type models interface {
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
	ClientTypeOpenAI clientType = "openai"
)

type Config struct {
	Clients             []http.Request
	Models              models
	MaxRetries          map[string]int
	Timeout             map[string]float64
	ModelMessages       map[string][]Message
	Backoff             map[string]float64
	StatusCodeRetry     interface{}
	AvgLatencyThreshold float64
	NoOfCalls           int
	RecoveryTime        time.Duration
}
