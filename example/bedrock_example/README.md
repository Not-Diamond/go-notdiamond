# Amazon Bedrock Integration Example

This example demonstrates how to use the notdiamond-golang client with Amazon Bedrock services.

## Prerequisites

- AWS credentials configured in your environment:
  - `AWS_ACCESS_KEY_ID`
  - `AWS_SECRET_ACCESS_KEY`
  - `AWS_REGION` (default is "us-east-1")
- Amazon Bedrock access and model permissions configured in your AWS account

## Features Demonstrated

This example shows:

1. **Basic Claude Model Integration**: Making a request to Anthropic Claude models via Bedrock
2. **Basic Titan Model Integration**: Making a request to Amazon Titan models via Bedrock
3. **Region Fallback**: Demonstrating the region fallback functionality which tries alternative regions if the model is not available in the primary region

## Usage

Run the example:

```bash
cd example/bedrock_example
go run main.go
```

## Example Output

The example will test three different scenarios:

1. Claude model:

```
--- Testing with Claude model ---
🤖 Model: anthropic.claude
⏱️  Time: 1.23s
💬 Response: Amazon Bedrock is a fully managed service by AWS that provides access to foundation models (FMs) from leading AI companies through a unified API. It allows developers to build generative AI applications using high-performance models from companies like Anthropic, AI21 Labs, Cohere, Meta, Stability AI, and Amazon without having to manage the underlying infrastructure or negotiate with multiple providers individually.
```

2. Titan model:

```
--- Testing with Titan model ---
🤖 Model: amazon.titan
⏱️  Time: 0.85s
💬 Response: Amazon Bedrock is a fully managed service that makes high-performance foundation models from leading AI companies available through a single API, allowing you to build generative AI applications without having to manage the underlying infrastructure.
```

3. Region fallback:

```
--- Testing region fallback ---
🤖 Model: anthropic.claude
⏱️  Time: 1.05s
💬 Response: I am Claude, specifically the Claude 3 Haiku model, running via Amazon Bedrock.
```

## Customization

You can modify the example by:

1. Changing the models used in each test function
2. Adjusting parameters like temperature, max tokens, etc.
3. Changing the prompt/input text
4. Adding more regions to the fallback list

## AWS Configuration

Ensure your AWS credentials are properly configured and that your account has the necessary permissions to access Bedrock models. You can manage model access in the AWS Management Console under Bedrock > Model access.
