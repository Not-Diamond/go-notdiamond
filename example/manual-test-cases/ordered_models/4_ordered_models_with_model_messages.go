package test_ordered

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var OrderedModelsWithModelMessages = model.Config{
	Models: model.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
	},
	ModelMessages: map[string][]model.Message{
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
