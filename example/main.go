package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/transport"
	"github.com/joho/godotenv"

	"example/azure"
	"example/bedrock"
	"example/config"
	"example/openai"
	"example/response"
	"example/vertex"
)

// signRequestWithSigV4 signs the request with AWS Signature Version 4
func signRequestWithSigV4(r *http.Request, region, service, accessKey, secretKey string) {
	// Time
	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	datestamp := t.Format("20060102")

	// Create canonical request
	method := r.Method
	canonicalURI := r.URL.Path

	// Canonical query string
	canonicalQueryString := r.URL.RawQuery

	// Create payload hash
	var payloadHash string
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Restore body
		payloadHash = hashSHA256(bodyBytes)
	} else {
		payloadHash = hashSHA256([]byte{})
	}

	// Add host header if not present
	if r.Header.Get("Host") == "" {
		r.Header.Set("Host", r.Host)
	}

	// Canonical headers (make sure they are sorted)
	headerKeys := make([]string, 0)
	headerMap := make(map[string]string)

	// AWS requires headers to be sorted by key
	for key, values := range r.Header {
		lowerKey := strings.ToLower(key)
		if lowerKey == "host" || lowerKey == "content-type" || strings.HasPrefix(lowerKey, "x-amz-") {
			headerKeys = append(headerKeys, lowerKey)
			headerMap[lowerKey] = values[0]
		}
	}

	// Add required X-Amz-Date if not present
	if _, exists := headerMap["x-amz-date"]; !exists {
		headerKeys = append(headerKeys, "x-amz-date")
		headerMap["x-amz-date"] = amzDate
		r.Header.Set("X-Amz-Date", amzDate)
	}

	// Add X-Amz-Security-Token if present in environment
	securityToken := os.Getenv("AWS_SESSION_TOKEN")
	if securityToken != "" {
		headerKeys = append(headerKeys, "x-amz-security-token")
		headerMap["x-amz-security-token"] = securityToken
		r.Header.Set("X-Amz-Security-Token", securityToken)
	}

	sort.Strings(headerKeys)

	canonicalHeaders := ""
	signedHeaders := ""

	for i, key := range headerKeys {
		canonicalHeaders += key + ":" + headerMap[key] + "\n"
		if i > 0 {
			signedHeaders += ";"
		}
		signedHeaders += key
	}

	// Create canonical request
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash)

	// Create string to sign
	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", datestamp, region, service)
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		amzDate,
		credentialScope,
		hashSHA256([]byte(canonicalRequest)))

	// Calculate signature
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(datestamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	// Add the signing info to the request
	authorizationHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		accessKey,
		credentialScope,
		signedHeaders,
		signature)

	r.Header.Set("Authorization", authorizationHeader)

	// Log signing details for debugging
	fmt.Printf("Signing details:\n")
	fmt.Printf("- Region: %s\n", region)
	fmt.Printf("- Service: %s\n", service)
	fmt.Printf("- SignedHeaders: %s\n", signedHeaders)
	fmt.Printf("- CanonicalRequest:\n%s\n", canonicalRequest)
}

