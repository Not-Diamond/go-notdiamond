package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/transport"

	"example/bedrock"
	"example/config"
	"example/response"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Test with Claude model
	fmt.Println("\n--- Testing with Claude model ---")
	testClaude(cfg)

	// Test with Titan model
	fmt.Println("\n--- Testing with Titan model ---")
	testTitan(cfg)

	// Test region fallback
	fmt.Println("\n--- Testing region fallback ---")
	testRegionFallback(cfg)
}

func testClaude(cfg config.Config) {
	// Create Bedrock request for Claude model
	modelID := "anthropic.claude-3-sonnet-20240229-v1:0"
	bedrockRequest, err := bedrock.NewRequest(cfg.AWSRegion, modelID)
	if err != nil {
		log.Fatalf("Failed to create bedrock request: %v", err)
	}

	// Get model configuration
	modelConfig := config.GetModelConfig()
	modelConfig.Clients = []http.Request{*bedrockRequest}
	modelConfig.RedisConfig = &cfg.RedisConfig

	// Set up Bedrock regions
	modelConfig.BedrockRegions = map[string]string{
		"us-east-1": "https://bedrock-runtime.us-east-1.amazonaws.com",
		"us-west-2": "https://bedrock-runtime.us-west-2.amazonaws.com",
	}

	// Create transport with configuration
	transport, err := transport.NewTransport(modelConfig)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create HTTP client with our transport
	client := &http.Client{
		Transport: transport,
	}

	// Using Amazon Bedrock with Claude model
	bedrockPayload := map[string]interface{}{
		"model":       modelID,
		"prompt":      "\n\nHuman: Explain briefly what Amazon Bedrock is.\n\nAssistant: ",
		"temperature": 0.7,
		"max_tokens":  1024,
		"top_p":       0.95,
	}

	makeAndPrintRequest(client, bedrockRequest, bedrockPayload)
}

func testTitan(cfg config.Config) {
	// Create Bedrock request for Titan model
	modelID := "amazon.titan-text-express-v1:0"
	bedrockRequest, err := bedrock.NewRequest(cfg.AWSRegion, modelID)
	if err != nil {
		log.Fatalf("Failed to create bedrock request: %v", err)
	}

	// Get model configuration
	modelConfig := config.GetModelConfig()
	modelConfig.Clients = []http.Request{*bedrockRequest}
	modelConfig.RedisConfig = &cfg.RedisConfig

	// Set up Bedrock regions
	modelConfig.BedrockRegions = map[string]string{
		"us-east-1": "https://bedrock-runtime.us-east-1.amazonaws.com",
		"us-west-2": "https://bedrock-runtime.us-west-2.amazonaws.com",
	}

	// Create transport with configuration
	transport, err := transport.NewTransport(modelConfig)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create HTTP client with our transport
	client := &http.Client{
		Transport: transport,
	}

	// Using Amazon Bedrock with Titan model
	bedrockPayload := map[string]interface{}{
		"model":     modelID,
		"inputText": "Explain briefly what Amazon Bedrock is.",
		"textGenerationConfig": map[string]interface{}{
			"temperature":   0.7,
			"maxTokenCount": 1024,
			"topP":          0.95,
		},
	}

	makeAndPrintRequest(client, bedrockRequest, bedrockPayload)
}

func testRegionFallback(cfg config.Config) {
	// Create Bedrock request with a model that might need region fallback
	modelID := "anthropic.claude-3-haiku-20240307-v1:0"
	bedrockRequest, err := bedrock.NewRequest(cfg.AWSRegion, modelID)
	if err != nil {
		log.Fatalf("Failed to create bedrock request: %v", err)
	}

	// Get model configuration
	modelConfig := config.GetModelConfig()
	modelConfig.Clients = []http.Request{*bedrockRequest}
	modelConfig.RedisConfig = &cfg.RedisConfig

	// Set up Bedrock regions
	modelConfig.BedrockRegions = map[string]string{
		"us-east-1": "https://bedrock-runtime.us-east-1.amazonaws.com",
		"us-west-2": "https://bedrock-runtime.us-west-2.amazonaws.com",
	}

	// Create transport with configuration
	transport, err := transport.NewTransport(modelConfig)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create HTTP client with our transport
	client := &http.Client{
		Transport: transport,
	}

	// Using Amazon Bedrock with a model that doesn't exist in the primary region
	// This should trigger region fallback
	bedrockPayload := map[string]interface{}{
		"model":       modelID,
		"prompt":      "\n\nHuman: Hello, can you confirm which model you are?\n\nAssistant: ",
		"temperature": 0.7,
		"max_tokens":  512,
		"top_p":       0.95,
	}

	makeAndPrintRequest(client, bedrockRequest, bedrockPayload)
}

func makeAndPrintRequest(client *http.Client, bedrockRequest *http.Request, payload map[string]interface{}) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	// Create a new request with the same URL but with our payload
	req, err := http.NewRequest("POST", bedrockRequest.URL.String(), io.NopCloser(bytes.NewBuffer(jsonData)))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	// Copy headers from the bedrock request
	for key, values := range bedrockRequest.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Make request with transport client
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	// Print raw response for debugging
	fmt.Println("Raw response:")
	fmt.Println(string(body))

	result, err := response.Parse(body, start)
	if err != nil {
		log.Printf("Error parsing response: %v", err)
		return
	}

	fmt.Printf("🤖 Model: %s\n", result.Model)
	fmt.Printf("⏱️  Time: %.2fs\n", result.TimeTaken.Seconds())
	fmt.Printf("💬 Response: %s\n", result.Response)
}
