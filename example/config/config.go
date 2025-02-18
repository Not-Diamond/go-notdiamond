package config

import (
	test_ordered "example/manual-test-cases/ordered_models"
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
	RedisConfig      redis.Config
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

	cfg := Config{
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		AzureAPIKey:      os.Getenv("AZURE_API_KEY"),
		AzureEndpoint:    os.Getenv("AZURE_ENDPOINT"),
		AzureAPIVersion:  os.Getenv("AZURE_API_VERSION"),
		OpenAIAPIVersion: os.Getenv("OPENAI_API_VERSION"),
		RedisConfig: redis.Config{
			Addr:     redisAddr,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0, // Default DB
		},
	}

	return cfg
}

// GetModelConfig returns a model configuration for testing
func GetModelConfig() model.Config {
	return test_ordered.OrderedModelsWithLatency
}
