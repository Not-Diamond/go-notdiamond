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
)

// NotDiamondHttpClient is a type that can be used to represent a NotDiamond HTTP client.
type NotDiamondHttpClient struct {
	*http.Client
	config         model.Config
	metricsTracker *metric.Tracker
}

// NewNotDiamondHttpClient creates a new NotDiamond HTTP client.
func NewNotDiamondHttpClient(config model.Config) (*NotDiamondHttpClient, error) {
	metricsTracker, err := metric.NewTracker("metrics")
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
	slog.Info("→ Executing request", "url", req.URL.String())

	messages := request.ExtractMessagesFromRequest(req)
	extractedModel := request.ExtractModelFromRequest(req)
	extractedProvider := request.ExtractProviderFromRequest(req)
	currentModel := extractedProvider + "/" + extractedModel

	var lastErr error
	originalCtx := req.Context()

	if client, ok := originalCtx.Value(clientKey).(*Client); ok {
		var modelsToTry []string

		if client.isOrdered {
			modelsToTry = client.models.(model.OrderedModels)
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
			if resp, err := c.tryWithRetries(modelFull, req, messages, originalCtx); err == nil {
				return resp, nil
			} else {
				lastErr = err
				slog.Error("✕ Attempt failed", "model", modelFull, "error", err.Error())
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

	for attempt := 0; ; attempt++ {
		maxRetries := c.getMaxRetriesForStatus(modelFull, lastStatusCode)
		if attempt >= maxRetries {
			break
		}

		slog.Info(fmt.Sprintf("↺ Request %d of %d for model %s", attempt+1, maxRetries, modelFull))

		timeout := 100.0
		if t, ok := c.config.Timeout[modelFull]; ok && t > 0 {
			timeout = t
		}

		ctx, cancel := context.WithTimeout(originalCtx, time.Duration(timeout*float64(time.Second)))
		defer cancel()

		_err := c.metricsTracker.CheckModelHealth(modelFull, "success", c.config)
		if _err != nil {
			cancel()
			lastErr = _err
			slog.Error("Model health check failed", "model", modelFull, "error", _err.Error())
			// Don't retry if model is unhealthy or in recovery period
			if strings.Contains(_err.Error(), "is unhealthy") || strings.Contains(_err.Error(), "recovery period") {
				return nil, _err
			}
			if attempt < maxRetries-1 && c.config.Backoff[modelFull] > 0 {
				time.Sleep(time.Duration(c.config.Backoff[modelFull]) * time.Second)
			}
			continue
		}

		startTime := time.Now()
		var resp *http.Response
		var err error

		if attempt == 0 && modelFull == request.ExtractProviderFromRequest(req)+"/"+request.ExtractModelFromRequest(req) {
			currentReq := req.Clone(ctx)
			resp, err = c.Client.Do(currentReq)
		} else {
			if client, ok := originalCtx.Value(clientKey).(*Client); ok {
				resp, err = tryNextModel(client, modelFull, messages, ctx)
			}
		}

		elapsed := time.Since(startTime).Seconds()

		if err != nil {
			cancel()
			lastErr = err
			slog.Error("! Request", "failed", lastErr)
			// Record the latency in SQLite.
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
			err := resp.Body.Close()
			if err != nil {
				return nil, err
			}
			cancel()

			if readErr != nil {
				lastErr = readErr
				continue
			}

			lastStatusCode = resp.StatusCode
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				// Record the latency in SQLite.
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
			if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.Error.Message != "" {
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
			slog.Error("! Request", "failed", lastErr)
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

// tryNextModel tries the next model.
func tryNextModel(client *Client, modelFull string, messages []model.Message, ctx context.Context) (*http.Response, error) {
	parts := strings.Split(modelFull, "/")
	nextProvider, nextModel := parts[0], parts[1]

	var nextReq *http.Request

	for _, clientReq := range client.clients {
		if strings.Contains(clientReq.URL.String(), nextProvider) {
			nextReq = clientReq.Clone(ctx)
			slog.Info("↝ Fallback to", "model:", modelFull, "| URL:", nextReq.URL.String())
			break
		}
	}

	if nextReq == nil {
		slog.Info("! No more fallbacks available for", "model:", modelFull)
		return nil, fmt.Errorf("no client found for provider %s", nextProvider)
	}

	if nextProvider == "azure" {
		nextReq.URL.Path = fmt.Sprintf("/openai/deployments/%s/chat/completions", nextModel)
		nextReq.URL.RawQuery = "api-version=2023-05-15"
	}

	modelMessages := client.HttpClient.config.ModelMessages[modelFull]
	combinedMessages, err := combineMessages(modelMessages, messages)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"model":    nextModel,
		"messages": combinedMessages,
	}

	if nextProvider == "azure" {
		delete(payload, "model")
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	nextReq.Body = io.NopCloser(bytes.NewBuffer(jsonData))

	// Initialize headers if nil
	if nextReq.Header == nil {
		nextReq.Header = make(http.Header)
	}
	nextReq.Header.Set("Content-Type", "application/json")

	// Extract API key from either header format
	var apiKey string
	authHeader := nextReq.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		apiKey = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		apiKey = nextReq.Header.Get("api-key")
	}

	if nextProvider == "openai" {
		nextReq.Header.Set("Authorization", "Bearer "+apiKey)
		nextReq.Header.Del("api-key")
	} else if nextProvider == "azure" {
		nextReq.Header.Set("api-key", apiKey)
		nextReq.Header.Del("Authorization")
	}

	return client.HttpClient.Client.Do(nextReq)
}
