package test_ordered

import (
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

var OrderedModelsWithErrorTracking = model.Config{
	Models: model.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
	},
	MaxRetries: map[string]int{
		"openai/gpt-4o-mini": 1,
		"azure/gpt-4o-mini":  3,
		"azure/gpt-4o":       3,
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
	ModelLimits: model.ModelLimits{
		MaxNoOfCalls:    10000,
		MaxRecoveryTime: time.Hour * 24,
	},
}
