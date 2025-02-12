package notdiamond

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/http/request"
	"github.com/Not-Diamond/go-notdiamond/pkg/metric"
	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

func TestCombineMessages(t *testing.T) {
	tests := []struct {
		name          string
		modelMessages []model.Message
		userMessages  []model.Message
		expected      []model.Message
	}{
		{
			name: "both model and user messages",
			modelMessages: []model.Message{
				{"role": "system", "content": "You are a helpful assistant"},
			},
			userMessages: []model.Message{
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": "Hi there"},
			},
			expected: []model.Message{
				{"role": "system", "content": "You are a helpful assistant"},
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": "Hi there"},
			},
		},
		{
			name:          "empty model messages",
			modelMessages: []model.Message{},
			userMessages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			expected: []model.Message{
				{"role": "user", "content": "Hello"},
			},
		},
		{
			name: "empty user messages",
			modelMessages: []model.Message{
				{"role": "system", "content": "You are a helpful assistant"},
			},
			userMessages: []model.Message{},
			expected: []model.Message{
				{"role": "system", "content": "You are a helpful assistant"},
			},
		},
		{
			name:          "both empty messages",
			modelMessages: []model.Message{},
			userMessages:  []model.Message{},
			expected:      []model.Message{},
		},
		{
			name: "multiple model messages",
			modelMessages: []model.Message{
				{"role": "system", "content": "You are a helpful assistant"},
			},
			userMessages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			expected: []model.Message{
				{"role": "system", "content": "You are a helpful assistant"},
				{"role": "user", "content": "Hello"},
			},
		},
		{
			name: "user message system ignored if model message system exists",
			modelMessages: []model.Message{
				{"role": "system", "content": "You are a helpful assistant initial"},
			},
			userMessages: []model.Message{
				{"role": "system", "content": "You are a helpful assistant ignored"},
				{"role": "user", "content": "Hello"},
			},
			expected: []model.Message{
				{"role": "system", "content": "You are a helpful assistant initial"},
				{"role": "user", "content": "Hello"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := combineMessages(tt.modelMessages, tt.userMessages)
			if err != nil {
				t.Errorf("combineMessages() = %v, want %v", err, nil)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("combineMessages() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTryWithRetries(t *testing.T) {
	tests := []struct {
		name           string
		modelFull      string
		maxRetries     map[string]int
		timeout        map[string]float64
		backoff        map[string]float64
		modelMessages  map[string][]model.Message
		modelLatency   model.ModelLatency
		messages       []model.Message
		setupTransport func() *mockTransport
		expectedCalls  int
		expectError    bool
		errorContains  string
	}{
		{
			name:      "successful first attempt",
			modelFull: "openai/gpt-4",
			maxRetries: map[string]int{
				"openai/gpt-4": 3,
			},
			timeout: map[string]float64{
				"openai/gpt-4": 0.1,
			},
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			modelLatency: model.ModelLatency{
				"openai/gpt-4": &model.RollingAverageLatency{
					AvgLatencyThreshold: 3.5,
					NoOfCalls:           5,
					RecoveryTime:        100 * time.Millisecond,
				},
			},
			setupTransport: func() *mockTransport {
				return &mockTransport{
					responses: []*http.Response{
						{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString(`{"success": true}`)),
						},
					},
				}
			},
			expectedCalls: 1,
			expectError:   false,
		},
		{
			name:      "retry success on third attempt",
			modelFull: "openai/gpt-4",
			maxRetries: map[string]int{
				"openai/gpt-4": 3,
			},
			timeout: map[string]float64{
				"openai/gpt-4": 0.1,
			},
			backoff: map[string]float64{
				"openai/gpt-4": 0.01,
			},
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			modelLatency: model.ModelLatency{
				"openai/gpt-4": &model.RollingAverageLatency{
					AvgLatencyThreshold: 3.5,
					NoOfCalls:           5,
					RecoveryTime:        100 * time.Millisecond,
				},
			},
			setupTransport: func() *mockTransport {
				return &mockTransport{
					errors: []error{
						fmt.Errorf("network error"),
						fmt.Errorf("network error"),
					},
					responses: []*http.Response{
						nil,
						nil,
						{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString(`{"success": true}`)),
						},
					},
				}
			},
			expectedCalls: 3,
			expectError:   false,
		},
		{
			name:      "all attempts fail",
			modelFull: "openai/gpt-4",
			maxRetries: map[string]int{
				"openai/gpt-4": 2,
			},
			timeout: map[string]float64{
				"openai/gpt-4": 0.1,
			},
			backoff: map[string]float64{
				"openai/gpt-4": 0.01,
			},
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			modelLatency: model.ModelLatency{
				"openai/gpt-4": &model.RollingAverageLatency{
					AvgLatencyThreshold: 3.5,
					NoOfCalls:           5,
					RecoveryTime:        100 * time.Millisecond,
				},
			},
			setupTransport: func() *mockTransport {
				return &mockTransport{
					errors: []error{
						fmt.Errorf("persistent error"),
						fmt.Errorf("persistent error"),
					},
				}
			},
			expectedCalls: 2,
			expectError:   true,
			errorContains: "persistent error",
		},
		{
			name:      "non-200 status code",
			modelFull: "openai/gpt-4",
			maxRetries: map[string]int{
				"openai/gpt-4": 1,
			},
			timeout: map[string]float64{
				"openai/gpt-4": 0.1,
			},
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			modelLatency: model.ModelLatency{
				"openai/gpt-4": &model.RollingAverageLatency{
					AvgLatencyThreshold: 3.5,
					NoOfCalls:           5,
					RecoveryTime:        100 * time.Millisecond,
				},
			},
			setupTransport: func() *mockTransport {
				return &mockTransport{
					responses: []*http.Response{
						{
							StatusCode: 429,
							Body:       io.NopCloser(bytes.NewBufferString(`{"error": "rate limit"}`)),
						},
					},
				}
			},
			expectedCalls: 1,
			expectError:   true,
			errorContains: "429",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := tt.setupTransport()
			req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`))
			metrics, err := metric.NewTracker(":memory:" + tt.name)
			if err != nil {
				log.Fatalf("Failed to open database connection: %v", err)
			}
			httpClient := &NotDiamondHttpClient{
				Client: &http.Client{Transport: transport},
				config: model.Config{
					MaxRetries:    tt.maxRetries,
					Timeout:       tt.timeout,
					Backoff:       tt.backoff,
					ModelMessages: tt.modelMessages,
					ModelLatency:  tt.modelLatency,
				},
				metricsTracker: metrics,
			}

			ctx := context.WithValue(context.Background(), clientKey, &Client{
				clients:    []http.Request{*req},
				HttpClient: httpClient,
			})

			resp, err := httpClient.tryWithRetries(tt.modelFull, req, tt.messages, ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q but got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("expected response but got nil")
				}
			}

			if transport.callCount != tt.expectedCalls {
				t.Errorf("expected %d calls but got %d", tt.expectedCalls, transport.callCount)
			}
		})
	}
}

func TestGetWeightedModelsList(t *testing.T) {
	tests := []struct {
		name    string
		weights map[string]float64
		want    []string
	}{
		{
			name: "two models",
			weights: map[string]float64{
				"openai/gpt-4": 0.6,
				"azure/gpt-4":  0.4,
			},
			want: []string{"openai/gpt-4", "azure/gpt-4"},
		},
		{
			name: "three models",
			weights: map[string]float64{
				"openai/gpt-4":       0.6,
				"azure/gpt-4":        0.4,
				"openai/gpt-4o-mini": 0.2,
			},
			want: []string{"openai/gpt-4", "azure/gpt-4", "openai/gpt-4o-mini"},
		},
		{
			name:    "empty map",
			weights: map[string]float64{},
			want:    []string{},
		},
		{
			name: "single model",
			weights: map[string]float64{
				"openai/gpt-4": 1.0,
			},
			want: []string{"openai/gpt-4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getWeightedModelsList(tt.weights)

			sort.Strings(got)
			sort.Strings(tt.want)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getWeightedModelsList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTryNextModel(t *testing.T) {
	tests := []struct {
		name          string
		modelFull     string
		messages      []model.Message
		setupClient   func() (*Client, *mockTransport)
		expectedURL   string
		expectedBody  map[string]interface{}
		expectedError string
		mockResponse  *http.Response
		mockError     error
		checkRequest  func(t *testing.T, req *http.Request)
	}{
		{
			name:      "successful azure request",
			modelFull: "azure/gpt-4",
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			setupClient: func() (*Client, *mockTransport) {
				req, _ := http.NewRequest("POST", "https://myresource.azure.openai.com", nil)
				req.Header.Set("api-key", "test-key")
				transport := &mockTransport{
					responses: []*http.Response{
						{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"Hello"}}]}`)),
						},
					},
				}

				metrics, err := metric.NewTracker(":memory:" + "successful_azure_request")
				if err != nil {
					log.Fatalf("Failed to open database connection: %v", err)
				}
				return &Client{
					clients: []http.Request{*req},
					HttpClient: &NotDiamondHttpClient{
						Client: &http.Client{
							Transport: transport,
						},
						config: model.Config{
							ModelMessages: map[string][]model.Message{
								"azure/gpt-4": {
									{"role": "system", "content": "Hello"},
								},
							},
						},
						metricsTracker: metrics,
					},
				}, transport
			},
			expectedURL: "https://myresource.azure.openai.com/openai/deployments/gpt-4/chat/completions?api-version=2023-05-15",
			checkRequest: func(t *testing.T, req *http.Request) {
				if req.Header.Get("api-key") != "test-key" {
					t.Errorf("Expected api-key header to be 'test-key', got %q", req.Header.Get("api-key"))
				}
				if req.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type header to be 'application/json', got %q", req.Header.Get("Content-Type"))
				}
				if req.URL.String() != "https://myresource.azure.openai.com/openai/deployments/gpt-4/chat/completions?api-version=2023-05-15" {
					t.Errorf("Expected URL %q, got %q", "https://myresource.azure.openai.com/openai/deployments/gpt-4/chat/completions?api-version=2023-05-15", req.URL.String())
				}
			},
		},
		{
			name:      "successful openai request",
			modelFull: "openai/gpt-4",
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			setupClient: func() (*Client, *mockTransport) {
				req, _ := http.NewRequest("POST", "https://api.openai.com", nil)
				req.Header.Set("api-key", "test-key")
				transport := &mockTransport{
					responses: []*http.Response{
						{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"Hello"}}]}`)),
						},
					},
				}
				metrics, err := metric.NewTracker(":memory:" + "successful_openai_request")
				if err != nil {
					log.Fatalf("Failed to open database connection: %v", err)
				}
				return &Client{
					clients: []http.Request{*req},
					HttpClient: &NotDiamondHttpClient{
						Client: &http.Client{
							Transport: transport,
						},
						config: model.Config{
							ModelMessages: map[string][]model.Message{
								"openai/gpt-4": {
									{"role": "system", "content": "Hello"},
								},
							},
						},
						metricsTracker: metrics,
					},
				}, transport
			},
			checkRequest: func(t *testing.T, req *http.Request) {
				if req.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("Expected Authorization header to be 'Bearer test-key', got %q", req.Header.Get("Authorization"))
				}
				if req.Header.Get("api-key") != "" {
					t.Errorf("Expected api-key header to be empty, got %q", req.Header.Get("api-key"))
				}
				if req.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type header to be 'application/json', got %q", req.Header.Get("Content-Type"))
				}
			},
		},
		{
			name:      "provider not found",
			modelFull: "unknown/gpt-4",
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			setupClient: func() (*Client, *mockTransport) {
				req, _ := http.NewRequest("POST", "https://api.openai.com", nil)
				transport := &mockTransport{}
				metrics, err := metric.NewTracker(":memory:" + "provider_not_found")
				if err != nil {
					log.Fatalf("Failed to open database connection: %v", err)
				}

				return &Client{
					clients: []http.Request{*req},
					HttpClient: &NotDiamondHttpClient{
						Client: &http.Client{
							Transport: transport,
						},
						config: model.Config{
							ModelMessages: map[string][]model.Message{
								"unknown/gpt-4": {
									{"role": "user", "content": "Hello"},
								},
							},
						},
						metricsTracker: metrics,
					},
				}, transport
			},
			expectedError: "no client found for provider unknown",
		},
		{
			name:      "http client error",
			modelFull: "openai/gpt-4",
			messages: []model.Message{
				{"role": "user", "content": "Hello"},
			},
			setupClient: func() (*Client, *mockTransport) {
				req, _ := http.NewRequest("POST", "https://api.openai.com", nil)
				req.Header.Set("api-key", "test-key")
				transport := &mockTransport{
					errors: []error{fmt.Errorf("network error")},
				}
				metrics, err := metric.NewTracker(":memory:" + "http_client_error")
				if err != nil {
					log.Fatalf("Failed to open database connection: %v", err)
				}
				return &Client{
					clients: []http.Request{*req},
					HttpClient: &NotDiamondHttpClient{
						Client: &http.Client{
							Transport: transport,
						},
						config: model.Config{
							ModelMessages: map[string][]model.Message{
								"openai/gpt-4": {
									{"role": "system", "content": "Hello"},
								},
							},
						},
						metricsTracker: metrics,
					},
				}, transport
			},
			expectedError: "network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := tt.setupClient()
			ctx := context.Background()

			resp, err := tryNextModel(client, tt.modelFull, tt.messages, ctx)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing %q but got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q but got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if resp == nil {
				t.Fatal("Expected response but got nil")
			}

			if tt.checkRequest != nil && transport.lastRequest != nil {
				tt.checkRequest(t, transport.lastRequest)
			}
		})
	}
}

func TestExtractMessagesFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected []model.Message
	}{
		{
			name: "valid messages",
			payload: []byte(`{
				"messages": [
					{"role": "user", "content": "Hello"},
					{"role": "assistant", "content": "Hi there"}
				]
			}`),
			expected: []model.Message{
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": "Hi there"},
			},
		},
		{
			name:     "empty messages array",
			payload:  []byte(`{"messages": []}`),
			expected: []model.Message{},
		},
		{
			name:     "invalid json",
			payload:  []byte(`{invalid json}`),
			expected: nil,
		},
		{
			name:     "missing messages field",
			payload:  []byte(`{"other": "field"}`),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "http://example.com", bytes.NewBuffer(tt.payload))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			got := request.ExtractMessagesFromRequest(req)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractMessagesFromRequest() = %v, want %v", got, tt.expected)
			}

			body := make([]byte, len(tt.payload))
			n, err := req.Body.Read(body)
			if err != nil && err.Error() != "EOF" {
				t.Errorf("Failed to read request body after extraction: %v", err)
			}
			if n != len(tt.payload) {
				t.Errorf("Request body length after extraction = %d, want %d", n, len(tt.payload))
			}
		})
	}
}

func TestExtractProviderFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "OpenAI URL",
			url:      "https://api.openai.com/v1/chat/completions",
			expected: "openai",
		},
		{
			name:     "Azure URL",
			url:      "https://myresource.azure.openai.com/openai/deployments/gpt-4/chat/completions",
			expected: "azure",
		},
		{
			name:     "Invalid URL",
			url:      "https://api.example.com/v1/chat/completions",
			expected: "",
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", tt.url, nil)
			if err != nil && tt.url != "" {
				t.Fatalf("Failed to create request: %v", err)
			}

			got := request.ExtractProviderFromRequest(req)
			if got != tt.expected {
				t.Errorf("extractProviderFromRequest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractModelFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected string
	}{
		{
			name: "valid model",
			payload: []byte(`{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "Hello"}]
			}`),
			expected: "gpt-4",
		},
		{
			name: "missing model field",
			payload: []byte(`{
				"messages": [{"role": "user", "content": "Hello"}]
			}`),
			expected: "",
		},
		{
			name:     "invalid json",
			payload:  []byte(`{invalid json}`),
			expected: "",
		},
		{
			name: "model is not string",
			payload: []byte(`{
				"model": 123,
				"messages": [{"role": "user", "content": "Hello"}]
			}`),
			expected: "",
		},
		{
			name:     "empty payload",
			payload:  []byte{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "http://example.com", bytes.NewBuffer(tt.payload))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			got := request.ExtractModelFromRequest(req)
			if got != tt.expected {
				t.Errorf("extractModelFromRequest() = %v, want %v", got, tt.expected)
			}

			// Verify that the request body can still be read after extraction
			body := make([]byte, len(tt.payload))
			n, err := req.Body.Read(body)
			if err != nil && err.Error() != "EOF" {
				t.Errorf("Failed to read request body after extraction: %v", err)
			}
			if n != len(tt.payload) {
				t.Errorf("Request body length after extraction = %d, want %d", n, len(tt.payload))
			}
		})
	}
}

func TestGetMaxRetriesForStatus(t *testing.T) {
	tests := []struct {
		name            string
		modelFull       string
		statusCode      int
		maxRetries      map[string]int
		statusCodeRetry interface{}
		expected        int
	}{
		{
			name:       "model specific status code retry",
			modelFull:  "openai/gpt-4",
			statusCode: 429,
			statusCodeRetry: map[string]map[string]int{
				"openai/gpt-4": {
					"429": 5,
				},
			},
			maxRetries: map[string]int{
				"openai/gpt-4": 3,
			},
			expected: 5,
		},
		{
			name:       "global status code retry",
			modelFull:  "openai/gpt-4",
			statusCode: 429,
			statusCodeRetry: map[string]int{
				"429": 4,
			},
			maxRetries: map[string]int{
				"openai/gpt-4": 3,
			},
			expected: 4,
		},
		{
			name:       "fallback to model max retries",
			modelFull:  "openai/gpt-4",
			statusCode: 429,
			statusCodeRetry: map[string]int{
				"500": 5,
			},
			maxRetries: map[string]int{
				"openai/gpt-4": 3,
			},
			expected: 3,
		},
		{
			name:            "default to 1 when no config exists",
			modelFull:       "openai/gpt-4",
			statusCode:      429,
			statusCodeRetry: map[string]int{},
			maxRetries:      map[string]int{},
			expected:        1,
		},
		{
			name:       "model specific takes precedence over global",
			modelFull:  "openai/gpt-4",
			statusCode: 429,
			statusCodeRetry: map[string]map[string]int{
				"openai/gpt-4": {
					"429": 5,
				},
			},
			maxRetries: map[string]int{
				"openai/gpt-4": 3,
			},
			expected: 5,
		},
		{
			name:       "different status code in model specific",
			modelFull:  "openai/gpt-4",
			statusCode: 429,
			statusCodeRetry: map[string]map[string]int{
				"openai/gpt-4": {
					"500": 5,
				},
			},
			maxRetries: map[string]int{
				"openai/gpt-4": 3,
			},
			expected: 3,
		},
		{
			name:       "different model in model specific",
			modelFull:  "openai/gpt-4",
			statusCode: 429,
			statusCodeRetry: map[string]map[string]int{
				"azure/gpt-4": {
					"429": 5,
				},
			},
			maxRetries: map[string]int{
				"openai/gpt-4": 3,
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &NotDiamondHttpClient{
				config: model.Config{
					MaxRetries:      tt.maxRetries,
					StatusCodeRetry: tt.statusCodeRetry,
				},
			}

			got := client.getMaxRetriesForStatus(tt.modelFull, tt.statusCode)
			if got != tt.expected {
				t.Errorf("getMaxRetriesForStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDo(t *testing.T) {
	tests := []struct {
		name          string
		setupClient   func() (*NotDiamondHttpClient, *mockTransport)
		expectedCalls int
		expectError   bool
		errorContains string
	}{
		{
			name: "successful first attempt with ordered models",
			setupClient: func() (*NotDiamondHttpClient, *mockTransport) {
				transport := &mockTransport{
					responses: []*http.Response{
						{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString(`{"success": true}`)),
						},
					},
				}
				metrics, err := metric.NewTracker(":memory:" + "successful first attempt with ordered models")
				if err != nil {
					log.Fatalf("Failed to open database connection: %v", err)
				}
				client := &NotDiamondHttpClient{
					Client: &http.Client{Transport: transport},
					config: model.Config{
						MaxRetries: map[string]int{"openai/gpt-4": 3},
						Timeout:    map[string]float64{"openai/gpt-4": 30.0},
						ModelLatency: model.ModelLatency{
							"openai/gpt-4": &model.RollingAverageLatency{
								AvgLatencyThreshold: 3.5,
								NoOfCalls:           5,               // Max 10
								RecoveryTime:        5 * time.Minute, // Max 1h
							},
						},
					},
					metricsTracker: metrics,
				}

				return client, transport
			},
			expectedCalls: 1,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := tt.setupClient()

			// Create a request with the necessary context
			req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions",
				bytes.NewBufferString(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`))

			// Create NotDiamondClient and add it to context
			notDiamondClient := &Client{
				HttpClient: client,
				models:     model.OrderedModels{"openai/gpt-4", "azure/gpt-4"},
				isOrdered:  true,
			}
			ctx := context.WithValue(context.Background(), clientKey, notDiamondClient)
			req = req.WithContext(ctx)

			// Make the request
			resp, err := client.Do(req)

			// Verify error expectations
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q but got %q", tt.errorContains, err.Error())
				}
				return
			}

			// Verify success case
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp == nil {
				t.Error("expected response but got nil")
				return
			}

			// Verify number of calls made
			if transport.callCount != tt.expectedCalls {
				t.Errorf("expected %d calls but got %d", tt.expectedCalls, transport.callCount)
			}
		})
	}
}

