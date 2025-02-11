package test_ordered

import (
	"notdiamond"
)

var OrderedModels = notdiamond.Config{
	Models: notdiamond.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
		"openai/gpt-4o",
	},
}
