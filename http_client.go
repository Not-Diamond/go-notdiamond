package notdiamond

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type NotDiamondHttpClient struct {
	*http.Client
	config         *Config
	metricsTracker *MetricsTracker
}

func NewNotDiamondHttpClient(config Config) (*NotDiamondHttpClient, error) {
	metricsTracker, err := NewMetricsTracker("metrics")
	if err != nil {
		Error("Failed to create metrics tracker:", err)
		return nil, err
	}

	return &NotDiamondHttpClient{
		Client:         &http.Client{},
		config:         &config,
		metricsTracker: metricsTracker,
	}, nil
}

func (c *NotDiamondHttpClient) Do(req *http.Request) (*http.Response, error) {
	Info("Doing request via custom Do: ", req.URL.String())

	messages := extractMessagesFromRequest(req)
	model := extractModelFromRequest(req)
	provider := extractProviderFromRequest(req)
	currentModel := provider + "/" + model

	var lastErr error
	originalCtx := req.Context()

	if client, ok := originalCtx.Value(NotdiamondClientKey).(*Client); ok {
		var modelsToTry []string

		if client.isOrdered {
			modelsToTry = client.models.(OrderedModels)
		} else {
			modelsToTry = getWeightedModelsList(client.models.(WeightedModels))
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

		Info("Models to try in order: ", modelsToTry)

		for _, modelFull := range modelsToTry {
			if resp, err := c.tryWithRetries(modelFull, req, messages, originalCtx); err == nil {
				return resp, nil
			} else {
				lastErr = err
				Error("Attempt failed for model ", modelFull, ": ", err)
			}
		}
	}

	return nil, fmt.Errorf("all attempts failed: %v", lastErr)
}

func (c *NotDiamondHttpClient) getMaxRetriesForStatus(modelFull string, statusCode int) int {
	Info("Getting max retries for status code: ", statusCode)
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

func (c *NotDiamondHttpClient) tryWithRetries(modelFull string, req *http.Request, messages []Message, originalCtx context.Context) (*http.Response, error) {
	var lastErr error
	var lastStatusCode int

	for attempt := 0; ; attempt++ {
		maxRetries := c.getMaxRetriesForStatus(modelFull, lastStatusCode)
		if attempt >= maxRetries {
			break
		}

		Info(fmt.Sprintf("Attempt %d of %d for model %s", attempt+1, maxRetries, modelFull))

		timeout := 30.0
		if t, ok := c.config.Timeout[modelFull]; ok && t > 0 {
			timeout = t
		}

		healthy, _err := c.metricsTracker.CheckModelHealth(modelFull, c.config)
		if _err != nil {
			Error("Error checking model health:", _err)
		}
		if !healthy {
			lastErr = fmt.Errorf("model %s is unhealthy (average latency too high)", modelFull)
			Error(lastErr)
			// Do not retry further; return error to trigger fallback.
			return nil, lastErr
		}

		ctx, cancel := context.WithTimeout(originalCtx, time.Duration(timeout*float64(time.Second)))
		defer cancel()

		startTime := time.Now()
		var resp *http.Response
		var err error

		if attempt == 0 && modelFull == extractProviderFromRequest(req)+"/"+extractModelFromRequest(req) {
			currentReq := req.Clone(ctx)
			resp, err = c.Client.Do(currentReq)
		} else {
			if client, ok := originalCtx.Value(NotdiamondClientKey).(*Client); ok {
				resp, err = tryNextModel(client, modelFull, messages, ctx)
			}
		}

		elapsed := time.Since(startTime).Seconds()
		// Record the latency in SQLite.
		recErr := c.metricsTracker.RecordLatency(modelFull, elapsed)
		if recErr != nil {
			Error("Error recording latency:", recErr)
		}

		if err != nil {
			cancel()
			lastErr = err
			Error("Request failed:", err)
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
				return &http.Response{
					Status:     resp.Status,
					StatusCode: resp.StatusCode,
					Header:     resp.Header,
					Body:       io.NopCloser(bytes.NewBuffer(body)),
				}, nil
			}

			lastErr = fmt.Errorf("request failed with status code %d: %s", resp.StatusCode, string(body))
			Error("Request failed:", lastErr)
		}

		if attempt < maxRetries-1 && c.config.Backoff[modelFull] > 0 {
			time.Sleep(time.Duration(c.config.Backoff[modelFull]) * time.Second)
		}
	}

	return nil, lastErr
}

func getWeightedModelsList(weights map[string]float64) []string {
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

func combineMessages(modelMessages []Message, userMessages []Message) []Message {
	combinedMessages := make([]Message, 0)
	if len(modelMessages) > 0 {
		combinedMessages = append(combinedMessages, modelMessages...)
	}
	combinedMessages = append(combinedMessages, userMessages...)
	return combinedMessages
}

func tryNextModel(client *Client, modelFull string, messages []Message, ctx context.Context) (*http.Response, error) {
	parts := strings.Split(modelFull, "/")
	nextProvider, nextModel := parts[0], parts[1]

	var nextReq *http.Request

	for _, clientReq := range client.clients {
		if strings.Contains(clientReq.URL.String(), nextProvider) {
			nextReq = clientReq.Clone(ctx)
			Info("Fallback URL: ", nextReq.URL.String())
			break
		}
	}

	if nextReq == nil {
		return nil, fmt.Errorf("no client found for provider %s", nextProvider)
	}

	if nextProvider == "azure" {
		nextReq.URL.Path = fmt.Sprintf("/openai/deployments/%s/chat/completions", nextModel)
		nextReq.URL.RawQuery = "api-version=2023-05-15"
	}

	modelMessages := client.HttpClient.config.ModelMessages[modelFull]
	combinedMessages := combineMessages(modelMessages, messages)

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
	nextReq.Header.Set("Content-Type", "application/json")

	if nextProvider == "openai" {
		nextReq.Header.Set("Authorization", "Bearer "+nextReq.Header.Get("api-key"))
		nextReq.Header.Del("api-key")
	}

	return client.HttpClient.Client.Do(nextReq)
}

func extractModelFromRequest(req *http.Request) string {
	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload map[string]interface{}
	err := json.Unmarshal(body, &payload)
	if err != nil {
		return ""
	}

	if model, ok := payload["model"].(string); ok {
		return model
	}
	return ""
}

func extractProviderFromRequest(req *http.Request) string {
	url := req.URL.String()
	if strings.Contains(url, "azure") {
		return "azure"
	} else if strings.Contains(url, "openai.com") {
		return "openai"
	}
	return ""
}

func extractMessagesFromRequest(req *http.Request) []Message {
	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload struct {
		Messages []Message `json:"messages"`
	}
	err := json.Unmarshal(body, &payload)
	if err != nil {
		return nil
	}
	return payload.Messages
}
