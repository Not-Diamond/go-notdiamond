# 💎 Not Diamond Go SDK [![Go Report Card](https://goreportcard.com/badge/github.com/Not-Diamond/go-notdiamond)](https://goreportcard.com/report/github.com/Not-Diamond/go-notdiamond) [![codecov](https://codecov.io/gh/Not-Diamond/go-notdiamond/graph/badge.svg?token=V99TFTE05X)](https://codecov.io/gh/Not-Diamond/go-notdiamond)

One line statement to improve reliability and uptime of LLM requests. [Documentation](https://docs.notdiamond.ai/docs/fallbacks-and-timeouts/)

> **Note**
> Currently supported providers:
>
> - **OpenAI** models
> - **Azure** models
> - **Vertex AI**

## ✨ Features:

- Fallback to other models if one fails
- Load balance requests between multiple models
- Max retries and timeout for each model
- Exponential backoff strategy
- Retry based on HTTP status codes
- Average rolling latency fallback

## 📦 Installation

```
go get github.com/Not-Diamond/go-notdiamond
```

## 🚀 Basic Usage

Error handling intentionally ommited in the example for simplicity.

Redis needs to be running and accessible from the machine where this code is executed. (runs on default port 6379, but can be changed)

```go
// Get keys
openaiApiKey := ''
azureApiKey := ''
azureEndpoint := ''
vertexProjectID := '' // Your Google Cloud project ID
vertexLocation := 'us-central1' // Your Google Cloud region

// Create requests
openaiRequest := openai.NewRequest("https://api.openai.com/v1/chat/completions", openaiApiKey)
azureRequest := azure.NewRequest(azureEndpoint, azureApiKey)
vertexRequest := vertex.NewRequest(vertexProjectID, vertexLocation)

// Create config
config := model.Config{
	Clients: []http.Request{ openaiRequest, azureRequest, vertexRequest },
	Models: model.OrderedModels{ "vertex/gemini-pro", "azure/gpt-4o-mini", "openai/gpt-4o-mini" },
	MaxRetries: map[string]int{
		"vertex/gemini-pro": 2,
		"azure/gpt-4o-mini": 2,
		"openai/gpt-4o-mini": 2,
	},
	VertexProjectID: vertexProjectID,
	VertexLocation: vertexLocation,
}

// Create transport
transport, err := notdiamond.NewTransport(config)

// Create a standard http.Client with our transport
client := &http.Client{
	Transport: transport,
}

// Prepare Payload
messages := []map[string]string{ {"role": "user", "content": "Hello, how are you?"} }
payload := map[string]interface{}{ "model": "gpt-4o-mini", "messages": messages }
jsonData := json.Marshal(payload)

// Create request
req := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Authorization", "Bearer "+openaiApiKey)

// Do request via standard http.Client with our transport
resp := client.Do(req)
defer resp.Body.Close()
body := io.ReadAll(resp.Body)

// Final response
fmt.Println(string(body))
```

## Load Balancing

You can configure load balancing between models using weights:

```go
config := notdiamond.Config{
	// ... other config ...
	Models: notdiamond.WeightedModels{
		"vertex/gemini-pro": 0.4, // 40% of requests
		"azure/gpt-4": 0.3, // 30% of requests
		"openai/gpt-4": 0.3, // 30% of requests
	},
}
```

## Max Retries

Configure custom max retries for each model:

```go
// Default max retries is 1
config := notdiamond.Config{
	// ... other config ...
	MaxRetries: map[string]int{
		"azure/gpt-4":  3, // 3 retries
		"openai/gpt-4": 2, // 2 retries
	},
}
```

## Timeout

Configure custom timeout (in seconds) for each model:

```go
// Default timeout is 100 seconds
config := notdiamond.Config{
	// ... other config ...
	Timeout: map[string]float64{
		"azure/gpt-4":  10.0, // 10 seconds
		"openai/gpt-4": 5.0,  // 5 seconds
	},
}
```

## Exponential Backoff

Configure custom backoff times (in seconds) for each model:

```go
// Default backoff is 1 second
config := notdiamond.Config{
	// ... other config ...
	Backoff: map[string]float64{
		"azure/gpt-4":  0.5,  // Start with 0.5s, then 1s, 2s, etc.
		"openai/gpt-4": 1.0,  // Start with 1s, then 2s, 4s, etc.
	},
}
```

## Model-Specific Messages

You can configure system messages that will be prepended to user messages for specific models:

```go
config := notdiamond.Config{
	// ... other config ...
	ModelMessages: map[string][]map[string]string{
		"azure/gpt-4": {
			{"role": "system", "content": "You are a helpful assistant."},
		},
		"openai/gpt-4": {
			{"role": "system", "content": "Respond concisely."},
		},
	},
}
```

## Status Code Retries

You can configure specific retry behavior for different HTTP status codes, either globally or per model.

Per model retry behavior:

```go
config := notdiamond.Config{
	// ... other config ...
	StatusCodeRetry: map[string]map[string]int{
		"openai/gpt-4": {
			"429": 3, // Retry rate limit errors 3 times
			"500": 2, // Retry internal server errors 2 times
		},
	},
}
```

Global retry behavior:

```go
config := notdiamond.Config{
	// ... other config ...
	StatusCodeRetry: map[string]int{
		"429": 3, // Retry rate limit errors 3 times
	},
}
```

## Average Rolling Latency Fallback

Configure custom average rolling latency threshold and recovery time for each model:

```go
config := notdiamond.Config{
	// ... other config ...
	ModelLatency: map[string]notdiamond.RollingAverageLatency{
		"azure/gpt-4": {
			AvgLatencyThreshold: 3.2, // Average latency threshold, if average latency is greater than this, fallback to other models
			NoOfCalls:           10, // Number of calls to make to get the average latency
			RecoveryTime:        3 * time.Second, // Time to wait before retrying
		},
	},
}
```

## Model Limits

Configure custom limits for each model:

```go
config := notdiamond.Config{
	// ... other config ...
	ModelLimits: model.ModelLimits{
		MaxNoOfCalls:    10000,
		MaxRecoveryTime: time.Hour * 24,
	},
}
```

### Customise Redis

```go
config := notdiamond.Config{
	// ... other config ...
	Redis: notdiamond.RedisConfig{
		Addr: "localhost:6379",
		Password: "password",
		DB: 0,
	},
}
```

## Error Rate Fallback

Configure custom error rate thresholds and recovery time for each model, with different thresholds for different status codes:

```go
config := model.Config{
	// ... other config ...
	ModelErrorTracking: model.ModelErrorTracking{
		"openai/gpt-4": &model.RollingErrorTracking{
			StatusConfigs: map[int]*model.StatusCodeConfig{
				401: {
					ErrorThresholdPercentage: 80,  // Fallback if 80% of calls return 401
					NoOfCalls:                5,   // Number of calls to consider
					RecoveryTime:             1 * time.Minute, // Time to wait before retrying
				},
				500: {
					ErrorThresholdPercentage: 70,  // Fallback if 70% of calls return 500
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
				502: {
					ErrorThresholdPercentage: 60,  // Fallback if 60% of calls return 502
					NoOfCalls:                5,
					RecoveryTime:             1 * time.Minute,
				},
				429: {
					ErrorThresholdPercentage: 40,  // Fallback if 40% of calls return 429 (rate limit)
					NoOfCalls:                5,
					RecoveryTime:             30 * time.Second,
				},
			},
		},
	},
}
```

The error tracking system will:

1. Track all HTTP status codes for each model
2. Calculate the error percentage for each status code over the last N calls
3. If any status code's error percentage exceeds its threshold, mark the model as unhealthy and fallback to other models
4. After the recovery time, the model will be tried again

This allows for fine-grained control over error handling:

- Set different thresholds for different types of errors (e.g., more aggressive fallback for rate limits)
- Configure different number of calls and recovery times per status code
- Track any HTTP status code you want to monitor
- Configure different thresholds for different models based on their reliability
