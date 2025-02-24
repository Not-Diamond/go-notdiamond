package request

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

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

			got := ExtractMessagesFromRequest(req)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ExtractMessagesFromRequest() = %v, want %v", got, tt.expected)
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

			got := ExtractProviderFromRequest(req)
			if got != tt.expected {
				t.Errorf("ExtractProviderFromRequest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractModelFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected string
		wantErr  bool
	}{
		{
			name:     "valid model",
			payload:  []byte(`{"model": "gpt-4"}`),
			expected: "gpt-4",
			wantErr:  false,
		},
		{
			name:     "missing model field",
			payload:  []byte(`{"other": "field"}`),
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid json",
			payload:  []byte(`invalid json`),
			expected: "",
			wantErr:  true,
		},
		{
			name:     "model is not string",
			payload:  []byte(`{"model": 123}`),
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty payload",
			payload:  []byte{},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "http://example.com", bytes.NewBuffer(tt.payload))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			got, err := ExtractModelFromRequest(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractModelFromRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ExtractModelFromRequest() = %v, want %v", got, tt.expected)
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

func TestExtractOpenAIMessages(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected []model.Message
	}{
		{
			name: "valid single message",
			payload: []byte(`{
				"messages": [
					{"role": "user", "content": "Hello"}
				]
			}`),
			expected: []model.Message{
				{"role": "user", "content": "Hello"},
			},
		},
		{
			name: "valid multiple messages",
			payload: []byte(`{
				"messages": [
					{"role": "system", "content": "You are a helpful assistant"},
					{"role": "user", "content": "Hello"},
					{"role": "assistant", "content": "Hi there"}
				]
			}`),
			expected: []model.Message{
				{"role": "system", "content": "You are a helpful assistant"},
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": "Hi there"},
			},
		},
		{
			name: "empty messages array",
			payload: []byte(`{
				"messages": []
			}`),
			expected: []model.Message{},
		},
		{
			name:     "invalid JSON",
			payload:  []byte(`{invalid json}`),
			expected: nil,
		},
		{
			name: "missing messages field",
			payload: []byte(`{
				"model": "gpt-4",
				"temperature": 0.7
			}`),
			expected: nil,
		},
		{
			name: "null messages field",
			payload: []byte(`{
				"messages": null
			}`),
			expected: nil,
		},
		{
			name: "invalid message format",
			payload: []byte(`{
				"messages": [
					{"invalid": "format"}
				]
			}`),
			expected: []model.Message{
				{"invalid": "format"},
			},
		},
		{
			name:     "empty payload",
			payload:  []byte{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOpenAIMessages(tt.payload)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractOpenAIMessages() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractVertexMessages(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected []model.Message
	}{
		{
			name: "valid single message",
			payload: []byte(`{
				"contents": [
					{
						"role": "user",
						"parts": [
							{
								"text": "Hello"
							}
						]
					}
				]
			}`),
			expected: []model.Message{
				{"role": "user", "content": "Hello"},
			},
		},
		{
			name: "valid multiple messages",
			payload: []byte(`{
				"contents": [
					{
						"role": "user",
						"parts": [
							{
								"text": "Hello"
							}
						]
					},
					{
						"role": "model",
						"parts": [
							{
								"text": "Hi there"
							}
						]
					}
				]
			}`),
			expected: []model.Message{
				{"role": "user", "content": "Hello"},
				{"role": "model", "content": "Hi there"},
			},
		},
		{
			name: "empty parts array",
			payload: []byte(`{
				"contents": [
					{
						"role": "user",
						"parts": []
					}
				]
			}`),
			expected: []model.Message{},
		},
		{
			name:     "empty contents array",
			payload:  []byte(`{"contents": []}`),
			expected: []model.Message{},
		},
		{
			name:     "invalid json",
			payload:  []byte(`{invalid json}`),
			expected: nil,
		},
		{
			name:     "missing contents field",
			payload:  []byte(`{"other": "field"}`),
			expected: nil,
		},
		{
			name: "multiple parts (should take first)",
			payload: []byte(`{
				"contents": [
					{
						"role": "user",
						"parts": [
							{
								"text": "First message"
							},
							{
								"text": "Second message"
							}
						]
					}
				]
			}`),
			expected: []model.Message{
				{"role": "user", "content": "First message"},
			},
		},
		{
			name:     "empty payload",
			payload:  []byte{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVertexMessages(tt.payload)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractVertexMessages() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTransformToVertexRequest(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		model       string
		expected    string
		expectError bool
	}{
		{
			name: "basic transformation",
			payload: []byte(`{
				"messages": [
					{"role": "user", "content": "Hello"},
					{"role": "assistant", "content": "Hi there"}
				],
				"temperature": 0.8,
				"max_tokens": 100,
				"top_p": 0.9,
				"top_k": 50
			}`),
			model: "vertex/chat-bison",
			expected: `{
				"model": "chat-bison",
				"contents": [
					{
						"role": "user",
						"parts": [{"text": "Hello"}]
					},
					{
						"role": "model",
						"parts": [{"text": "Hi there"}]
					}
				],
				"generationConfig": {
					"temperature": 0.8,
					"maxOutputTokens": 100,
					"topP": 0.9,
					"topK": 50
				}
			}`,
			expectError: false,
		},
		{
			name: "with system message",
			payload: []byte(`{
				"messages": [
					{"role": "system", "content": "You are helpful"},
					{"role": "user", "content": "Hello"}
				]
			}`),
			model: "chat-bison",
			expected: `{
				"model": "chat-bison",
				"contents": [
					{
						"role": "user",
						"parts": [{"text": "You are helpful"}]
					},
					{
						"role": "user",
						"parts": [{"text": "Hello"}]
					}
				],
				"generationConfig": {
					"temperature": 0.7,
					"maxOutputTokens": 1024,
					"topP": 0.95,
					"topK": 40
				}
			}`,
			expectError: false,
		},
		{
			name: "with stop sequences",
			payload: []byte(`{
				"messages": [{"role": "user", "content": "Hello"}],
				"stop": ["END", "STOP"]
			}`),
			model: "chat-bison",
			expected: `{
				"model": "chat-bison",
				"contents": [
					{
						"role": "user",
						"parts": [{"text": "Hello"}]
					}
				],
				"generationConfig": {
					"temperature": 0.7,
					"maxOutputTokens": 1024,
					"topP": 0.95,
					"topK": 40
				},
				"stopSequences": ["END", "STOP"]
			}`,
			expectError: false,
		},
		{
			name: "with extra parameters",
			payload: []byte(`{
				"messages": [{"role": "user", "content": "Hello"}],
				"extra": {
					"custom_param": "value"
				}
			}`),
			model: "chat-bison",
			expected: `{
				"model": "chat-bison",
				"contents": [
					{
						"role": "user",
						"parts": [{"text": "Hello"}]
					}
				],
				"generationConfig": {
					"temperature": 0.7,
					"maxOutputTokens": 1024,
					"topP": 0.95,
					"topK": 40
				},
				"extra": {
					"custom_param": "value"
				}
			}`,
			expectError: false,
		},
		{
			name:        "invalid json",
			payload:     []byte(`{invalid json}`),
			model:       "chat-bison",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty payload",
			payload:     []byte{},
			model:       "chat-bison",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformToVertexRequest(tt.payload, tt.model)
			if (err != nil) != tt.expectError {
				t.Errorf("TransformToVertexRequest() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				// Normalize the JSON for comparison
				var gotJSON, expectedJSON map[string]interface{}
				if err := json.Unmarshal(got, &gotJSON); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}
				if err := json.Unmarshal([]byte(tt.expected), &expectedJSON); err != nil {
					t.Fatalf("Failed to unmarshal expected: %v", err)
				}

				if !reflect.DeepEqual(gotJSON, expectedJSON) {
					t.Errorf("TransformToVertexRequest() = %v, want %v", string(got), tt.expected)
				}
			}
		})
	}
}

func TestTransformFromVertexResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expected    string
		expectError bool
	}{
		{
			name: "valid response with single candidate",
			input: []byte(`{
				"candidates": [{
					"content": {
						"parts": [{"text": "Hello there"}],
						"role": "model"
					},
					"finishReason": "STOP",
					"safetyRatings": [
						{"category": "HARM", "probability": "LOW"}
					]
				}],
				"usageMetadata": {
					"promptTokenCount": 10,
					"candidatesTokenCount": 5,
					"totalTokenCount": 15
				}
			}`),
			expected: `{
				"choices": [{
					"index": 0,
					"message": {
						"role": "model",
						"content": "Hello there"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 5,
					"total_tokens": 15
				}
			}`,
			expectError: false,
		},
		{
			name: "multiple candidates",
			input: []byte(`{
				"candidates": [
					{
						"content": {
							"parts": [{"text": "Response 1"}],
							"role": "model"
						},
						"finishReason": "STOP"
					},
					{
						"content": {
							"parts": [{"text": "Response 2"}],
							"role": "model"
						},
						"finishReason": "LENGTH"
					}
				],
				"usageMetadata": {
					"promptTokenCount": 10,
					"candidatesTokenCount": 10,
					"totalTokenCount": 20
				}
			}`),
			expected: `{
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "model",
							"content": "Response 1"
						},
						"finish_reason": "stop"
					},
					{
						"index": 1,
						"message": {
							"role": "model",
							"content": "Response 2"
						},
						"finish_reason": "length"
					}
				],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 10,
					"total_tokens": 20
				}
			}`,
			expectError: false,
		},
		{
			name: "empty parts array",
			input: []byte(`{
				"candidates": [{
					"content": {
						"parts": [],
						"role": "model"
					},
					"finishReason": "STOP"
				}],
				"usageMetadata": {
					"promptTokenCount": 10,
					"candidatesTokenCount": 0,
					"totalTokenCount": 10
				}
			}`),
			expected: `{
				"choices": [],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 0,
					"total_tokens": 10
				}
			}`,
			expectError: false,
		},
		{
			name:        "invalid json",
			input:       []byte(`{invalid json}`),
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty input",
			input:       []byte{},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromVertexResponse(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("TransformFromVertexResponse() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				// Normalize the JSON for comparison
				var gotJSON, expectedJSON map[string]interface{}
				if err := json.Unmarshal(got, &gotJSON); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}
				if err := json.Unmarshal([]byte(tt.expected), &expectedJSON); err != nil {
					t.Fatalf("Failed to unmarshal expected: %v", err)
				}

				if !reflect.DeepEqual(gotJSON, expectedJSON) {
					t.Errorf("TransformFromVertexResponse() = %v, want %v", string(got), tt.expected)
				}
			}
		})
	}
}

func TestTransformFromVertexToOpenAI(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expected    string
		expectError bool
	}{
		{
			name: "valid single message",
			input: []byte(`{
				"contents": [
					{
						"role": "user",
						"parts": [{"text": "Hello"}]
					}
				],
				"generationConfig": {
					"temperature": 0.8,
					"maxOutputTokens": 100,
					"topP": 0.9,
					"topK": 40
				}
			}`),
			expected: `{
				"messages": [
					{"role": "user", "content": "Hello"}
				],
				"temperature": 0.8,
				"max_tokens": 100,
				"top_p": 0.9
			}`,
			expectError: false,
		},
		{
			name: "multiple messages with model role conversion",
			input: []byte(`{
				"contents": [
					{
						"role": "user",
						"parts": [{"text": "Hello"}]
					},
					{
						"role": "model",
						"parts": [{"text": "Hi there"}]
					}
				]
			}`),
			expected: `{
				"messages": [
					{"role": "user", "content": "Hello"},
					{"role": "assistant", "content": "Hi there"}
				]
			}`,
			expectError: false,
		},
		{
			name: "empty parts array should be skipped",
			input: []byte(`{
				"contents": [
					{
						"role": "user",
						"parts": []
					},
					{
						"role": "model",
						"parts": [{"text": "Response"}]
					}
				]
			}`),
			expected: `{
				"messages": [
					{"role": "assistant", "content": "Response"}
				]
			}`,
			expectError: false,
		},
		{
			name: "with default generation config",
			input: []byte(`{
				"contents": [
					{
						"role": "user",
						"parts": [{"text": "Hello"}]
					}
				],
				"generationConfig": {}
			}`),
			expected: `{
				"messages": [
					{"role": "user", "content": "Hello"}
				]
			}`,
			expectError: false,
		},
		{
			name:        "invalid json",
			input:       []byte(`{invalid json}`),
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty input",
			input:       []byte{},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromVertexToOpenAI(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("TransformFromVertexToOpenAI() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				// Normalize the JSON for comparison
				var gotJSON, expectedJSON map[string]interface{}
				if err := json.Unmarshal(got, &gotJSON); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}
				if err := json.Unmarshal([]byte(tt.expected), &expectedJSON); err != nil {
					t.Fatalf("Failed to unmarshal expected: %v", err)
				}

				if !reflect.DeepEqual(gotJSON, expectedJSON) {
					t.Errorf("TransformFromVertexToOpenAI() = %v, want %v", string(got), tt.expected)
				}
			}
		})
	}
}
