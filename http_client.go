package notdiamond

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/http/request"
	"github.com/Not-Diamond/go-notdiamond/pkg/metric"
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/Not-Diamond/go-notdiamond/pkg/validation"
	"golang.org/x/oauth2/google"
)

// NotDiamondHttpClient is a type that can be used to represent a NotDiamond HTTP client.
type NotDiamondHttpClient struct {
	*http.Client
	config         model.Config
	metricsTracker *metric.Tracker
}

// NewNotDiamondHttpClient creates a new NotDiamond HTTP client.
func NewNotDiamondHttpClient(config model.Config) (*NotDiamondHttpClient, error) {
	var metricsTracker *metric.Tracker
	var err error

	if config.RedisConfig != nil {
		metricsTracker, err = metric.NewTracker(config.RedisConfig.Addr)
	} else {
		metricsTracker, err = metric.NewTracker("localhost:6379")
	}
	if err != nil {
		slog.Error("failed to create metrics tracker", "error", err)
		return nil, err
	}

	return &NotDiamondHttpClient{
		Client:         &http.Client{},
		config:         config,
		metricsTracker: metricsTracker,
	}, nil
}

// Do executes a request.
func (c *NotDiamondHttpClient) Do(req *http.Request) (*http.Response, error) {
	slog.Info("🔍 Executing request", "url", req.URL.String())

	// Read and log the initial request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	// Create a new reader for each operation that needs the body
	req.Body = io.NopCloser(bytes.NewBuffer(body))
	messages := request.ExtractMessagesFromRequest(req.Clone(req.Context()))

	req.Body = io.NopCloser(bytes.NewBuffer(body))
	extractedModel, err := request.ExtractModelFromRequest(req.Clone(req.Context()))
	if err != nil {
		return nil, fmt.Errorf("failed to extract model: %w", err)
	}

	req.Body = io.NopCloser(bytes.NewBuffer(body))
	extractedProvider := request.ExtractProviderFromRequest(req.Clone(req.Context()))

	currentModel := extractedProvider + "/" + extractedModel

	// Restore the original body for future operations
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	var lastErr error
	originalCtx := req.Context()

	if client, ok := originalCtx.Value(clientKey).(*Client); ok {
		var modelsToTry []string

		// Read and preserve the original request body
		originalBody, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read original request body: %v", err)
		}
		// Restore the body for future reads
		req.Body = io.NopCloser(bytes.NewBuffer(originalBody))

		if client.isOrdered {
			modelsToTry = client.models.(model.OrderedModels)
			// Validate that requested model is in the configured list
			modelExists := false
			for _, m := range modelsToTry {
				if m == currentModel {
					modelExists = true
					break
				}
			}
			if !modelExists {
				return nil, fmt.Errorf("requested model %s is not in the configured model list", currentModel)
			}
		} else {
			modelsToTry = getWeightedModelsList(client.models.(model.WeightedModels))
		}

		// Move the requested model to the front of the slice
		for i, m := range modelsToTry {
			if m == currentModel {
				// Remove it from its current position and insert at front
				modelsToTry = append(modelsToTry[:i], modelsToTry[i+1:]...)
				modelsToTry = append([]string{currentModel}, modelsToTry...)
				break
			}
		}

		for _, modelFull := range modelsToTry {
			// Reset the request body for each attempt
			req.Body = io.NopCloser(bytes.NewBuffer(originalBody))

			if resp, err := c.tryWithRetries(modelFull, req, messages, originalCtx); err == nil {
				return resp, nil
			} else {
				lastErr = err
				slog.Error("❌ Attempt failed", "model", modelFull, "error", err.Error())
			}
		}
	}

	return nil, fmt.Errorf("all requests failed: %v", lastErr)
}

// getMaxRetriesForStatus gets the maximum retries for a status code.
func (c *NotDiamondHttpClient) getMaxRetriesForStatus(modelFull string, statusCode int) int {
	// Check model-specific status code retries first
	if modelRetries, ok := c.config.StatusCodeRetry.(map[string]map[string]int); ok {
		if modelConfig, exists := modelRetries[modelFull]; exists {
			if retries, hasCode := modelConfig[strconv.Itoa(statusCode)]; hasCode {
				return retries
			}
		}
	}

	// Check global status code retries
	if globalRetries, ok := c.config.StatusCodeRetry.(map[string]int); ok {
		if retries, exists := globalRetries[strconv.Itoa(statusCode)]; exists {
			return retries
		}
	}

	// Fall back to default MaxRetries
	if maxRetries, exists := c.config.MaxRetries[modelFull]; exists {
		return maxRetries
	}
	return 1
}

