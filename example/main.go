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
	"time"

	"github.com/joho/godotenv"
)

// AWS Signature V4 constants
const (
	awsAlgorithm   = "AWS4-HMAC-SHA256"
	awsRequestType = "aws4_request"
	awsService     = "bedrock-runtime"
	awsContentType = "application/json"
	awsAccept      = "application/json"
	awsBedrockHost = "bedrock-runtime.%s.amazonaws.com"
	awsBedrockPath = "/model/%s/invoke"
)

// BedrockConfig holds AWS Bedrock configuration
type BedrockConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
}

// ClaudeRequest represents the request format for Claude models
type ClaudeRequest struct {
	AnthropicVersion string                   `json:"anthropic_version"`
	MaxTokens        int                      `json:"max_tokens"`
	Messages         []map[string]interface{} `json:"messages"`
}

// ClaudeResponse represents the response format from Claude models
type ClaudeResponse struct {
	Content    []map[string]interface{} `json:"content"`
	StopReason string                   `json:"stop_reason"`
	Usage      map[string]int           `json:"usage"`
}

func main() {
	// Load .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Load configuration from environment variables
	bedrockConfig := BedrockConfig{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		Region:          os.Getenv("AWS_REGION"),
	}

	// Check if Bedrock credentials are available
	if bedrockConfig.AccessKeyID == "" || bedrockConfig.SecretAccessKey == "" || bedrockConfig.Region == "" {
		log.Fatalf("AWS Bedrock credentials not found in environment variables")
	}

	fmt.Println("🔑 AWS Bedrock credentials loaded successfully")
	fmt.Printf("Using AWS region: %s\n", bedrockConfig.Region)

	// Test Bedrock API with Claude model
	modelID := "anthropic.claude-3-sonnet-20240229-v1:0"

	// Create request payload for Claude
	request := ClaudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        1000,
		Messages: []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, how are you? Tell me a short joke.",
			},
		},
	}

	// Make request to Bedrock API
	response, err := invokeBedrockModel(bedrockConfig, modelID, request)
	if err != nil {
		log.Fatalf("Failed to invoke Bedrock model: %v", err)
	}

	// Print response
	fmt.Println("\n🤖 AWS Bedrock Response:")
	fmt.Println(response)
}

// invokeBedrockModel makes a request to the AWS Bedrock API
func invokeBedrockModel(config BedrockConfig, modelID string, request interface{}) (string, error) {
	// Convert request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create request URL and host
	host := fmt.Sprintf(awsBedrockHost, config.Region)
	path := fmt.Sprintf(awsBedrockPath, modelID)
	url := fmt.Sprintf("https://%s%s", host, path)

	// Get current time for signing
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	// Calculate payload hash
	payloadHash := hashSHA256(jsonData)

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", awsContentType)
	req.Header.Set("Accept", awsAccept)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Host = host

	// Sign the request
	signature := createSignature(req, config, dateStamp, amzDate, payloadHash)

	// Create the authorization header
	credentialScope := fmt.Sprintf("%s/%s/%s/%s",
		dateStamp,
		config.Region,
		awsService,
		awsRequestType)

	credential := fmt.Sprintf("%s/%s", config.AccessKeyID, credentialScope)

	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"

	authHeader := fmt.Sprintf("%s Credential=%s, SignedHeaders=%s, Signature=%s",
		awsAlgorithm, credential, signedHeaders, signature)

	req.Header.Set("Authorization", authHeader)

	// Debug output
	fmt.Println("\n🔐 Request Headers:")
	for k, v := range req.Header {
		fmt.Printf("%s: %s\n", k, v)
	}

	// Create HTTP client and make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	// Pretty print the JSON response
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, body, "", "  ")
	if err != nil {
		// If we can't pretty print, return the raw response
		return string(body), nil
	}

	return prettyJSON.String(), nil
}

// createSignature creates an AWS Signature V4 for the request
func createSignature(req *http.Request, config BedrockConfig, dateStamp, amzDate, payloadHash string) string {
	// Step 1: Create a canonical request
	canonicalURI := req.URL.Path
	canonicalQueryString := req.URL.RawQuery

	// Create canonical headers
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"),
		req.Host,
		payloadHash,
		amzDate)

	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"

	// Create the canonical request
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash)

	// Debug output
	fmt.Println("\n🔐 Canonical Request:")
	fmt.Println(canonicalRequest)

	// Step 2: Create a string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/%s",
		dateStamp,
		config.Region,
		awsService,
		awsRequestType)

	// Hash the canonical request
	hashedCanonicalRequest := hashSHA256([]byte(canonicalRequest))

	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		awsAlgorithm,
		amzDate,
		credentialScope,
		hashedCanonicalRequest)

	// Debug output
	fmt.Println("\n🔍 String to Sign:")
	fmt.Println(stringToSign)

	// Step 3: Calculate the signature
	// Create the signing key
	kDate := hmacSHA256([]byte("AWS4"+config.SecretAccessKey), dateStamp)
	kRegion := hmacSHA256(kDate, config.Region)
	kService := hmacSHA256(kRegion, awsService)
	kSigning := hmacSHA256(kService, awsRequestType)

	// Sign the string to sign
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	// Debug output
	fmt.Println("\n🔍 Signature:")
	fmt.Println(signature)

	return signature
}

// Helper functions for AWS Signature V4
func hashSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
