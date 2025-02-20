package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Not-Diamond/go-notdiamond"
	"golang.org/x/oauth2/google"

	"example/azure"
	"example/config"
	"example/openai"
	"example/vertex"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Create API requests
	openaiRequest, err := openai.NewRequest("https://api.openai.com/v1/chat/completions", cfg.OpenAIAPIKey)
	if err != nil {
		log.Fatalf("Failed to create openai request: %v", err)
	}
	azureRequest, err := azure.NewRequest(cfg.AzureEndpoint, cfg.AzureAPIKey)
	if err != nil {
		log.Fatalf("Failed to create azure request: %v", err)
	}
	vertexRequest, err := vertex.NewRequest(cfg.VertexProjectID, cfg.VertexLocation)
	if err != nil {
		log.Fatalf("Failed to create vertex request: %v", err)
	}

	// Get model configuration
	modelConfig := config.GetModelConfig()
	modelConfig.Clients = []http.Request{
		*openaiRequest,
		*azureRequest,
		*vertexRequest,
	}
	modelConfig.RedisConfig = &cfg.RedisConfig

	// Create transport with configuration
	transport, err := notdiamond.NewTransport(modelConfig)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create HTTP client with our transport
	client := &http.Client{
		Transport: transport,
	}

	// Prepare request payload
	// Create Vertex AI payload format
	payload := map[string]interface{}{
		"model": "gemini-pro",
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"text": "Hello, what model are you??",
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 1024,
			"topP":            0.95,
			"topK":            40,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	// Create request using vertex.NewRequest
	req, err := vertex.NewRequest(cfg.VertexProjectID, cfg.VertexLocation)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	// Modify the URL to include the typo for testing fallback
	req.Body = io.NopCloser(bytes.NewBuffer(jsonData))

	// Set up Google credentials for Vertex AI
	credentials, err := google.FindDefaultCredentials(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		log.Fatalf("Failed to get credentials: %v", err)
	}
	token, err := credentials.TokenSource.Token()
	if err != nil {
		log.Fatalf("Failed to get token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

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

	// Try to parse as OpenAI response first
	var openaiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &openaiResponse); err == nil && len(openaiResponse.Choices) > 0 {
		timeTaken := time.Since(start)
		fmt.Printf("🤖 Model: %s\n", "gpt-4o")
		fmt.Printf("⏱️  Time: %.2fs\n", timeTaken.Seconds())
		fmt.Printf("💬 Response: %s\n", openaiResponse.Choices[0].Message.Content)
		return
	}

	// If not OpenAI, try to parse as Vertex AI response
	var vertexResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &vertexResponse); err != nil {
		fmt.Printf("Raw response: %s\n", string(body))
		log.Fatalf("Failed to parse response: %v", err)
	}

	if len(vertexResponse.Candidates) > 0 && len(vertexResponse.Candidates[0].Content.Parts) > 0 {
		timeTaken := time.Since(start)
		fmt.Printf("🤖 Model: %s\n", "gemini-pro")
		fmt.Printf("⏱️  Time: %.2fs\n", timeTaken.Seconds())
		fmt.Printf("💬 Response: %s\n", vertexResponse.Candidates[0].Content.Parts[0].Text)
		return
	}

	fmt.Printf("Raw response: %s\n", string(body))
	log.Fatal("Response did not contain expected data")
}
