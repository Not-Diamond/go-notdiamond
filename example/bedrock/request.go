package bedrock

import (
	"fmt"
	"net/http"
)

// NewRequest creates a new request for AWS Bedrock
func NewRequest(region string, modelID string) (*http.Request, error) {
	if region == "" {
		region = "us-east-1" // Default region
	}

	if modelID == "" {
		modelID = "anthropic.claude-3-sonnet-20240229-v1:0" // Default model if none specified
	}

	// Create the URL for Bedrock with the specified model
	url := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke", region, modelID)

	// Create a new POST request
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Bedrock request: %w", err)
	}

	// Set the necessary headers for a Bedrock request
	req.Header.Set("Content-Type", "application/json")
	// Note: We don't set authentication headers here because they will be
	// handled by the notdiamond client using AWS SigV4 signing

	return req, nil
}
