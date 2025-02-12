package request

import (
	"bytes"
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

			got := ExtractModelFromRequest(req)
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
