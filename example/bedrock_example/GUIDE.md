# Comprehensive Guide to Amazon Bedrock Integration

This guide provides detailed information on how to use the Amazon Bedrock integration with the notdiamond-golang client.

## Setup and Configuration

### Environment Variables

First, ensure you have the following environment variables set:

```
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
AWS_REGION=us-east-1
```

You can set these in your `.env` file or directly in your system environment.

### Configuring Bedrock Regions

The notdiamond-golang client supports multiple Bedrock regions for failover. Configure them in your application:

```go
modelConfig.BedrockRegions = map[string]string{
    "us-east-1": "https://bedrock-runtime.us-east-1.amazonaws.com",
    "us-west-2": "https://bedrock-runtime.us-west-2.amazonaws.com",
    // Add more regions as needed
}
```

## Supported Models

### Anthropic Claude Models

Anthropic Claude models are accessible through Bedrock with the following model IDs:

- `anthropic.claude-3-sonnet-20240229-v1:0`
- `anthropic.claude-3-haiku-20240307-v1:0`
- `anthropic.claude-instant-v1`
- `anthropic.claude-v2`

Example Claude request payload:

```go
bedrockPayload := map[string]interface{}{
    "model":       "anthropic.claude-3-sonnet-20240229-v1:0",
    "prompt":      "\n\nHuman: Your prompt here\n\nAssistant: ",
    "temperature": 0.7,
    "max_tokens":  1024,
    "top_p":       0.95,
}
```

### Amazon Titan Models

Amazon Titan models are accessible with the following model IDs:

- `amazon.titan-text-express-v1:0`
- `amazon.titan-text-lite-v1:0`

Example Titan request payload:

```go
bedrockPayload := map[string]interface{}{
    "model":     "amazon.titan-text-express-v1:0",
    "inputText": "Your prompt here",
    "textGenerationConfig": map[string]interface{}{
        "temperature":   0.7,
        "maxTokenCount": 1024,
        "topP":          0.95,
    },
}
```

## Region Fallback

The notdiamond-golang client automatically handles region fallback for Bedrock requests. If a model is not available in the primary region, the client will try alternative regions configured in `modelConfig.BedrockRegions`.

Example configuration for region fallback:

```go
modelConfig.BedrockRegions = map[string]string{
    "us-east-1": "https://bedrock-runtime.us-east-1.amazonaws.com", // Primary region
    "us-west-2": "https://bedrock-runtime.us-west-2.amazonaws.com", // Fallback region
}
```

## Response Parsing

The notdiamond client automatically transforms Bedrock responses into a unified format. The response parser handles:

1. Claude model responses (extracting the completion)
2. Titan model responses (extracting the outputText)

Example response handling:

```go
result, err := response.Parse(responseBody, startTime)
if err != nil {
    log.Printf("Error parsing response: %v", err)
    return
}

fmt.Printf("Model: %s\n", result.Model)
fmt.Printf("Response: %s\n", result.Response)
fmt.Printf("Time: %.2fs\n", result.TimeTaken.Seconds())
```

## Error Handling

When working with Bedrock, you might encounter various errors:

1. **Authentication Errors**: Ensure your AWS credentials are correct
2. **Model Access Errors**: Verify you have enabled the models in your AWS account
3. **Region Availability Errors**: Some models may not be available in all regions
4. **Rate Limit Errors**: Bedrock has rate limits that may require throttling

Example error handling:

```go
resp, err := client.Do(req)
if err != nil {
    if strings.Contains(err.Error(), "AccessDeniedException") {
        log.Printf("Authentication error: %v", err)
    } else if strings.Contains(err.Error(), "ModelNotReadyException") {
        log.Printf("Model not ready: %v", err)
    } else {
        log.Printf("Request failed: %v", err)
    }
    return
}
```

## Advanced Usage

### Custom Headers

For advanced use cases, you may need to add custom headers to your Bedrock requests:

```go
req.Header.Add("X-Amzn-Bedrock-Trace", "true") // Enable tracing
```

### Streaming Responses

If you want to implement streaming responses with Bedrock:

```go
req.Header.Add("Accept", "application/json")
```

Note that streaming implementation requires custom handling on the response side.

## Troubleshooting

### Common Issues

1. **"AccessDeniedException"**: Verify your AWS credentials and that your IAM user/role has permission to call Bedrock
2. **"ModelNotReadyException"**: The model is not yet ready in the region - check the AWS console
3. **"ThrottlingException"**: You've exceeded the rate limit - implement backoff/retry logic
4. **"ValidationException"**: Check your request payload format
5. **"ResourceNotFoundException"**: The model ID might be incorrect or not available

### Debugging Tips

1. Enable verbose logging:

   ```go
   log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
   ```

2. Print raw responses for debugging:

   ```go
   fmt.Println("Raw response:", string(responseBody))
   ```

3. Check AWS CloudTrail logs for detailed error information

## Additional Resources

- [AWS Bedrock Documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/what-is-bedrock.html)
- [Bedrock API Reference](https://docs.aws.amazon.com/bedrock/latest/APIReference/welcome.html)
- [Anthropic Claude Documentation](https://docs.anthropic.com/claude/reference/getting-started-with-the-api)
- [AWS SDK for Go Documentation](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/bedrockruntime)
