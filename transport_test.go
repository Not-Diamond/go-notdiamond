package notdiamond

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/Not-Diamond/go-notdiamond/pkg/database"
	"github.com/Not-Diamond/go-notdiamond/pkg/metric"
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

type testMockTransport struct {
	responses   []*http.Response
	errors      []error
	lastRequest *http.Request
	callCount   int
}

func (m *testMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Store the request for later inspection
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	// Store the request with a fresh body copy
	m.lastRequest = req.Clone(req.Context())
	m.lastRequest.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Restore original request body
	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	m.callCount++

	// For invalid JSON test case
	if strings.Contains(string(bodyBytes), "invalid_json") && len(m.errors) > 0 {
		return nil, m.errors[0]
	}

	// Return mock response for valid cases
	if len(m.responses) > 0 && m.responses[0] != nil {
		resp := m.responses[0]
		// Ensure response has a body
		if resp.Body == nil {
			resp.Body = io.NopCloser(bytes.NewBufferString("{}"))
		}
		// Read and clone the response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			respBody = []byte("{}")
		}
		return &http.Response{
			Status:     resp.Status,
			StatusCode: resp.StatusCode,
			Body:       io.NopCloser(bytes.NewBuffer(respBody)),
			Header:     resp.Header,
		}, nil
	}

	return nil, fmt.Errorf("no response configured")
}

func TestNewTransport(t *testing.T) {
	tests := []struct {
		name      string
		config    model.Config
		wantErr   bool
		errString string
	}{
		{
			name: "valid config with ordered models",
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
					{
						Host: "myresource.azure.openai.com",
						URL: &url.URL{
							Scheme: "https",
							Host:   "myresource.azure.openai.com",
							Path:   "/openai/deployments/gpt-4/chat/completions",
						},
					},
				},
				Models: model.OrderedModels{"openai/gpt-4", "azure/gpt-4"},
				ModelLatency: model.ModelLatency{
					"openai/gpt-4": &model.RollingAverageLatency{
						AvgLatencyThreshold: 3.5,
						NoOfCalls:           5,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with weighted models",
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
					{
						Host: "myresource.azure.openai.com",
						URL: &url.URL{
							Scheme: "https",
							Host:   "myresource.azure.openai.com",
							Path:   "/openai/deployments/gpt-4/chat/completions",
						},
					},
				},
				Models: model.WeightedModels{
					"openai/gpt-4": 0.6,
					"azure/gpt-4":  0.4,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid config - empty models",
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
				Models: model.OrderedModels{},
			},
			wantErr:   true,
			errString: "invalid config: at least one model must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a temporary directory for database files
			database.DataFolder = t.TempDir()

			transport, err := NewTransport(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTransport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errString != "" && !reflect.DeepEqual(err.Error(), tt.errString) {
				t.Errorf("NewTransport() error = %v, wantErr %v", err, tt.errString)
				return
			}
			if err == nil {
				if transport == nil {
					t.Error("NewTransport() returned nil transport with no error")
					return
				}
				// Clean up
				if err := transport.metricsTracker.Close(); err != nil {
					t.Errorf("Failed to close metrics tracker: %v", err)
				}
			}
		})
	}
}

