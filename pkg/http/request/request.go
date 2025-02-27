package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Not-Diamond/go-notdiamond/pkg/model"
)

// ExtractModelFromRequest extracts the model from the request body.
func ExtractModelFromRequest(req *http.Request) (string, error) {
	if req == nil {
		return "", fmt.Errorf("request is nil")
	}

	if req.Body == nil {
		return "", fmt.Errorf("request body is nil")
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	// Always restore the body for future reads
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	// Handle empty body
	if len(body) == 0 {
		return "", fmt.Errorf("empty request body")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("failed to unmarshal body: %w", err)
	}

	modelStr, ok := payload["model"].(string)
	if !ok {
		return "", fmt.Errorf("model field not found or not a string")
	}

	// Check if the model string contains a region (format: model/region)
	parts := strings.Split(modelStr, "/")

	// If it's in provider/model format, extract just the model part
	if len(parts) == 2 && (parts[0] == "openai" || parts[0] == "azure" || parts[0] == "vertex") {
		return parts[1], nil // Return just the model part
	}

	// If it's in model/region format, keep it as is to preserve the region information
	if len(parts) == 2 && parts[0] != "openai" && parts[0] != "azure" && parts[0] != "vertex" {
		return modelStr, nil // Return model/region
	}

	// If it's in provider/model/region format, extract model/region
	if len(parts) == 3 {
		return parts[1] + "/" + parts[2], nil // Return model/region
	}

	return modelStr, nil
}

// ExtractProviderFromRequest extracts the provider from the request URL or model name.
func ExtractProviderFromRequest(req *http.Request) string {
	// First try to extract from URL
	url := req.URL.String()
	if strings.Contains(url, "azure") {
		return "azure"
	} else if strings.Contains(url, "openai.com") {
		return "openai"
	} else if strings.Contains(url, "aiplatform.googleapis.com") ||
		strings.Contains(url, "-aiplatform.googleapis.com") ||
		strings.Contains(url, "aiplatform.googleapiss.com") ||
		strings.Contains(url, "-aiplatform.googleapiss.com") {
		return "vertex"
	} else if strings.Contains(url, "bedrock-runtime") ||
		strings.Contains(url, "amazonaws.com/model") {
		return "bedrock"
	}

	// If not found in URL, try to extract from model name in the request body
	if req.Body == nil {
		return ""
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		slog.Error("❌ Error reading request body", "error", err)
		return ""
	}

	// Restore the body for future reads
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("❌ Error unmarshaling request body", "error", err)
		return ""
	}

	modelStr, ok := payload["model"].(string)
	if !ok {
		slog.Error("❌ Model field not found or not a string")
		return ""
	}

	// Check if the model string contains a provider
	parts := strings.Split(modelStr, "/")

	// If it's in provider/model format or provider/model/region format
	if len(parts) >= 2 {
		provider := parts[0]
		if provider == "vertex" || provider == "azure" || provider == "openai" || provider == "bedrock" {
			return provider
		}
	}

	return ""
}

// ExtractMessagesFromRequest extracts the messages from the request body.
func ExtractMessagesFromRequest(req *http.Request) []model.Message {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		slog.Error("❌ Failed to read body in ExtractMessagesFromRequest", "error", err)
		return nil
	}

	req.Body = io.NopCloser(bytes.NewBuffer(body))

	provider := ExtractProviderFromRequest(req)
	switch provider {
	case "vertex":
		return extractVertexMessages(body)
	case "bedrock":
		// For Bedrock, we need to extract from vendor-specific formats
		// Extract model name to determine the vendor
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			slog.Error("❌ Failed to unmarshal Bedrock payload", "error", err)
			return nil
		}

		// Try to determine vendor from model field
		modelStr, ok := payload["model"].(string)
		if !ok {
			// If no model field, try to infer from payload structure
			if _, hasPrompt := payload["prompt"]; hasPrompt {
				// Likely Anthropic format
				return extractBedrockAnthropicMessages(body)
			} else if _, hasInputText := payload["inputText"]; hasInputText {
				// Likely Amazon Titan format
				return extractBedrockTitanMessages(body)
			}
			// Default to OpenAI-like format
			return extractOpenAIMessages(body)
		}

		// If model field exists, determine vendor from model name
		if strings.HasPrefix(modelStr, "anthropic.") || strings.Contains(modelStr, "claude") {
			return extractBedrockAnthropicMessages(body)
		} else if strings.HasPrefix(modelStr, "amazon.") || strings.Contains(modelStr, "titan") {
			return extractBedrockTitanMessages(body)
		}
		// Default to OpenAI-like format for unknown model types
		return extractOpenAIMessages(body)
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

	// Check if contents field exists in the JSON
	var rawPayload map[string]interface{}
	if err := json.Unmarshal(body, &rawPayload); err != nil {
		return nil
	}
	if _, exists := rawPayload["contents"]; !exists {
		return nil
	}

	messages := make([]model.Message, 0)
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

