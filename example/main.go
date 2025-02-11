package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	test_weighted "main/manual-test-cases/weighted_models"
	"main/openai"
	"net/http"
	"time"

	"notdiamond"
)

func main() {
	var (
		openaiApiKey  = GetEnvVariable("OPENAI_API_KEY")
		azureApiKey   = GetEnvVariable("AZURE_API_KEY")
		azureEndpoint = GetEnvVariable("AZURE_ENDPOINT")
	)

	openaiRequest, err := openai.NewRequest("https://api.openai.com/v1/chat/completions", openaiApiKey)
	if err != nil {
		log.Fatalf("Failed to create openai request: %v", err)
	}
	azureRequest, err := openai.NewRequest(azureEndpoint, azureApiKey)
	if err != nil {
		log.Fatalf("Failed to create azure request: %v", err)
	}

	config := test_weighted.WeightedModels

	config.Clients = []http.Request{
		*openaiRequest,
		*azureRequest,
	}

	notdiamondClient, err := notdiamond.Init(config)
	if err != nil {
		log.Fatalf("Failed to initialize notdiamond: %v", err)
	}

	messages := []map[string]string{
		{"role": "user", "content": "Hello, how are you?"},
	}

	payload := map[string]interface{}{
		"model":    "gpt-4o-mini",
		"messages": messages,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openaiApiKey)

	startTime := time.Now()
	resp, err := notdiamondClient.Do(req)

	if err != nil {
		log.Fatalf("Failed to do request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	var response struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Print(string(body))
		log.Fatalf("Failed to parse response: %v", err)
	}

	timeTaken := time.Since(startTime)

	fmt.Printf("🤖 Model: %s\n", response.Model)
	fmt.Printf("⏱️  Time: %.2fs\n", timeTaken.Seconds())
	fmt.Printf("💬 Response: %s\n", response.Choices[0].Message.Content)
}
