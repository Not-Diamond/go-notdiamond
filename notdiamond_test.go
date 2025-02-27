package notdiamond

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
	"github.com/Not-Diamond/go-notdiamond/pkg/redis"
	"github.com/alicebob/miniredis/v2"
)

func TestInit(t *testing.T) {
	// Set up miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create miniredis: %v", err)
	}
	defer mr.Close()

	tests := []struct {
		name      string
		config    model.Config
		wantErr   bool
		errString string
	}{
		{
			name: "valid ordered models config",
			config: model.Config{
				Clients: []http.Request{
					{
						Host: "api.openai.com",
						URL: &url.URL{
							Scheme: "https",
							Host:   "api.openai.com",
							Path:   "/v1/chat/completions",
						},
					},
				},
				Models: model.OrderedModels{"openai/gpt-4"},
				RedisConfig: &redis.Config{
					Addr:     mr.Addr(),
					Password: "",
					DB:       0,
				},
			},
			wantErr: false,
		},
		{
			name: "valid weighted models config",
			config: model.Config{
				Clients: []http.Request{
					{
						Host: "api.openai.com",
						URL: &url.URL{
							Scheme: "https",
							Host:   "api.openai.com",
							Path:   "/v1/chat/completions",
						},
					},
				},
				Models: model.WeightedModels{
					"openai/gpt-4": 1.0,
				},
				RedisConfig: &redis.Config{
					Addr:     mr.Addr(),
					Password: "",
					DB:       0,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - no clients",
			config: model.Config{
				Models: model.OrderedModels{"openai/gpt-4"},
			},
			wantErr:   true,
			errString: "at least one client must be provided",
		},
		{
			name: "invalid - no models",
			config: model.Config{
				Clients: []http.Request{
					{
						Host: "api.openai.com",
						URL: &url.URL{
							Scheme: "https",
							Host:   "api.openai.com",
							Path:   "/v1/chat/completions",
						},
					},
				},
				Models: nil,
			},
			wantErr:   true,
			errString: "models must be either notdiamond.OrderedModels or map[string]float64, got <nil>",
		},
		{
			name: "invalid - incorrect model format",
			config: model.Config{
				Clients: []http.Request{
					{
						Host: "api.openai.com",
						URL: &url.URL{
							Scheme: "https",
							Host:   "api.openai.com",
							Path:   "/v1/chat/completions",
						},
					},
				},
				Models: model.OrderedModels{"invalid-model-format"},
			},
			wantErr:   true,
			errString: "invalid model format: invalid-model-format (expected 'provider/model' or 'provider/model/region')",
		},
		{
			name: "invalid - unknown provider",
			config: model.Config{
				Clients: []http.Request{
					{
						Host: "api.openai.com",
						URL: &url.URL{
							Scheme: "https",
							Host:   "api.openai.com",
							Path:   "/v1/chat/completions",
						},
					},
				},
				Models: model.OrderedModels{"unknown/gpt-4"},
			},
			wantErr:   true,
			errString: "invalid provider in model unknown/gpt-4: unknown provider: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := Init(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.errString)
				return
			}
			if err == nil {
				if client == nil {
					t.Error("Init() returned nil client with no error")
					return
				}
				// Clean up
				if err := client.HttpClient.metricsTracker.Close(); err != nil {
					t.Errorf("Failed to close metrics tracker: %v", err)
				}
			}
		})
	}
}
