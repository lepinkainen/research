package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a function call from the AI
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    string                 `json:"-"` // Not sent to API
}

// ChatResponse represents a response from the AI
type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
}

// Provider interface for AI providers
type Provider interface {
	Chat(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error)
}

// CreateProvider creates a provider based on type
func CreateProvider(providerType string) (Provider, error) {
	switch providerType {
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}
		return &OpenAIProvider{
			APIKey: apiKey,
			Model:  "gpt-4-turbo-preview",
		}, nil

	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		return &AnthropicProvider{
			APIKey: apiKey,
			Model:  "claude-3-5-sonnet-20241022",
		}, nil

	case "ollama":
		return &OllamaProvider{
			BaseURL: "http://localhost:11434",
			Model:   "llama3.1",
		}, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}

// OpenAIProvider implements Provider for OpenAI
type OpenAIProvider struct {
	APIKey string
	Model  string
}

type openAIRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []interface{} `json:"tools,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func (p *OpenAIProvider) Chat(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error) {
	req := openAIRequest{
		Model:    p.Model,
		Messages: messages,
	}

	if len(tools) > 0 {
		req.Tools = make([]interface{}, len(tools))
		for i, tool := range tools {
			req.Tools[i] = map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  tool.Parameters,
				},
			}
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from API")
	}

	response := &ChatResponse{
		Content: apiResp.Choices[0].Message.Content,
	}

	for _, tc := range apiResp.Choices[0].Message.ToolCalls {
		var args map[string]interface{}
		json.Unmarshal([]byte(tc.Function.Arguments), &args)

		response.ToolCalls = append(response.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return response, nil
}

// AnthropicProvider implements Provider for Anthropic Claude
type AnthropicProvider struct {
	APIKey string
	Model  string
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error) {
	// Convert to Anthropic format
	req := map[string]interface{}{
		"model":      p.Model,
		"max_tokens": 4096,
		"messages":   messages,
	}

	if len(tools) > 0 {
		anthropicTools := make([]map[string]interface{}, len(tools))
		for i, tool := range tools {
			anthropicTools[i] = map[string]interface{}{
				"name":         tool.Name,
				"description":  tool.Description,
				"input_schema": tool.Parameters,
			}
		}
		req["tools"] = anthropicTools
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var apiResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	response := &ChatResponse{}

	content, ok := apiResp["content"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	for _, block := range content {
		blockMap := block.(map[string]interface{})
		blockType := blockMap["type"].(string)

		if blockType == "text" {
			response.Content += blockMap["text"].(string)
		} else if blockType == "tool_use" {
			response.ToolCalls = append(response.ToolCalls, ToolCall{
				ID:        blockMap["id"].(string),
				Name:      blockMap["name"].(string),
				Arguments: blockMap["input"].(map[string]interface{}),
			})
		}
	}

	return response, nil
}

// OllamaProvider implements Provider for Ollama local models
type OllamaProvider struct {
	BaseURL string
	Model   string
}

func (p *OllamaProvider) Chat(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error) {
	req := map[string]interface{}{
		"model":    p.Model,
		"messages": messages,
		"stream":   false,
	}

	if len(tools) > 0 {
		ollamaTools := make([]map[string]interface{}, len(tools))
		for i, tool := range tools {
			ollamaTools[i] = map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  tool.Parameters,
				},
			}
		}
		req["tools"] = ollamaTools
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var apiResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	response := &ChatResponse{}

	if message, ok := apiResp["message"].(map[string]interface{}); ok {
		if content, ok := message["content"].(string); ok {
			response.Content = content
		}

		if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
			for _, tc := range toolCalls {
				tcMap := tc.(map[string]interface{})
				funcMap := tcMap["function"].(map[string]interface{})

				response.ToolCalls = append(response.ToolCalls, ToolCall{
					ID:        fmt.Sprintf("%v", tcMap["id"]),
					Name:      funcMap["name"].(string),
					Arguments: funcMap["arguments"].(map[string]interface{}),
				})
			}
		}
	}

	return response, nil
}
