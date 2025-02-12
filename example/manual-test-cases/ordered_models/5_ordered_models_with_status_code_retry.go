package test_ordered

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var OrderedModelsWithStatusCodeRetry = model.Config{
	Models: model.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
	},
	MaxRetries: map[string]int{
		"openai/gpt-4o-mini": 2,
		"azure/gpt-4o-mini":  2,
		"azure/gpt-4o":       2,
	},
	StatusCodeRetry: map[string]int{
		"401": 1,
		"429": 1,
		"500": 2,
	},
}