// tryWithRetries tries a request with retries.
func (c *NotDiamondHttpClient) tryWithRetries(modelFull string, req *http.Request, messages []model.Message, originalCtx context.Context) (*http.Response, error) {
	var lastErr error
	var lastStatusCode int

	// Check model health (both latency and error rate) before starting attempts
	slog.Info("🏥 Checking initial model health", "model", modelFull)
	healthy, healthErr := c.metricsTracker.CheckModelOverallHealth(modelFull, c.config)
	if healthErr != nil {
		lastErr = healthErr
		slog.Error("❌ Initial health check failed", "model", modelFull, "error", healthErr.Error())
		return nil, fmt.Errorf("model health check failed: %w", healthErr)
	}
	if !healthy {
		lastErr = fmt.Errorf("model %s is unhealthy", modelFull)
		slog.Info("⚠️ Model is already unhealthy, skipping", "model", modelFull)
		return nil, lastErr
	}
	slog.Info("✅ Initial health check passed", "model", modelFull)

	for attempt := 0; ; attempt++ {
		maxRetries := c.getMaxRetriesForStatus(modelFull, lastStatusCode)
		if attempt >= maxRetries {
			break
		}

		slog.Info(fmt.Sprintf("🔄 Request %d of %d for model %s", attempt+1, maxRetries, modelFull))

		timeout := 100.0
		if t, ok := c.config.Timeout[modelFull]; ok && t > 0 {
			timeout = t
		}

		ctx, cancel := context.WithTimeout(originalCtx, time.Duration(timeout*float64(time.Second)))
		defer cancel()

		startTime := time.Now()
		var resp *http.Response
		var reqErr error

		if attempt == 0 {
			extractedModel, err := request.ExtractModelFromRequest(req)
			if err != nil {
				return nil, fmt.Errorf("failed to extract model: %w", err)
			}
			extractedProvider := request.ExtractProviderFromRequest(req)
			if modelFull == extractedProvider+"/"+extractedModel {
				currentReq := req.Clone(ctx)
				// Read and preserve the original request body
				body, err := io.ReadAll(currentReq.Body)
				if err != nil {
					return nil, err
				}
				currentReq.Body = io.NopCloser(bytes.NewBuffer(body))
				// Use a raw client for the initial request
				rawClient := &http.Client{}
				resp, reqErr = rawClient.Do(currentReq)
			}
		} else {
			if client, ok := originalCtx.Value(clientKey).(*Client); ok {
				resp, reqErr = tryNextModel(client, modelFull, messages, ctx, req)
			}
		}

		elapsed := time.Since(startTime).Seconds()

		if reqErr != nil {
			cancel()
			lastErr = reqErr
			slog.Error("❌ Request", "failed", lastErr)
			// Record the latency in Redis
			recErr := c.metricsTracker.RecordLatency(modelFull, elapsed, "failed")
			if recErr != nil {
				slog.Error("error", "recording latency", recErr)
			}
			if attempt < maxRetries-1 && c.config.Backoff[modelFull] > 0 {
				time.Sleep(time.Duration(c.config.Backoff[modelFull]) * time.Second)
			}
			continue
		}

		if resp != nil {
			body, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if closeErr != nil {
				return nil, closeErr
			}
			cancel()

			if readErr != nil {
				lastErr = readErr
				continue
			}

			lastStatusCode = resp.StatusCode
			if err := c.metricsTracker.RecordErrorCode(modelFull, resp.StatusCode); err != nil {
				slog.Error("Failed to record error code", "error", err)
			}

			// Check model health after recording the error code
			slog.Info("🏥 Checking model health after error", "model", modelFull, "status_code", resp.StatusCode)
			healthy, healthErr := c.metricsTracker.CheckModelOverallHealth(modelFull, c.config)
			if healthErr != nil {
				slog.Error("❌ Health check failed after error", "model", modelFull, "error", healthErr.Error())
				return nil, fmt.Errorf("model %s health check failed after error: %w", modelFull, healthErr)
			}
			if !healthy {
				slog.Info("⚠️ Model became unhealthy after error", "model", modelFull, "status_code", resp.StatusCode)
				return nil, fmt.Errorf("model %s became unhealthy after error", modelFull)
			}
			slog.Info("✅ Health check passed after error", "model", modelFull)

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				// Record the latency in Redis
				recErr := c.metricsTracker.RecordLatency(modelFull, elapsed, "success")
				if recErr != nil {
					slog.Error("recording latency", "error", recErr)
				}

				return &http.Response{
					Status:     resp.Status,
					StatusCode: resp.StatusCode,
					Header:     resp.Header,
					Body:       io.NopCloser(bytes.NewBuffer(body)),
				}, nil
			}

			// Parse error response body if possible
			var errorResponse struct {
				Error struct {
					Message string `json:"message"`
					Type    string `json:"type"`
				} `json:"error"`
			}
			if unmarshalErr := json.Unmarshal(body, &errorResponse); unmarshalErr == nil && errorResponse.Error.Message != "" {
				lastErr = fmt.Errorf("with status %d (%s): %s",
					resp.StatusCode,
					http.StatusText(resp.StatusCode),
					errorResponse.Error.Message)
			} else {
				// Fallback to raw body if can't parse error response
				lastErr = fmt.Errorf("with status %d (%s): %s",
					resp.StatusCode,
					http.StatusText(resp.StatusCode),
					string(body))
			}
			slog.Error("❌ Request", "failed", lastErr)
		}

		if attempt < maxRetries-1 && c.config.Backoff[modelFull] > 0 {
			time.Sleep(time.Duration(c.config.Backoff[modelFull]) * time.Second)
		}
	}

	return nil, lastErr
}