func TestDoWithLatencies(t *testing.T) {
	tests := []struct {
		name          string
		setupClient   func() (*NotDiamondHttpClient, *mockTransport)
		expectedCalls int
		expectError   bool
		errorContains string
	}{
		{
			name: "successful first attempt with ordered models",
			setupClient: func() (*NotDiamondHttpClient, *mockTransport) {
				// In this scenario, the transport delay is 0.5 seconds.
				// Since the number of calls required is 5 (window size),
				// and only one record is recorded (insufficient to compute moving average),
				// the model is assumed healthy.
				transport := &mockTransport{
					responses: []*http.Response{
						{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString(`{"success": true}`)),
						},
					},
					// 0.5-second delay (recorded latency ~0.5 seconds)
					delay: 500 * time.Millisecond,
				}
				metrics, err := metric.NewTracker(":memory:" + "successful_first_attempt")
				if err != nil {
					log.Fatalf("Failed to open database connection: %v", err)
				}
				client := &NotDiamondHttpClient{
					Client: &http.Client{Transport: transport},
					config: model.Config{
						// Using a window size of 5, so with one record (insufficient) the model is healthy.
						ModelLatency: model.ModelLatency{
							"openai/gpt-4": &model.RollingAverageLatency{
								AvgLatencyThreshold: 3.5,
								NoOfCalls:           5,                      // window size
								RecoveryTime:        100 * time.Millisecond, // recovery period
							},
							"azure/gpt-4": &model.RollingAverageLatency{
								AvgLatencyThreshold: 3.5,
								NoOfCalls:           5,                      // window size
								RecoveryTime:        100 * time.Millisecond, // recovery period
							},
						},
					},
					metricsTracker: metrics,
				}
				return client, transport
			},
			expectedCalls: 1,
			expectError:   false,
		},
		{
			name: "latency delay without recovery (model unhealthy)",
			setupClient: func() (*NotDiamondHttpClient, *mockTransport) {
				// In this case, we set the window size to 1 so that the single measured latency (~0.6 seconds)
				// immediately becomes the moving average. Because 0.6 > threshold (0.35)
				// and the record's timestamp is current (i.e. within the RecoveryTime),
				// the model should be considered unhealthy and tryWithRetries will return an error
				// without making an HTTP call.
				transport := &mockTransport{
					// No response will be used because the unhealthy check fails before sending.
					delay: 600 * time.Millisecond,
				}
				metrics, err := metric.NewTracker(":memory:" + "latency_delay_without_recovery")
				if err != nil {
					log.Fatalf("Failed to open database connection: %v", err)
				}
				client := &NotDiamondHttpClient{
					Client: &http.Client{Transport: transport},
					config: model.Config{
						// Set window size to 1 so that the measured latency is used immediately.
						ModelLatency: model.ModelLatency{
							"openai/gpt-4": &model.RollingAverageLatency{
								AvgLatencyThreshold: 0.35,
								NoOfCalls:           1,                      // immediate decision
								RecoveryTime:        100 * time.Millisecond, // recovery period
							},
							"azure/gpt-4": &model.RollingAverageLatency{
								AvgLatencyThreshold: 0.35,
								NoOfCalls:           1,                      // immediate decision
								RecoveryTime:        100 * time.Millisecond, // recovery period
							},
						},
					},
					metricsTracker: metrics,
				}
				return client, transport
			},
			expectedCalls: 0,
			expectError:   true,
			errorContains: "azure",
		},
		{
			name: "latency delay with recovery (model healthy)",
			setupClient: func() (*NotDiamondHttpClient, *mockTransport) {
				transport := &mockTransport{
					responses: []*http.Response{
						{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBufferString(`{"success": true}`)),
						},
					},
					delay: 500 * time.Millisecond,
				}
				metrics, err := metric.NewTracker(":memory:" + "latency_delay_with_recovery")
				if err != nil {
					log.Fatalf("Failed to open database connection: %v", err)
				}
				client := &NotDiamondHttpClient{
					Client: &http.Client{Transport: transport},
					config: model.Config{
						ModelLatency: model.ModelLatency{
							"openai/gpt-4": &model.RollingAverageLatency{
								AvgLatencyThreshold: 3.5,
								NoOfCalls:           5,                      // window size
								RecoveryTime:        100 * time.Millisecond, // recovery period
							},
							"azure/gpt-4": &model.RollingAverageLatency{
								AvgLatencyThreshold: 3.5,
								NoOfCalls:           5,                      // window size
								RecoveryTime:        100 * time.Millisecond, // recovery period
							},
						},
					},
					metricsTracker: metrics,
				}
				return client, transport
			},
			expectedCalls: 1,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := tt.setupClient()

			// Create a request with the necessary context.
			req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions",
				bytes.NewBufferString(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`))
			// Create a NotDiamondClient and add it to the context.
			notDiamondClient := &Client{
				HttpClient: client,
				models:     model.OrderedModels{"openai/gpt-4", "azure/gpt-4"},
				isOrdered:  true,
			}
			ctx := context.WithValue(context.Background(), clientKey, notDiamondClient)
			req = req.WithContext(ctx)

			// Make the request.
			resp, err := client.Do(req)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q but got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp == nil {
				t.Error("expected response but got nil")
				return
			}

			if transport.callCount != tt.expectedCalls {
				t.Errorf("expected %d calls but got %d", tt.expectedCalls, transport.callCount)
			}
		})
	}
}

type mockTransport struct {
	responses   []*http.Response
	errors      []error
	lastRequest *http.Request
	callCount   int
	currentIdx  int
	delay       time.Duration
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Introduce an artificial delay.
	time.Sleep(m.delay)

	m.lastRequest = req
	m.callCount++

	if m.currentIdx < len(m.responses) || m.currentIdx < len(m.errors) {
		resp := (*http.Response)(nil)
		if m.currentIdx < len(m.responses) {
			resp = m.responses[m.currentIdx]
		}

		err := error(nil)
		if m.currentIdx < len(m.errors) {
			err = m.errors[m.currentIdx]
		}

		m.currentIdx++
		return resp, err
	}

	// Default case: return the last configured response/error
	if len(m.responses) > 0 {
		return m.responses[len(m.responses)-1], nil
	}
	if len(m.errors) > 0 {
		return nil, m.errors[len(m.errors)-1]
	}
	return nil, fmt.Errorf("no response configured")
}
