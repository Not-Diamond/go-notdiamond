package notdiamond

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/Not-Diamond/go-notdiamond/pkg/validation"
)

type Client struct {
	clients        []http.Request
	models         model.Models
	modelProviders map[string]map[string]bool
	isOrdered      bool
	HttpClient     *NotDiamondHttpClient
}

type contextKey string

const clientKey contextKey = "notdiamondClient"

// ClientKey returns the context key used for storing the NotDiamond client
func ClientKey() interface{} {
	return clientKey
}

func Init(config model.Config) (*Client, error) {
	slog.Info("▷ Initializing Client...")

	// Log Vertex AI configuration
	slog.Info("▷ Vertex AI Configuration",
		"project_id", config.VertexProjectID,
		"location", config.VertexLocation)

	if err := validation.ValidateConfig(config); err != nil {
		slog.Error("validation", "error", err.Error())
		return nil, err
	}

	isOrdered := false
	if _, ok := config.Models.(model.OrderedModels); ok {
		isOrdered = true
	}

	// Create modelProviders map
	modelProviders := make(map[string]map[string]bool)

	switch models := config.Models.(type) {
	case model.WeightedModels:
		for modelFull := range models {
			parts := strings.Split(modelFull, "/")
			provider := parts[0]
			modelName := parts[1]

			// Handle region if present
			if len(parts) > 2 {
				// For provider/model/region format, we use model as the key
				// but we don't include the region in the key
				if modelProviders[modelName] == nil {
					modelProviders[modelName] = make(map[string]bool)
				}
				modelProviders[modelName][provider] = true
			} else {
				// For provider/model format
				if modelProviders[modelName] == nil {
					modelProviders[modelName] = make(map[string]bool)
				}
				modelProviders[modelName][provider] = true
			}
		}
	case model.OrderedModels:
		for _, modelFull := range models {
			parts := strings.Split(modelFull, "/")
			provider := parts[0]
			modelName := parts[1]

			// Handle region if present
			if len(parts) > 2 {
				// For provider/model/region format, we use model as the key
				// but we don't include the region in the key
				if modelProviders[modelName] == nil {
					modelProviders[modelName] = make(map[string]bool)
				}
				modelProviders[modelName][provider] = true
			} else {
				// For provider/model format
				if modelProviders[modelName] == nil {
					modelProviders[modelName] = make(map[string]bool)
				}
				modelProviders[modelName][provider] = true
			}
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
