package test_weighted

import "github.com/Not-Diamond/go-notdiamond/pkg/model"

var WeightedModelsWithMaxRetries = model.Config{
	Models: model.WeightedModels{
		"azure/gpt-4o-mini":  0.1,
		"openai/gpt-4o-mini": 0.1,
		"openai/gpt-4o":      0.7,
		"azure/gpt-4o":       0.1,
	},
	MaxRetries: map[string]int{
		"openai/gpt-4o-mini": 2,
		"azure/gpt-4o-mini":  2,
		"azure/gpt-4o":       2,
		"openai/gpt-4o":      2,
	},
}
