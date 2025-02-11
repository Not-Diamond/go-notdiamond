package test_ordered

import (
	"github.com/Not-Diamond/go-notdiamond/types"
)

var OrderedModelsWithTimeout = types.Config{
	Models: types.OrderedModels{
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
