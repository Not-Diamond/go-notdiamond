package test_region_fallback

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

// RegionFallbackVertexTest demonstrates region fallback for Vertex AI
// If a request to us-east fails, it will fallback to us-west
var RegionFallbackVertexTest = model.Config{
	Models: model.OrderedModels{
		"vertex/gemini-pro/us-east4",    // Try us-east4 first
		"vertex/gemini-pro/us-west1",    // Fallback to us-west1
		"vertex/gemini-pro/us-central1", // Final fallback to us-central1
	},
}

// RegionFallbackOpenAITest demonstrates region fallback for OpenAI
// Note: OpenAI has limited region support (us and eu)
var RegionFallbackOpenAITest = model.Config{
	Models: model.OrderedModels{
		"openai/gpt-4o-mini/eu",   // Try EU region first
		"openai/gpt-4o-mini/us",   // Try US region first
		"openai/gpt-3.5-turbo/eu", // Try EU region first
		"openai/gpt-3.5-turbo/us", // Try US region first
		"openai/gpt-3.5-turbo",    // Fallback to default region (US)
	},
}

// RegionFallbackAzureTest demonstrates region fallback for Azure
var RegionFallbackAzureTest = model.Config{
	Models: model.OrderedModels{
		"azure/gpt-35-turbo/eastus",     // Try eastus first
		"azure/gpt-35-turbo/westus",     // Fallback to westus
		"azure/gpt-35-turbo/westeurope", // Final fallback to westeurope
	},
	AzureAPIVersion: "2023-05-15", // Specify Azure API version
}

// RegionFallbackMixedTest demonstrates region fallback across different providers
var RegionFallbackMixedTest = model.Config{
	Models: model.OrderedModels{
		"azure/gpt-4o-mini",
		"azure/gpt-35-turbo/eastus", // Fallback to Azure in eastus
		"openai/gpt-4o-mini",
		"vertex/gemini-pro/us-east4",    // Try Vertex in us-east4 first
		"openai/gpt-3.5-turbo/eu",       // Fallback to OpenAI in EU
		"vertex/gemini-pro/us-central1", // Final fallback to Vertex in us-central1
	},
	AzureAPIVersion: "2023-05-15", // Specify Azure API version
}
