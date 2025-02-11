package test_weighted

import (
	"github.com/Not-Diamond/go-notdiamond/types"
)

var WeightedModelsWithModelMessages = types.Config{
	Models: types.WeightedModels{
		"azure/gpt-4o-mini":  0.1,
		"openai/gpt-4o-mini": 0.1,
		"openai/gpt-4o":      0.7,
		"azure/gpt-4o":       0.1,
	},
	ModelMessages: map[string][]types.Message{
		"azure/gpt-4o-mini": {
			{"role": "user", "content": "Please respond only with answer in spanish."},
		},
		"openai/gpt-4o-mini": {
			{"role": "user", "content": "Please respond only with answer in romanian."},
		},
		"azure/gpt-4o": {
			{"role": "user", "content": "Please respond only with answer in french."},
		},
		"openai/gpt-4o": {
			{"role": "user", "content": "Please respond only with answer in english."},
		},
	},
}
