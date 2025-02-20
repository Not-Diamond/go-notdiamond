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

	if modelStr, ok := payload["model"].(string); ok {
		// If it's in provider/model format, extract just the model part
		parts := strings.Split(modelStr, "/")
		if len(parts) == 2 {
			return modelStr // Return the full string for provider extraction
		}
		return modelStr
	}
	return ""
}

// ExtractProviderFromRequest extracts the provider from the request URL or model name.
func ExtractProviderFromRequest(req *http.Request) string {
	// First try to extract from URL
	url := req.URL.String()
	if strings.Contains(url, "azure") {
		return "azure"
	} else if strings.Contains(url, "openai.com") {
		return "openai"
	} else if strings.Contains(url, "aiplatform.googleapis.com") || strings.Contains(url, "-aiplatform.googleapis.com") {
		return "vertex"
	}

	// If not found in URL, try to extract from model name
	model := ExtractModelFromRequest(req)
	if strings.HasPrefix(model, "vertex/") {
		return "vertex"
	} else if strings.HasPrefix(model, "azure/") {
		return "azure"
	} else if strings.HasPrefix(model, "openai/") {
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

	provider := ExtractProviderFromRequest(req)
	switch provider {
	case "vertex":
		return extractVertexMessages(body)
	default:
		return extractOpenAIMessages(body)
	}
}

// extractOpenAIMessages extracts messages from OpenAI/Azure format
func extractOpenAIMessages(body []byte) []model.Message {
	var payload struct {
		Messages []model.Message `json:"messages"`
	}
	err := json.Unmarshal(body, &payload)
	if err != nil {
		return nil
	}
	return payload.Messages
}

// extractVertexMessages extracts messages from Vertex AI format
func extractVertexMessages(body []byte) []model.Message {
	var payload struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
	}
	err := json.Unmarshal(body, &payload)
	if err != nil {
		return nil
	}

	var messages []model.Message
	for _, content := range payload.Contents {
		if len(content.Parts) > 0 {
			messages = append(messages, model.Message{
				"role":    content.Role,
				"content": content.Parts[0].Text,
			})
		}
	}
	return messages
}

// TransformToVertexRequest transforms OpenAI format to Vertex AI format
func TransformToVertexRequest(body []byte, model string) ([]byte, error) {
	var openAIPayload struct {
		Messages    []map[string]string    `json:"messages"`
		Temperature float64                `json:"temperature"`
		MaxTokens   int                    `json:"max_tokens"`
		TopP        float64                `json:"top_p"`
		TopK        int                    `json:"top_k"`
		Stream      bool                   `json:"stream"`
		Stop        []string               `json:"stop"`
		Extra       map[string]interface{} `json:"extra,omitempty"`
	}

	if err := json.Unmarshal(body, &openAIPayload); err != nil {
		return nil, err
	}

	vertexPayload := map[string]interface{}{
		"contents": make([]map[string]interface{}, 0, len(openAIPayload.Messages)),
		"generationConfig": map[string]interface{}{
			"temperature":     openAIPayload.Temperature,
			"maxOutputTokens": openAIPayload.MaxTokens,
			"topP":            openAIPayload.TopP,
			"topK":            openAIPayload.TopK,
		},
	}

	for _, msg := range openAIPayload.Messages {
		vertexPayload["contents"] = append(vertexPayload["contents"].([]map[string]interface{}), map[string]interface{}{
			"role": msg["role"],
			"parts": []map[string]interface{}{
				{
					"text": msg["content"],
				},
			},
		})
	}

	if openAIPayload.Stream {
		vertexPayload["stream"] = true
	}

	if len(openAIPayload.Stop) > 0 {
		vertexPayload["stopSequences"] = openAIPayload.Stop
	}

	// Copy any extra parameters
	for k, v := range openAIPayload.Extra {
		if _, exists := vertexPayload[k]; !exists {
			vertexPayload[k] = v
		}
	}

	return json.Marshal(vertexPayload)
}

// TransformFromVertexResponse transforms Vertex AI response to OpenAI format
func TransformFromVertexResponse(body []byte) ([]byte, error) {
	var vertexResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
				Role string `json:"role"`
			} `json:"content"`
			FinishReason  string `json:"finishReason"`
			SafetyRatings []struct {
				Category    string `json:"category"`
				Probability string `json:"probability"`
			} `json:"safetyRatings"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(body, &vertexResponse); err != nil {
		return nil, err
	}

	openAIResponse := map[string]interface{}{
		"choices": make([]map[string]interface{}, 0, len(vertexResponse.Candidates)),
		"usage": map[string]interface{}{
			"prompt_tokens":     vertexResponse.UsageMetadata.PromptTokenCount,
			"completion_tokens": vertexResponse.UsageMetadata.CandidatesTokenCount,
			"total_tokens":      vertexResponse.UsageMetadata.TotalTokenCount,
		},
	}

	for i, candidate := range vertexResponse.Candidates {
		if len(candidate.Content.Parts) > 0 {
			choice := map[string]interface{}{
				"index": i,
				"message": map[string]interface{}{
					"role":    candidate.Content.Role,
					"content": candidate.Content.Parts[0].Text,
				},
				"finish_reason": strings.ToLower(candidate.FinishReason),
			}
			openAIResponse["choices"] = append(openAIResponse["choices"].([]map[string]interface{}), choice)
		}
	}

	return json.Marshal(openAIResponse)
}
