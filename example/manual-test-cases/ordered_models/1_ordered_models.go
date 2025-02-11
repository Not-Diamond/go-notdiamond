package test_ordered

import (
	"github.com/Not-Diamond/go-notdiamond/types"
)

var OrderedModels = types.Config{
	Models: types.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
		"openai/gpt-4o",
	},
}
