package test_weighted

import (
	"notdiamond"
)

var WeightedModels = notdiamond.Config{
	Models: notdiamond.WeightedModels{
		"azure/gpt-4o-mini":  0.1,
		"openai/gpt-4o-mini": 0.4,
		"openai/gpt-4o":      0.2,
		"azure/gpt-4o":       0.3,
	},
}
