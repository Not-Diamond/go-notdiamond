package test_region_fallback

import (
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

// RegionFallbackBedrockTest demonstrates region fallback for AWS Bedrock
var RegionFallbackBedrockTest = model.Config{
	Models: model.OrderedModels{
		"bedrock/anthropic.claude-3-sonnet-20240229-v1:0/us-east-1", // Try us-east-1 first
		"bedrock/anthropic.claude-3-sonnet-20240229-v1:0/us-west-2", // Fallback to us-west-2
		"vertex/gemini-pro/us-central1",                             // Final fallback to Vertex
		"openai/gpt-3.5-turbo",                                      // Final fallback to OpenAI
	},
	BedrockRegions: map[string]string{
		"us-east-1": "https://bedrock-runtime.us-east-1.amazonaws.com",
		"us-west-2": "https://bedrock-runtime.us-west-2.amazonaws.com",
	},
}

// RegionFallbackMixedWithBedrockTest adds Bedrock to the mixed provider test
var RegionFallbackMixedWithBedrockTest = model.Config{
	Models: model.OrderedModels{
		"bedrock/anthropic.claude-3-sonnet-20240229-v1:0/us-east-1", // Try Bedrock in us-east-1 first
		"azure/gpt-35-turbo/eastus",                                 // Fallback to Azure in eastus
		"vertex/gemini-pro/us-east4",                                // Fallback to Vertex in us-east4
		"openai/gpt-3.5-turbo",                                      // Final fallback to OpenAI
	},
	AzureAPIVersion: "2023-05-15", // Specify Azure API version
	AzureRegions: map[string]string{
		"eastus":     "https://notdiamond-azure-openai.openai.azure.com",
		"westeurope": "https://custom-westeurope.openai.azure.com",
	},
	BedrockRegions: map[string]string{
		"us-east-1": "https://bedrock-runtime.us-east-1.amazonaws.com",
		"us-west-2": "https://bedrock-runtime.us-west-2.amazonaws.com",
	},
}
