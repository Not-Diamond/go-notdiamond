package request

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

// ExtractModelFromRequest extracts the model from the request body.
func ExtractModelFromRequest(req *http.Request) string {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return ""
	}

	req.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload map[string]interface{}
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return ""
	}

	if model, ok := payload["model"].(string); ok {
		return model
	}
	return ""
}

// ExtractProviderFromRequest extracts the provider from the request URL.
func ExtractProviderFromRequest(req *http.Request) string {
	url := req.URL.String()
	if strings.Contains(url, "azure") {
		return "azure"
	} else if strings.Contains(url, "openai.com") {
		return "openai"
	}
	return ""
}

// ExtractMessagesFromRequest extracts the messages from the request body.
func ExtractMessagesFromRequest(req *http.Request) []model.Message {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil
	}

	req.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload struct {
		Messages []model.Message `json:"messages"`
	}
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return nil
	}
	return payload.Messages
}
