package config

import (
	test_region_fallback "example/manual-test-cases/region_fallback"
	"log"
	"os"
	"path/filepath"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/Not-Diamond/go-notdiamond/pkg/redis"
	"github.com/joho/godotenv"
)

// Config holds the example configuration
type Config struct {
	OpenAIAPIKey     string
	AzureAPIKey      string
	AzureEndpoint    string
	AzureAPIVersion  string
	OpenAIAPIVersion string
	VertexProjectID  string
	VertexLocation   string
	RedisConfig      redis.Config
	AzureRegions     map[string]string
	// AWS Bedrock Configuration
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string
	BedrockRegions     map[string]string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() Config {
	// Load .env file from the example directory
	envPath := filepath.Join(".env")
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Set default Redis address if not provided
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Set default Vertex location if not provided
	vertexLocation := os.Getenv("VERTEX_LOCATION")
	if vertexLocation == "" {
		vertexLocation = "us-central1"
	}

	cfg := Config{
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		AzureAPIKey:      os.Getenv("AZURE_API_KEY"),
		AzureEndpoint:    os.Getenv("AZURE_ENDPOINT"),
		AzureAPIVersion:  os.Getenv("AZURE_API_VERSION"),
		OpenAIAPIVersion: os.Getenv("OPENAI_API_VERSION"),
		VertexProjectID:  os.Getenv("VERTEX_PROJECT_ID"),
		VertexLocation:   vertexLocation,
		RedisConfig: redis.Config{
			Addr:     redisAddr,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0, // Default DB
		},
		AzureRegions: map[string]string{
			"eastus":     os.Getenv("AZURE_ENDPOINT"),
			"westeurope": "https://custom-westeurope.openai.azure.com", // Example endpoint
		},
		AWSAccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSRegion:          os.Getenv("AWS_REGION"),
		BedrockRegions: map[string]string{
			"us-east-1": "https://bedrock-runtime.us-east-1.amazonaws.com",
			"us-west-2": "https://bedrock-runtime.us-west-2.amazonaws.com",
		},
	}

	return cfg
}

// GetModelConfig returns a model configuration for testing
func GetModelConfig() model.Config {
	cfg := LoadConfig()
	modelConfig := test_region_fallback.RegionFallbackMixedTest
	modelConfig.VertexProjectID = cfg.VertexProjectID
	modelConfig.VertexLocation = cfg.VertexLocation
	modelConfig.AzureAPIVersion = cfg.AzureAPIVersion

	// Set up Azure regions if not already set
	if modelConfig.AzureRegions == nil {
		modelConfig.AzureRegions = cfg.AzureRegions
	}

	// Set up Bedrock regions
	if modelConfig.BedrockRegions == nil {
		modelConfig.BedrockRegions = cfg.BedrockRegions
	}

	return modelConfig
}
