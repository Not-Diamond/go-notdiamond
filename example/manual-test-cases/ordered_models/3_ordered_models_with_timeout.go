package test_ordered

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var OrderedModelsWithTimeout = model.Config{
	Models: model.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
	},
	Timeout: map[string]float64{
		"openai/gpt-4o-mini": 0.05,
		"azure/gpt-4o-mini":  10,
		"azure/gpt-4o":       10,
	},
}
