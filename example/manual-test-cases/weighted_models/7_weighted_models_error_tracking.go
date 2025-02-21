package test_weighted

import (
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var WeightedModelsWithErrorTracking = model.Config{
	Models: model.WeightedModels{
		"openai/gpt-4o-mini": 0.2,
		"openai/gpt-4o":      0.4,
		"azure/gpt-4o":       0.4,
	},
	ModelErrorTracking: model.ModelErrorTracking{
		"openai/gpt-4o-mini": &model.RollingErrorTracking{
			StatusConfigs: map[int]*model.StatusCodeConfig{
				401: {
					ErrorThresholdPercentage: 80,
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
			},
		},
		"azure/gpt-4o-mini": &model.RollingErrorTracking{
			StatusConfigs: map[int]*model.StatusCodeConfig{
				401: {
					ErrorThresholdPercentage: 80,
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
				500: {
					ErrorThresholdPercentage: 70,
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
				502: {
					ErrorThresholdPercentage: 60,
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
			},
		},
		"azure/gpt-4o": &model.RollingErrorTracking{
			StatusConfigs: map[int]*model.StatusCodeConfig{
				401: {
					ErrorThresholdPercentage: 80,
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
				500: {
					ErrorThresholdPercentage: 70,
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
				502: {
					ErrorThresholdPercentage: 60,
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
			},
		},
	},
}
