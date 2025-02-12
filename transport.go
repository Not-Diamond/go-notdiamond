package notdiamond

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"bytes"

	"github.com/Not-Diamond/go-notdiamond/pkg/http/request"
	"github.com/Not-Diamond/go-notdiamond/pkg/metric"
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/Not-Diamond/go-notdiamond/pkg/validation"
)

type Transport struct {
	Base           http.RoundTripper
	client         *Client
	metricsTracker *metric.Tracker
	config         model.Config
}

// NewTransport creates a new Transport.
func NewTransport(config model.Config) (*Transport, error) {
	if err := validation.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	metricsTracker, err := metric.NewTracker("metrics")
	if err != nil {
		return nil, err
	}

	baseClient := &http.Client{Transport: http.DefaultTransport}

	ndHttpClient := &NotDiamondHttpClient{
		Client:         baseClient,
		config:         config,
		metricsTracker: metricsTracker,
	}

	client := &Client{
		clients:        config.Clients,
		models:         config.Models,
		modelProviders: buildModelProviders(config.Models),
		isOrdered:      isOrderedModels(config.Models),
		HttpClient:     ndHttpClient,
	}

	return &Transport{
		Base:           http.DefaultTransport,
		client:         client,
		metricsTracker: metricsTracker,
		config:         config,
	}, nil
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Extract the original messages and model
	messages := request.ExtractMessagesFromRequest(req)
	extractedModel := request.ExtractModelFromRequest(req)
	extractedProvider := request.ExtractProviderFromRequest(req)
	currentModel := extractedProvider + "/" + extractedModel

	// Combine with model messages if they exist
	if modelMessages, exists := t.config.ModelMessages[currentModel]; exists {
		combinedMessages, err := combineMessages(modelMessages, messages)
		if err != nil {
			return nil, err
		}

		// Update request body with combined messages
		payload := map[string]interface{}{
			"model":    extractedModel,
			"messages": combinedMessages,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		req.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		req.ContentLength = int64(len(jsonData))
	}

	// Add client to context and proceed with request
	ctx := context.WithValue(req.Context(), clientKey, t.client)
	req = req.WithContext(ctx)

	return t.client.HttpClient.Do(req)
}

func buildModelProviders(models model.Models) map[string]map[string]bool {
	modelProviders := make(map[string]map[string]bool)

	switch m := models.(type) {
	case model.WeightedModels:
		for modelFull := range m {
			parts := strings.Split(modelFull, "/")
			provider, model := parts[0], parts[1]
			if modelProviders[model] == nil {
				modelProviders[model] = make(map[string]bool)
			}
			modelProviders[model][provider] = true
		}
	case model.OrderedModels:
		for _, modelFull := range m {
			parts := strings.Split(modelFull, "/")
			provider, model := parts[0], parts[1]
			if modelProviders[model] == nil {
				modelProviders[model] = make(map[string]bool)
			}
			modelProviders[model][provider] = true
		}
	}
	return modelProviders
}

func isOrderedModels(models model.Models) bool {
	_, ok := models.(model.OrderedModels)
	return ok
}
