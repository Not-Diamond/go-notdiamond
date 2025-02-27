# AWS Bedrock Integration

This package provides support for integrating with AWS Bedrock in the notdiamond-golang client.

## Dynamic Model Support

The `NewRequest` function creates an HTTP request configured for AWS Bedrock:

```go
func NewRequest(region string, modelID string) (*http.Request, error)
```

### Parameters

- `region`: The AWS region to use (defaults to "us-east-1" if empty)
- `modelID`: The Bedrock model ID to use (defaults to "anthropic.claude-3-sonnet-20240229-v1:0" if empty)

### Example Usage

```go
// Create a request for Claude in us-west-2
claudeReq, err := bedrock.NewRequest("us-west-2", "anthropic.claude-3-sonnet-20240229-v1:0")

// Create a request for Amazon Titan
titanReq, err := bedrock.NewRequest("us-east-1", "amazon.titan-text-express-v1:0")

// Using default region and model
defaultReq, err := bedrock.NewRequest("", "")
```

## URL Structure

The function constructs the appropriate Bedrock API URL in this format:

```
https://bedrock-runtime.{region}.amazonaws.com/model/{modelID}/invoke
```

Each model in AWS Bedrock has its own URL endpoint, which is why the model ID must be included in the URL path.

## Authentication

Authentication headers are not set in this function. Instead, they are handled by the notdiamond client using AWS SigV4 signing.

## Supported Models

Common model IDs for AWS Bedrock include:

### Anthropic Claude Models

- `anthropic.claude-3-sonnet-20240229-v1:0`
- `anthropic.claude-3-haiku-20240307-v1:0`
- `anthropic.claude-instant-v1`
- `anthropic.claude-v2`

### Amazon Titan Models

- `amazon.titan-text-express-v1:0`
- `amazon.titan-text-lite-v1:0`

See the full example implementation in `example/bedrock_example/main.go`.
