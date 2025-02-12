package test_ordered

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var OrderedModels = model.Config{
	Models: model.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
		"openai/gpt-4o",
	},
}
