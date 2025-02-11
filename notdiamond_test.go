package notdiamond

import (
	"net/http"
	"testing"

	"github.com/Not-Diamond/go-notdiamond/database"
	"github.com/Not-Diamond/go-notdiamond/types"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		config  types.Config
		wantErr bool
	}{
		{
			name: "valid ordered models config",
			config: types.Config{
				Clients: []http.Request{
					*&http.Request{},
				},
				Models: types.OrderedModels{
					"openai/gpt-4",
					"azure/gpt-4",
				},
				MaxRetries: map[string]int{
					"openai/gpt-4": 3,
					"azure/gpt-4":  3,
				},
				Timeout: map[string]float64{
					"openai/gpt-4": 30.0,
					"azure/gpt-4":  30.0,
				},
			},
			wantErr: false,
		},
		{
			name: "valid weighted models config",
			config: types.Config{
				Clients: []http.Request{
					*&http.Request{},
				},
				Models: types.WeightedModels{
					"openai/gpt-4": 0.6,
					"azure/gpt-4":  0.4,
				},
				MaxRetries: map[string]int{
					"openai/gpt-4": 3,
					"azure/gpt-4":  3,
				},
				Timeout: map[string]float64{
					"openai/gpt-4": 30.0,
					"azure/gpt-4":  30.0,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - no clients",
			config: types.Config{
				Models: types.OrderedModels{
					"openai/gpt-4",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - no models",
			config: types.Config{
				Clients: []http.Request{
					*&http.Request{},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - incorrect model format",
			config: types.Config{
				Clients: []http.Request{
					*&http.Request{},
				},
				Models: types.OrderedModels{
					"invalid-model-format",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - unknown provider",
			config: types.Config{
				Clients: []http.Request{
					*&http.Request{},
				},
				Models: types.OrderedModels{
					"unknown/gpt-4",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a temporary directory for the database files so that each test is isolated.
			database.DataFolder = t.TempDir()

			client, err := Init(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If the client was successfully created, make sure to clean up its resources.
			if client != nil && client.HttpClient != nil && client.HttpClient.metricsTracker != nil {
				if cerr := client.HttpClient.metricsTracker.Close(); cerr != nil {
					t.Errorf("failed to close metrics tracker: %v", cerr)
				}
			}
		})
	}
}
