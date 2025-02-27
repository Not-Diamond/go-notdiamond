package response

import (
	"encoding/json"
	"fmt"
	"time"
)

type Result struct {
	Model     string
	Response  string
	TimeTaken time.Duration
}

// Parse takes a response body and the time the request started,
// and returns the parsed result from either OpenAI or Vertex AI
func Parse(body []byte, startTime time.Time) (*Result, error) {
	// Try to parse as OpenAI response first
	var openaiResponse struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &openaiResponse); err == nil && len(openaiResponse.Choices) > 0 {
		// Extract model name to determine which provider was used
		modelName := openaiResponse.Model
		if modelName == "" {
			modelName = "gpt-4o" // Default if not specified
		}

		return &Result{
			Model:     modelName,
			Response:  openaiResponse.Choices[0].Message.Content,
			TimeTaken: time.Since(startTime),
		}, nil
	}

	// If not OpenAI, try to parse as Bedrock Anthropic Claude response
	var bedrockAnthropicResponse struct {
		Completion string `json:"completion"`
	}

	if err := json.Unmarshal(body, &bedrockAnthropicResponse); err == nil && bedrockAnthropicResponse.Completion != "" {
		return &Result{
			Model:     "anthropic.claude",
			Response:  bedrockAnthropicResponse.Completion,
			TimeTaken: time.Since(startTime),
		}, nil
	}

	// Try the new Bedrock Claude format (Claude 3)
	var bedrockClaude3Response struct {
		Type       string                 `json:"type"`
		Id         string                 `json:"id"`
		Model      string                 `json:"model"`
		StopReason string                 `json:"stop_reason"`
		Usage      map[string]interface{} `json:"usage"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
			Role string `json:"role"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &bedrockClaude3Response); err == nil && len(bedrockClaude3Response.Content) > 0 {
		var responseText string
		for _, content := range bedrockClaude3Response.Content {
			if content.Role == "assistant" && content.Type == "text" {
				responseText = content.Text
				break
			}
		}

		if responseText != "" {
			return &Result{
				Model:     bedrockClaude3Response.Model,
				Response:  responseText,
				TimeTaken: time.Since(startTime),
			}, nil
		}
	}

	// If not Anthropic, try to parse as Bedrock Amazon Titan response
	var bedrockTitanResponse struct {
		Results []struct {
			OutputText string `json:"outputText"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &bedrockTitanResponse); err == nil &&
		len(bedrockTitanResponse.Results) > 0 &&
		bedrockTitanResponse.Results[0].OutputText != "" {
		return &Result{
			Model:     "amazon.titan",
			Response:  bedrockTitanResponse.Results[0].OutputText,
			TimeTaken: time.Since(startTime),
		}, nil
	}

	// If not Bedrock, try to parse as Vertex AI response
	var vertexResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			CitationMetadata *struct {
				Citations []struct {
					StartIndex int `json:"startIndex"`
					EndIndex   int `json:"endIndex"`
				} `json:"citations"`
			} `json:"citationMetadata"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &vertexResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if len(vertexResponse.Candidates) == 0 {
		return nil, fmt.Errorf("response did not contain any candidates: %s", string(body))
	}

	candidate := vertexResponse.Candidates[0]

	// Check if the response was blocked due to recitation
	if candidate.FinishReason == "RECITATION" {
		return nil, fmt.Errorf("response was blocked due to content recitation. Please rephrase your query")
	}

	// Check if we have valid content
	if len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("response candidate did not contain any content parts: %s", string(body))
	}

	return &Result{
		Model:     "gemini-pro",
		Response:  candidate.Content.Parts[0].Text,
		TimeTaken: time.Since(startTime),
	}, nil
}
