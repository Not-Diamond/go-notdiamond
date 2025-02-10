package notdiamond

import (
	"net/http"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid ordered models config",
			config: Config{
				Clients: []http.Request{
					*&http.Request{},
				},
				Models: OrderedModels{
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
			config: Config{
				Clients: []http.Request{
					*&http.Request{},
				},
				Models: WeightedModels{
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
			config: Config{
				Models: OrderedModels{
					"openai/gpt-4",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - no models",
			config: Config{
				Clients: []http.Request{
					*&http.Request{},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - incorrect model format",
			config: Config{
				Clients: []http.Request{
					*&http.Request{},
				},
				Models: OrderedModels{
					"invalid-model-format",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - unknown provider",
			config: Config{
				Clients: []http.Request{
					*&http.Request{},
				},
				Models: OrderedModels{
					"unknown/gpt-4",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a temporary directory for the database files so that each test is isolated.
			DataFolder = t.TempDir()

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
