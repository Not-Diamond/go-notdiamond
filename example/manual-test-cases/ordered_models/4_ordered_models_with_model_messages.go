package test_ordered

import (
	"notdiamond"
)

var OrderedModelsWithModelMessages = notdiamond.Config{
	Models: notdiamond.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
	},
	ModelMessages: map[string][]notdiamond.Message{
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