// Helper functions for AWS signature
func hashSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// getBedrockInvokeURL returns the properly formatted Bedrock invoke URL
func getBedrockInvokeURL(region, modelID string) string {
	// Ensure region is set
	if region == "" {
		region = "us-east-1"
	}

	// Clean up modelID to ensure proper format
	modelID = strings.TrimSpace(modelID)

	// For Bedrock invoke endpoint
	return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke",
		region, modelID)
}

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

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
	bedrockRequest, err := bedrock.NewRequest(cfg.AWSRegion, "anthropic.claude-3-sonnet-20240229-v1:0")
	if err != nil {
		log.Fatalf("Failed to create bedrock request: %v", err)
	}

	// Get model configuration
	modelConfig := config.GetModelConfig()
	modelConfig.Clients = []http.Request{
		*openaiRequest,
		*azureRequest,
		*vertexRequest,
		*bedrockRequest,
	}
	modelConfig.RedisConfig = &cfg.RedisConfig

	// Set up Azure regions
	modelConfig.AzureRegions = map[string]string{
		"eastus":     cfg.AzureEndpoint,
		"westeurope": "https://custom-westeurope.openai.azure.com",
	}

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

	// Choose provider
	fmt.Println("Choose provider:")
	fmt.Println("1. OpenAI")
	fmt.Println("2. Vertex AI")
	fmt.Println("3. Bedrock")
	fmt.Println("4. Azure")
	fmt.Println("Enter choice (1-4):")

	var providerChoice int
	fmt.Scanln(&providerChoice)

	// Initialize variables to track request/response
	var jsonData []byte
	var req *http.Request
	var resp *http.Response
	var start time.Time
	var duration time.Duration

	switch providerChoice {
	case 1: // OpenAI
		// Using OpenAI ChatGPT
		openaiPayload := map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello, how are you??",
				},
			},
			"temperature": 0.7,
			"max_tokens":  150,
			"top_p":       0.95,
		}
		jsonData, err = json.Marshal(openaiPayload)
		if err != nil {
			log.Fatalf("Failed to marshal payload: %v", err)
		}
		req, err = http.NewRequest("POST", openaiRequest.URL.String(), io.NopCloser(bytes.NewBuffer(jsonData)))
		if err != nil {
			log.Fatalf("Failed to create request: %v", err)
		}
		// Copy headers from the OpenAI request
		for key, values := range openaiRequest.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	case 2: // Vertex
		vertexPayload := map[string]interface{}{
			"model": "gemini-pro",
			"contents": []map[string]interface{}{
				{
					"role": "user",
					"parts": []map[string]interface{}{
						{
							"text": "Hello, how are you??",
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
		jsonData, err = json.Marshal(vertexPayload)
		if err != nil {
			log.Fatalf("Failed to marshal payload: %v", err)
		}
		req, err = http.NewRequest("POST", vertexRequest.URL.String(), io.NopCloser(bytes.NewBuffer(jsonData)))
		if err != nil {
			log.Fatalf("Failed to create request: %v", err)
		}
		// Copy headers from the vertex request
		for key, values := range vertexRequest.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	case 3: // Bedrock
		// Using Amazon Bedrock with Claude model
		modelID := "anthropic.claude-3-sonnet-20240229-v1:0"

		// Format the request based on the latest AWS Bedrock Claude API format
		bedrockPayload := map[string]interface{}{
			"anthropic_version": "bedrock-2023-05-31",
			"max_tokens":        1024,
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello, how are you??",
				},
			},
			"temperature": 0.7,
			"top_p":       0.95,
		}

		jsonData, err = json.Marshal(bedrockPayload)
		if err != nil {
			log.Fatalf("Failed to marshal payload: %v", err)
		}

		// Create a direct request to the proper Bedrock endpoint
		bedrockEndpoint := getBedrockInvokeURL(cfg.AWSRegion, modelID)
		req, err = http.NewRequest("POST", bedrockEndpoint, io.NopCloser(bytes.NewBuffer(jsonData)))
		if err != nil {
			log.Fatalf("Failed to create direct Bedrock request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")

		// Add required headers for Bedrock
		req.Header.Set("X-Amz-Target", "com.amazonaws.bedrock.runtime.BedrockRuntime.InvokeModel")

		// Sign the request with AWS SigV4
		signRequestWithSigV4(req, cfg.AWSRegion, "bedrock", cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey)

		// Make this a direct request, bypassing the NotDiamond transport
		directClient := &http.Client{
			Timeout: time.Second * 30,
		}

		// Log the request for debugging
		fmt.Printf("Making direct Bedrock request to: %s\n", req.URL)
		fmt.Printf("Headers: %v\n", req.Header)

		start = time.Now()
		resp, err = directClient.Do(req)
		if err != nil {
			log.Fatalf("Direct Bedrock request failed: %v", err)
		}
		duration = time.Since(start)

		// Print response status and duration
		fmt.Printf("Direct Bedrock response status: %s (took %v)\n", resp.Status, duration)

		// Skip the NotDiamond client call for this case
		goto HandleResponse
	case 4: // Azure
		azurePayload := map[string]interface{}{
			"model": "gpt-35-turbo", // Use actual model name for Azure
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello, how are you??",
				},
			},
			"temperature": 0.7,
			"max_tokens":  1024,
		}
		jsonData, err = json.Marshal(azurePayload)
		if err != nil {
			log.Fatalf("Failed to marshal payload: %v", err)
		}
		req, err = http.NewRequest("POST", azureRequest.URL.String(), io.NopCloser(bytes.NewBuffer(jsonData)))
		if err != nil {
			log.Fatalf("Failed to create request: %v", err)
		}
		// Copy headers from the azure request
		for key, values := range azureRequest.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// Make request with transport client if not already made
	if resp == nil {
		start = time.Now()
		resp, err = client.Do(req)
		if err != nil {
			log.Fatalf("Request failed: %v", err)
		}
		duration = time.Since(start)
	}

HandleResponse:
	defer resp.Body.Close()

	// Print response details
	fmt.Printf("Response Status: %s\n", resp.Status)
	fmt.Printf("Time taken: %v\n", duration)

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	result, err := response.Parse(body, start)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("🤖 Model: %s\n", result.Model)
	fmt.Printf("⏱️  Time: %.2fs\n", result.TimeTaken.Seconds())
	fmt.Printf("💬 Response: %s\n", result.Response)
}
