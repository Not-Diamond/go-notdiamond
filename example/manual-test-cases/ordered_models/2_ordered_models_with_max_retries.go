package test_ordered

import (
	"notdiamond"
)

var OrderedModelsWithMaxRetries = notdiamond.Config{
	Models: notdiamond.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
		"openai/gpt-4o",
	},
	MaxRetries: map[string]int{
		"openai/gpt-4o-mini": 1,
		"azure/gpt-4o-mini":  2,
		"azure/gpt-4o":       2,
		"openai/gpt-4o":      1,
	},
}

type Model struct {
	Provider string
	Model    string
}