func TestBuildModelProviders(t *testing.T) {
	tests := []struct {
		name     string
		models   model.Models
		expected map[string]map[string]bool
	}{
		{
			name: "ordered models",
			models: model.OrderedModels{
				"openai/gpt-4",
				"azure/gpt-4",
				"openai/gpt-3.5-turbo",
			},
			expected: map[string]map[string]bool{
				"gpt-4": {
					"openai": true,
					"azure":  true,
				},
				"gpt-3.5-turbo": {
					"openai": true,
				},
			},
		},
		{
			name: "weighted models",
			models: model.WeightedModels{
				"openai/gpt-4": 0.6,
				"azure/gpt-4":  0.4,
			},
			expected: map[string]map[string]bool{
				"gpt-4": {
					"openai": true,
					"azure":  true,
				},
			},
		},
		{
			name:     "empty models",
			models:   model.OrderedModels{},
			expected: map[string]map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildModelProviders(tt.models)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("buildModelProviders() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTransport_RoundTrip(t *testing.T) {
	tests := []struct {
		name          string
		requestBody   string
		modelMessages map[string][]model.Message
		expectedBody  string
		mockResponse  *http.Response
		mockError     error
		expectError   bool
		errorContains string
		checkRequest  func(t *testing.T, req *http.Request)
	}{
		{
			name: "basic request without model messages",
			requestBody: `{
				"model": "openai/gpt-4",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			mockResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"success": true}`)),
			},
			checkRequest: func(t *testing.T, req *http.Request) {
				body, _ := io.ReadAll(req.Body)
				if !strings.Contains(string(body), `"content":"Hello"`) {
					t.Errorf("Expected original message to be preserved, got %s", string(body))
				}
			},
		},
		{
			name: "request with model messages",
			requestBody: `{
				"model": "openai/gpt-4",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			modelMessages: map[string][]model.Message{
				"openai/gpt-4": {
					{"role": "system", "content": "You are a helpful assistant"},
				},
			},
			checkRequest: func(t *testing.T, req *http.Request) {
				body, _ := io.ReadAll(req.Body)
				if !strings.Contains(string(body), `"content":"You are a helpful assistant"`) {
					t.Errorf("Expected system message to be included, got %s", string(body))
				}
				if !strings.Contains(string(body), `"content":"Hello"`) {
					t.Errorf("Expected user message to be preserved, got %s", string(body))
				}
			},
			mockResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"success": true}`)),
			},
		},
		{
			name: "invalid request body",
			requestBody: `{
				"model": "gpt-4",
				"messages": invalid_json
			}`,
			mockResponse: &http.Response{
				StatusCode: 400,
				Body:       io.NopCloser(bytes.NewBufferString(`{"error": "invalid character 'i' looking for beginning of value"}`)),
			},
			mockError:     fmt.Errorf("invalid character 'i' looking for beginning of value"),
			expectError:   true,
			errorContains: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a temporary directory for database files
			database.DataFolder = t.TempDir()

			// Create mock transport
			mockTransport := &testMockTransport{
				responses: []*http.Response{tt.mockResponse},
				errors:    []error{tt.mockError},
			}

			// Create metrics tracker with unique name for each test
			metrics, err := metric.NewTracker(":memory:" + tt.name)
			if err != nil {
				t.Fatalf("Failed to create metrics tracker: %v", err)
			}
			defer metrics.Close()

			// Create transport with mock client
			transport := &Transport{
				Base:           mockTransport,
				metricsTracker: metrics,
				config: model.Config{
					ModelMessages: tt.modelMessages,
					Models: model.WeightedModels{
						"openai/gpt-4": 1.0,
					},
				},
				client: &Client{
					HttpClient: &NotDiamondHttpClient{
						Client: &http.Client{Transport: mockTransport},
						config: model.Config{
							Models: model.WeightedModels{
								"openai/gpt-4": 1.0,
							},
							ModelMessages: tt.modelMessages,
						},
						metricsTracker: metrics,
					},
					models: model.WeightedModels{
						"openai/gpt-4": 1.0,
					},
					isOrdered: false,
					clients: []http.Request{
						{
							Method: "POST",
							Host:   "api.openai.com",
							URL: &url.URL{
								Scheme: "https",
								Host:   "api.openai.com",
								Path:   "/v1/chat/completions",
							},
						},
					},
				},
			}

			// Create request with POST method
			req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions",
				bytes.NewBufferString(tt.requestBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			// Add context with client for RoundTrip
			ctx := context.WithValue(req.Context(), clientKey, transport.client)
			req = req.WithContext(ctx)

			// Execute RoundTrip
			resp, err := transport.RoundTrip(req)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q but got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if resp == nil {
				t.Fatal("Expected response but got nil")
			}

			// Check request modifications if specified
			if tt.checkRequest != nil && mockTransport.lastRequest != nil {
				tt.checkRequest(t, mockTransport.lastRequest)
			}
		})
	}
}

func TestUpdateRequestWithCombinedMessages(t *testing.T) {
	tests := []struct {
		name           string
		modelMessages  []model.Message
		messages       []model.Message
		extractedModel string
		initialBody    string
		wantErr        bool
		checkRequest   func(t *testing.T, req *http.Request)
	}{
		{
			name: "successfully combines messages",
			modelMessages: []model.Message{
				{"role": "system", "content": "You are a helpful assistant"},
			},
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			extractedModel: "gpt-4",
			initialBody:    `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			checkRequest: func(t *testing.T, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("Failed to read request body: %v", err)
				}

				var payload map[string]interface{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("Failed to unmarshal request body: %v", err)
				}

				messages, ok := payload["messages"].([]interface{})
				if !ok {
					t.Fatal("Messages field is not an array")
				}

				if len(messages) != 2 {
					t.Errorf("Expected 2 messages, got %d", len(messages))
				}

				model, ok := payload["model"].(string)
				if !ok || model != "gpt-4" {
					t.Errorf("Expected model to be 'gpt-4', got %v", model)
				}

				// Check content length was set correctly
				expectedLength := int64(len(body))
				if req.ContentLength != expectedLength {
					t.Errorf("Expected ContentLength %d, got %d", expectedLength, req.ContentLength)
				}
			},
		},
		{
			name:          "handles empty model messages",
			modelMessages: []model.Message{},
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			extractedModel: "gpt-4",
			initialBody:    `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			checkRequest: func(t *testing.T, req *http.Request) {
				body, _ := io.ReadAll(req.Body)
				var payload map[string]interface{}
				json.Unmarshal(body, &payload)

				messages, _ := payload["messages"].([]interface{})
				if len(messages) != 1 {
					t.Errorf("Expected 1 message, got %d", len(messages))
				}
			},
		},
		{
			name: "invalid message sequence",
			modelMessages: []model.Message{
				{"role": "assistant", "content": "Invalid first message"},
			},
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			extractedModel: "gpt-4",
			initialBody:    `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "http://example.com",
				bytes.NewBufferString(tt.initialBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			err = updateRequestWithCombinedMessages(req, tt.modelMessages, tt.messages, tt.extractedModel)

			if (err != nil) != tt.wantErr {
				t.Errorf("updateRequestWithCombinedMessages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.checkRequest != nil {
				tt.checkRequest(t, req)
			}
		})
	}
}
