package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"notdiamond"
)

func NewOpenAIRequest(url string, apiKey string) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	return req, nil
}

func main() {
	openaiApiKey := GetEnvVariable("OPENAI_API_KEY")
	azureApiKey := GetEnvVariable("AZURE_API_KEY")
	azureEndpoint := GetEnvVariable("AZURE_ENDPOINT")

	openaiRequest, err := NewOpenAIRequest("https://api.openai.com/v1/chat/completions", openaiApiKey)
	if err != nil {
		log.Fatalf("Failed to create openai request: %v", err)
	}
	azureRequest, err := NewOpenAIRequest(azureEndpoint, azureApiKey)
	if err != nil {
		log.Fatalf("Failed to create azure request: %v", err)
	}

	config := notdiamond.Config{
		Clients: []http.Request{
			*openaiRequest,
			*azureRequest,
		},
		// Models: map[string]float64{
		// 	"azure/gpt-4o-mini":  0.1,
		// 	"openai/gpt-4o-mini": 0.1,
		// 	"openai/gpt-4o":      0.7,
		// 	"azure/gpt-4o":       0.1,
		// },
		Models: notdiamond.OrderedModels{
			"openai/gpt-4o-mini",
			"azure/gpt-4o-mini",
			"azure/gpt-4o",
		},
		MaxRetries: map[string]int{
			"openai/gpt-4o-mini": 2,
			"azure/gpt-4o-mini":  2,
			"azure/gpt-4o":       2,
		},
		Timeout: map[string]float64{
			"azure/gpt-4o-mini":  10,
			"openai/gpt-4o-mini": 10,
			"azure/gpt-4o":       10,
		},
		ModelMessages: map[string][]notdiamond.Message{
			"azure/gpt-4o-mini": {
				{"role": "user", "content": "Please respond only with answer in spanish."},
			},
			"openai/gpt-4o-mini": {
				{"role": "user", "content": "Please respond only with answer in romanian."},
			},
			"azure/gpt-4o": {
				{"role": "user", "content": "Please respond only with answer in french."},
			},
		},
		StatusCodeRetry: map[string]map[string]int{
			"openai/gpt-4o-mini": {
				"401": 1,
			},
		},
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