// extractBedrockAnthropicMessages extracts messages from Bedrock Anthropic Claude format
func extractBedrockAnthropicMessages(body []byte) []model.Message {
	var payload struct {
		Prompt string `json:"prompt"`
		System string `json:"system"`
	}

	err := json.Unmarshal(body, &payload)
	if err != nil {
		return nil
	}

	messages := make([]model.Message, 0)

	// Add system message if present
	if payload.System != "" {
		messages = append(messages, model.Message{
			"role":    "system",
			"content": payload.System,
		})
	}

	// Process the prompt which is in the format "\n\nHuman: ...\n\nAssistant: ..."
	parts := strings.Split(payload.Prompt, "\n\n")
	for _, part := range parts {
		if strings.HasPrefix(part, "Human: ") {
			content := strings.TrimPrefix(part, "Human: ")
			messages = append(messages, model.Message{
				"role":    "user",
				"content": content,
			})
		} else if strings.HasPrefix(part, "Assistant: ") {
			content := strings.TrimPrefix(part, "Assistant: ")
			// Skip empty Assistant messages (usually the last one expecting completion)
			if content != "" {
				messages = append(messages, model.Message{
					"role":    "assistant",
					"content": content,
				})
			}
		}
	}

	return messages
}

// extractBedrockTitanMessages extracts messages from Bedrock Amazon Titan format
func extractBedrockTitanMessages(body []byte) []model.Message {
	var payload struct {
		InputText string `json:"inputText"`
	}

	err := json.Unmarshal(body, &payload)
	if err != nil {
		return nil
	}

	messages := make([]model.Message, 0)

	// Process the input text which is in the format "System: ...\nUser: ...\nAssistant: ..."
	lines := strings.Split(payload.InputText, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "System: ") {
			content := strings.TrimPrefix(line, "System: ")
			messages = append(messages, model.Message{
				"role":    "system",
				"content": content,
			})
		} else if strings.HasPrefix(line, "User: ") {
			content := strings.TrimPrefix(line, "User: ")
			messages = append(messages, model.Message{
				"role":    "user",
				"content": content,
			})
		} else if strings.HasPrefix(line, "Assistant: ") {
			content := strings.TrimPrefix(line, "Assistant: ")
			messages = append(messages, model.Message{
				"role":    "assistant",
				"content": content,
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
		slog.Error("❌ Failed to unmarshal OpenAI payload",
			"error", err,
			"body", string(body))
		return nil, fmt.Errorf("failed to unmarshal OpenAI payload: %v, body: %s", err, string(body))
	}

	// Convert OpenAI messages to Vertex AI format
	var contents []map[string]interface{}
	for _, msg := range openAIPayload.Messages {
		role := msg["role"]
		content := msg["content"]

		if role == "assistant" {
			role = "model" // Vertex AI uses "model" instead of "assistant"
		} else if role == "system" {
			role = "user" // Vertex AI doesn't support system role, treat as user
		}

		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]interface{}{
				{
					"text": content,
				},
			},
		})
	}

	// Build the request payload according to Vertex AI's format
	type VertexPayload struct {
		Model            string                   `json:"model"`
		Contents         []map[string]interface{} `json:"contents"`
		GenerationConfig map[string]interface{}   `json:"generationConfig"`
		StopSequences    []string                 `json:"stopSequences,omitempty"`
		Extra            map[string]interface{}   `json:"extra,omitempty"`
	}

	// Extract just the model name if it contains a provider prefix or region
	modelName := model
	if strings.Contains(model, "/") {
		parts := strings.Split(model, "/")
		if len(parts) >= 2 {
			// For format: provider/model or model/region
			modelName = parts[1]

			// If it's provider/model/region format, we just want the model part
			if len(parts) > 2 && parts[0] == "vertex" {
				modelName = parts[1]
			}
		}
	}

	// Default to gemini-pro if no model is specified
	if modelName == "" {
		modelName = "gemini-pro"
		slog.Info("⚠️ No model specified, defaulting to gemini-pro")
	}

	slog.Info("🔄 Transforming to Vertex format", "model", modelName)

	vertexPayload := VertexPayload{
		Model:    modelName,
		Contents: contents,
		GenerationConfig: map[string]interface{}{
			"temperature":     openAIPayload.Temperature,
			"maxOutputTokens": openAIPayload.MaxTokens,
			"topP":            openAIPayload.TopP,
			"topK":            openAIPayload.TopK,
		},
		Extra: openAIPayload.Extra,
	}

	// Set defaults if values are not provided
	if openAIPayload.Temperature == 0 {
		vertexPayload.GenerationConfig["temperature"] = 0.7
	}
	if openAIPayload.MaxTokens == 0 {
		vertexPayload.GenerationConfig["maxOutputTokens"] = 1024
	}
	if openAIPayload.TopP == 0 {
		vertexPayload.GenerationConfig["topP"] = 0.95
	}
	if openAIPayload.TopK == 0 {
		vertexPayload.GenerationConfig["topK"] = 40
	}

	if len(openAIPayload.Stop) > 0 {
		vertexPayload.StopSequences = openAIPayload.Stop
	}

	// Initialize Extra if nil
	if vertexPayload.Extra == nil {
		vertexPayload.Extra = make(map[string]interface{})
	}

	// Copy any extra parameters
	for k, v := range openAIPayload.Extra {
		// Skip fields we already handle
		if k == "model" || k == "contents" || k == "generationConfig" || k == "stopSequences" {
			continue
		}
		vertexPayload.Extra[k] = v
		slog.Info("➕ Added extra parameter", "key", k, "value", v)
	}

	result, err := json.Marshal(vertexPayload)
	if err != nil {
		slog.Error("❌ Failed to marshal Vertex payload",
			"error", err,
			"payload", vertexPayload)
		return nil, fmt.Errorf("failed to marshal Vertex payload: %v", err)
	}

	return result, nil
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

