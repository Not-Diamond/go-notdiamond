package test_weighted

import (
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var WeightedModelsWithLatency = model.Config{
	Models: model.WeightedModels{
		"openai/gpt-4o-mini": 0.2,
		"openai/gpt-4o":      0.4,
		"azure/gpt-4o":       0.4,
	},
	ModelLatency: model.ModelLatency{
		"openai/gpt-4o-mini": &model.RollingAverageLatency{
			AvgLatencyThreshold: 0.5,
			NoOfCalls:           5,
			RecoveryTime:        1 * time.Minute,
		},
		"azure/gpt-4o-mini": &model.RollingAverageLatency{
			AvgLatencyThreshold: 6,
			NoOfCalls:           10,
			RecoveryTime:        1 * time.Minute,
		},
		"azure/gpt-4o": &model.RollingAverageLatency{
			AvgLatencyThreshold: 3.2,
			NoOfCalls:           10,
			RecoveryTime:        3 * time.Second,
		},
	},
}
