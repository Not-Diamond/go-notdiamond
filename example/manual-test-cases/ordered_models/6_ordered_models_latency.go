package test_ordered

import (
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var OrderedModelsWithLatency = model.Config{
	Models: model.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
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
	ModelLimits: model.ModelLimits{
		MaxNoOfCalls:    10000,
		MaxRecoveryTime: time.Hour * 24,
	},
}