// TransformFromVertexToOpenAI transforms Vertex AI format to OpenAI format
func TransformFromVertexToOpenAI(body []byte) ([]byte, error) {
	if len(body) == 0 {
		slog.Error("❌ Empty body received")
		return nil, fmt.Errorf("empty body received")
	}

	var vertexPayload struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		GenerationConfig map[string]interface{} `json:"generationConfig"`
	}

	if err := json.Unmarshal(body, &vertexPayload); err != nil {
		slog.Error("❌ Failed to unmarshal Vertex payload",
			"error", err,
			"body", string(body))
		return nil, fmt.Errorf("failed to unmarshal Vertex payload: %v", err)
	}

	// Convert Vertex messages to OpenAI format
	messages := make([]map[string]string, 0, len(vertexPayload.Contents))
	for _, content := range vertexPayload.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}
		if len(content.Parts) > 0 {
			message := map[string]string{
				"role":    role,
				"content": content.Parts[0].Text,
			}
			messages = append(messages, message)
		}
	}

	// Build OpenAI payload
	openaiPayload := map[string]interface{}{
		"messages": messages,
	}

	// Map generation config to OpenAI parameters
	if vertexPayload.GenerationConfig != nil {
		if temp, ok := vertexPayload.GenerationConfig["temperature"].(float64); ok {
			openaiPayload["temperature"] = temp
		}
		if maxTokens, ok := vertexPayload.GenerationConfig["maxOutputTokens"].(float64); ok {
			openaiPayload["max_tokens"] = int(maxTokens)
		}
		if topP, ok := vertexPayload.GenerationConfig["topP"].(float64); ok {
			openaiPayload["top_p"] = topP
		}
		// Intentionally skip topK as it's not supported by OpenAI/Azure
	}

	result, err := json.Marshal(openaiPayload)
	if err != nil {
		slog.Error("❌ Failed to marshal OpenAI payload",
			"error", err,
			"payload", openaiPayload)
		return nil, fmt.Errorf("failed to marshal OpenAI payload: %v", err)
	}

	return result, nil
}

// TransformFromBedrockResponse transforms a response from Bedrock to OpenAI format
func TransformFromBedrockResponse(body []byte, modelName string) ([]byte, error) {
	// Parse the Bedrock response
	var bedrockResponse map[string]interface{}
	err := json.Unmarshal(body, &bedrockResponse)
	if err != nil {
		return nil, fmt.Errorf("error parsing Bedrock response: %w", err)
	}

	// Determine the model vendor
	vendor := "anthropic" // Default to anthropic format
	if strings.Contains(modelName, ".") {
		vendor = strings.Split(modelName, ".")[0]
	}

	// Format OpenAI response
	var openAIResponse map[string]interface{}

	switch vendor {
	case "anthropic":
		// Extract completion from Anthropic response
		completion, ok := bedrockResponse["completion"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid or missing 'completion' in Anthropic response")
		}

		// Format as OpenAI response
		openAIResponse = map[string]interface{}{
			"id":      fmt.Sprintf("bedrock-%s-%d", modelName, time.Now().Unix()),
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   modelName,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": strings.TrimSpace(completion),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     0, // Bedrock doesn't report token counts in this way
				"completion_tokens": 0, // We'd need additional processing to estimate these
				"total_tokens":      0,
			},
		}

	case "amazon":
		// Extract completion from Amazon Titan response
		results, ok := bedrockResponse["results"].([]interface{})
		if !ok || len(results) == 0 {
			return nil, fmt.Errorf("invalid or missing 'results' in Amazon response")
		}

		resultObj, ok := results[0].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid first result in Amazon response")
		}

		outputText, ok := resultObj["outputText"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid or missing 'outputText' in Amazon response")
		}

		// Format as OpenAI response
		openAIResponse = map[string]interface{}{
			"id":      fmt.Sprintf("bedrock-%s-%d", modelName, time.Now().Unix()),
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   modelName,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": strings.TrimSpace(outputText),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     0, // Bedrock doesn't report token counts in this way
				"completion_tokens": 0, // We'd need additional processing to estimate these
				"total_tokens":      0,
			},
		}

	default:
		// For unsupported models, return the original response
		return body, nil
	}

	return json.Marshal(openAIResponse)
}