// getWeightedModelsList gets a list of models with their cumulative weights.
func getWeightedModelsList(weights model.WeightedModels) []string {
	// Create a slice to store models with their cumulative weights
	type weightedModel struct {
		model            string
		cumulativeWeight float64
	}

	models := make([]weightedModel, 0, len(weights))
	var cumulative float64

	// Calculate cumulative weights
	for model, weight := range weights {
		cumulative += weight
		models = append(models, weightedModel{
			model:            model,
			cumulativeWeight: cumulative,
		})
	}

	// Create result slice with the same models but ordered by weighted random selection
	result := make([]string, len(weights))
	remaining := make([]weightedModel, len(models))
	copy(remaining, models)

	for i := 0; i < len(weights); i++ {
		// Generate random number between 0 and remaining total weight
		r := rand.Float64() * remaining[len(remaining)-1].cumulativeWeight

		// Find the model whose cumulative weight range contains r
		selectedIdx := 0
		for j, m := range remaining {
			if r <= m.cumulativeWeight {
				selectedIdx = j
				break
			}
		}

		// Add selected model to result
		result[i] = remaining[selectedIdx].model

		// Remove selected model and recalculate cumulative weights
		remaining = append(remaining[:selectedIdx], remaining[selectedIdx+1:]...)
		cumulative = 0
		for j := range remaining {
			if j == 0 {
				cumulative = weights[remaining[j].model]
			} else {
				cumulative += weights[remaining[j].model]
			}
			remaining[j].cumulativeWeight = cumulative
		}
	}

	return result
}

// combineMessages combines model messages and user messages.
func combineMessages(modelMessages []model.Message, userMessages []model.Message) ([]model.Message, error) {
	combinedMessages := make([]model.Message, 0)

	// Find system message from modelMessages if any exists
	var systemMessage model.Message
	for _, msg := range modelMessages {
		if msg["role"] == "system" {
			systemMessage = msg
			break
		}
	}

	// Add the system message if found
	if systemMessage != nil {
		combinedMessages = append(combinedMessages, systemMessage)
	}

	// Add non-system messages from modelMessages
	for _, msg := range modelMessages {
		if msg["role"] != "system" {
			combinedMessages = append(combinedMessages, msg)
		}
	}

	// Add all non-system messages from userMessages
	for _, msg := range userMessages {
		if msg["role"] != "system" {
			combinedMessages = append(combinedMessages, msg)
		}
	}

	if err := validation.ValidateMessageSequence(combinedMessages); err != nil {
		slog.Error("invalid message sequence", "error", err)
		return nil, err
	}

	return combinedMessages, nil
}

func transformRequestForProvider(originalBody []byte, nextProvider, nextModel string, client *Client) ([]byte, error) {
	var jsonData []byte
	var err error

	switch nextProvider {
	case "azure", "openai":
		jsonData, err = transformToOpenAIFormat(originalBody, nextProvider, nextModel)
	case "vertex":
		jsonData, err = transformToVertexFormat(originalBody, nextModel, client)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", nextProvider)
	}

	return jsonData, err
}

