package notdiamond

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	clients        []http.Request
	models         Models
	modelProviders map[string]map[string]bool
	isOrdered      bool
	HttpClient     *NotDiamondHttpClient
}

type contextKey string

const NotdiamondClientKey contextKey = "notdiamondClient"

func Init(config Config) (*Client, error) {
	infoLog("⚡ Initializing Client...")

	if err := validateConfig(config); err != nil {
		errorLog("Config validation failed:", err)
		return nil, err
	}

	isOrdered := false
	if _, ok := config.Models.(OrderedModels); ok {
		isOrdered = true
	}

	// Create modelProviders map
	modelProviders := make(map[string]map[string]bool)

	switch models := config.Models.(type) {
	case WeightedModels:
		for modelFull := range models {
			parts := strings.Split(modelFull, "/")
			provider, model := parts[0], parts[1]

			if modelProviders[model] == nil {
				modelProviders[model] = make(map[string]bool)
			}
			modelProviders[model][provider] = true
		}
	case OrderedModels:
		for _, modelFull := range models {
			parts := strings.Split(modelFull, "/")
			provider, model := parts[0], parts[1]

			if modelProviders[model] == nil {
				modelProviders[model] = make(map[string]bool)
			}
			modelProviders[model][provider] = true
		}
	}
	ndHttpClient, err := NewNotDiamondHttpClient(config)
	if err != nil {
		return nil, err
	}
	client := &Client{
		clients:        config.Clients,
		models:         config.Models,
		modelProviders: modelProviders,
		isOrdered:      isOrdered,
		HttpClient:     ndHttpClient,
	}

	return client, nil
}

// Do Extend and added context to the request
// So the package can be used without manually passing not diamond client to the ctx
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	ctx := context.WithValue(context.Background(), NotdiamondClientKey, c)

	newReq := req.Clone(ctx)
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewBuffer(body)) // Restore original request body
		newReq.Body = io.NopCloser(bytes.NewBuffer(body))
	}

	return c.HttpClient.Do(newReq)
}
