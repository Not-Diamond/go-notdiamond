package test_ordered

import (
	"notdiamond"
)

var OrderedModelsWithStatusCodeRetry = notdiamond.Config{
	Models: notdiamond.OrderedModels{
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
