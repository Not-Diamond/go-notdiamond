package test_weighted

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var WeightedModelsWithStatusCodeRetry = model.Config{
	Models: model.WeightedModels{
		"azure/gpt-4o-mini":  0.1,
		"openai/gpt-4o-mini": 0.1,
		"openai/gpt-4o":      0.7,
		"azure/gpt-4o":       0.1,
	},
	StatusCodeRetry: map[string]map[string]int{
		"openai/gpt-4o-mini": {
			"401": 1,
		},
		"azure/gpt-4o-mini": {
			"401": 2,
		},
		"azure/gpt-4o": {
			"401": 1,
		},
		"openai/gpt-4o": {
			"401": 1,
		},
	},
}
