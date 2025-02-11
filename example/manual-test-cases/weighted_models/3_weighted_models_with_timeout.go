package test_weighted

import (
	"notdiamond"
)

var WeightedModelsWithTimeout = notdiamond.Config{
	Models: notdiamond.WeightedModels{
		"azure/gpt-4o-mini":  0.1,
		"openai/gpt-4o-mini": 0.1,
		"openai/gpt-4o":      0.7,
		"azure/gpt-4o":       0.1,
	},
	Timeout: map[string]float64{
		"azure/gpt-4o-mini":  10,
		"openai/gpt-4o-mini": 10,
		"azure/gpt-4o":       10,
		"openai/gpt-4o":      10,
	},
}
