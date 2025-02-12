package test_weighted

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var WeightedModels = model.Config{
	Models: model.WeightedModels{
		// "azure/gpt-4o-mini":  0.1,
		"openai/gpt-4o-mini": 0.2,
		"openai/gpt-4o":      0.4,
		"azure/gpt-4o":       0.4,
	},
}
