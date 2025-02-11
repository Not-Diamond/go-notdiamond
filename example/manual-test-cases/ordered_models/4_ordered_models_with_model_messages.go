package test_ordered

import (
	"github.com/Not-Diamond/go-notdiamond/types"
)

var OrderedModelsWithModelMessages = types.Config{
	Models: types.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
	},
	ModelMessages: map[string][]types.Message{
		"openai/gpt-4o-mini": {
			{"role": "user", "content": "Please respond only with answer in romanian."},
		},
		"azure/gpt-4o-mini": {
			{"role": "user", "content": "Please respond only with answer in spanish."},
		},
		"azure/gpt-4o": {
			{"role": "user", "content": "Please respond only with answer in french."},
		},
	},
}
