package notdiamond

import (
	"context"
	"net/http"
	"strings"

	"github.com/Not-Diamond/go-notdiamond/metric"
	"github.com/Not-Diamond/go-notdiamond/types"
)

type Transport struct {
	Base           http.RoundTripper
	client         *Client
	metricsTracker *metric.Tracker
	config         types.Config
}

func NewTransport(config types.Config) (*Transport, error) {
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
	ctx := context.WithValue(req.Context(), clientKey, t.client)
	req = req.WithContext(ctx)

	return t.client.HttpClient.Do(req)
}

func buildModelProviders(models types.Models) map[string]map[string]bool {
	modelProviders := make(map[string]map[string]bool)

	switch m := models.(type) {
	case types.WeightedModels:
		for modelFull := range m {
			parts := strings.Split(modelFull, "/")
			provider, model := parts[0], parts[1]
			if modelProviders[model] == nil {
				modelProviders[model] = make(map[string]bool)
			}
			modelProviders[model][provider] = true
		}
	case types.OrderedModels:
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

func isOrderedModels(models types.Models) bool {
	_, ok := models.(types.OrderedModels)
	return ok
}
