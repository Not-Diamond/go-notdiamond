package test_ordered

import (
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/Not-Diamond/go-notdiamond/pkg/redis"
)

var OrderedModels = model.Config{
	Models: model.OrderedModels{
		"openai/gpt-4o-mini",
		"azure/gpt-4o-mini",
		"azure/gpt-4o",
		"openai/gpt-4o",
	},
	ModelLatency: model.ModelLatency{
		"openai/gpt-4o-mini": &model.RollingAverageLatency{
			AvgLatencyThreshold: 0.5,
			NoOfCalls:           5,
			RecoveryTime:        1 * time.Minute,
		},
		"azure/gpt-4o-mini": &model.RollingAverageLatency{
			AvgLatencyThreshold: 3.2,
			NoOfCalls:           10,
			RecoveryTime:        3 * time.Second,
		},
		"azure/gpt-4o": &model.RollingAverageLatency{
			AvgLatencyThreshold: 3.2,
			NoOfCalls:           10,
			RecoveryTime:        3 * time.Second,
		},
		"openai/gpt-4o": &model.RollingAverageLatency{
			AvgLatencyThreshold: 3.2,
			NoOfCalls:           10,
			RecoveryTime:        3 * time.Second,
		},
	},
	RedisConfig: &redis.Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	},
}