// transformToOpenAIFormat transforms the request body to OpenAI/Azure format
func transformToOpenAIFormat(originalBody []byte, provider, modelName string) ([]byte, error) {
	var payload map[string]interface{}

	// If coming from Vertex, transform to OpenAI format first
	if bytes.Contains(originalBody, []byte("aiplatform.googleapis.com")) {
		transformed, err := request.TransformFromVertexToOpenAI(originalBody)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(transformed, &payload); err != nil {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(originalBody, &payload); err != nil {
			return nil, err
		}
	}

	// Update model name based on provider
	if provider == "openai" {
		payload["model"] = modelName
	} else if provider == "azure" {
		delete(payload, "model")
	}

	return json.Marshal(payload)
}

// transformToVertexFormat transforms the request body to Vertex format
func transformToVertexFormat(originalBody []byte, modelName string, client *Client) ([]byte, error) {
	return request.TransformToVertexRequest(originalBody, modelName)
}

// updateRequestURL updates the request URL based on the provider and model
func updateRequestURL(req *http.Request, provider, modelName string, client *Client) error {
	switch provider {
	case "azure":
		req.URL.Path = fmt.Sprintf("/openai/deployments/%s/chat/completions", modelName)
		// Use API version from config or fall back to default
		apiVersion := client.HttpClient.config.AzureAPIVersion
		if apiVersion == "" {
			apiVersion = "2023-05-15"
		}
		req.URL.RawQuery = fmt.Sprintf("api-version=%s", apiVersion)
	case "vertex":
		projectID := client.HttpClient.config.VertexProjectID
		location := client.HttpClient.config.VertexLocation
		req.URL.Host = fmt.Sprintf("%s-aiplatform.googleapis.com", location)
		req.URL.Scheme = "https"
		req.URL.Path = fmt.Sprintf("/v1beta1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
			projectID, location, modelName)
	}
	return nil
}

// updateRequestAuth updates the request authentication based on the provider
func updateRequestAuth(req *http.Request, provider string, ctx context.Context) error {
	// Extract API key from either header format
	var apiKey string
	authHeader := req.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		apiKey = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		apiKey = req.Header.Get("api-key")
	}

	switch provider {
	case "openai":
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Del("api-key")
	case "azure":
		req.Header.Set("api-key", apiKey)
		req.Header.Del("Authorization")
	case "vertex":
		credentials, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return fmt.Errorf("error getting credentials: %w", err)
		}
		token, err := credentials.TokenSource.Token()
		if err != nil {
			return fmt.Errorf("error getting token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}
	return nil
}

// tryNextModel tries the next model.
func tryNextModel(client *Client, modelFull string, messages []model.Message, ctx context.Context, originalReq *http.Request) (*http.Response, error) {
	parts := strings.Split(modelFull, "/")
	nextProvider, nextModel := parts[0], parts[1]

	var nextReq *http.Request

	// Find matching client request
	for _, clientReq := range client.clients {
		url := clientReq.URL.String()
		switch nextProvider {
		case "vertex":
			if strings.Contains(url, "aiplatform.googleapis.com") {
				nextReq = clientReq.Clone(ctx)
				slog.Info("🔄 Fallback to", "model:", modelFull, "| URL:", nextReq.URL.String())
				goto found
			}
		case "azure":
			if strings.Contains(url, "azure") {
				nextReq = clientReq.Clone(ctx)
				slog.Info("🔄 Fallback to", "model:", modelFull, "| URL:", nextReq.URL.String())
				goto found
			}
		case "openai":
			if strings.Contains(url, "openai.com") {
				nextReq = clientReq.Clone(ctx)
				slog.Info("🔄 Fallback to", "model:", modelFull, "| URL:", nextReq.URL.String())
				goto found
			}
		}
	}
found:

	if nextReq == nil {
		slog.Info("❌ No matching client found", "provider", nextProvider)
		return nil, fmt.Errorf("no client found for provider %s", nextProvider)
	}

	// Read and preserve the original request body
	originalBody, err := io.ReadAll(originalReq.Body)
	if err != nil {
		return nil, err
	}
	originalReq.Body = io.NopCloser(bytes.NewBuffer(originalBody))

	// Transform request body for the target provider
	jsonData, err := transformRequestForProvider(originalBody, nextProvider, nextModel, client)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	nextReq.Body = io.NopCloser(bytes.NewBuffer(jsonData))

	// Initialize headers if nil
	if nextReq.Header == nil {
		nextReq.Header = make(http.Header)
	}
	nextReq.Header.Set("Content-Type", "application/json")

	// Update request URL
	if err := updateRequestURL(nextReq, nextProvider, nextModel, client); err != nil {
		return nil, fmt.Errorf("failed to update URL: %w", err)
	}

	// Update authentication
	if err := updateRequestAuth(nextReq, nextProvider, ctx); err != nil {
		return nil, fmt.Errorf("failed to update authentication: %w", err)
	}

	return client.HttpClient.Client.Do(nextReq)
}
