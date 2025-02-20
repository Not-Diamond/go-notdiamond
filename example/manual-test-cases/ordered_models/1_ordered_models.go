package test_ordered

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var OrderedModels = model.Config{
	Models: model.OrderedModels{
		"vertex/gemini-pro",
		"openai/gpt-4o",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
	},
}
