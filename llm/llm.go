package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// OpenRouterClient interacts with the OpenRouter API.
type OpenRouterClient struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Logger     *log.Logger
	Prompts    map[string]string // Predefined prompts for injection
}

// NewOpenRouterClient creates a new OpenRouterClient with predefined prompts.
func NewOpenRouterClient(apiKey, baseURL string, logger *log.Logger) *OpenRouterClient {
	return &OpenRouterClient{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
		Logger:     logger,
		Prompts: map[string]string{
			"default":    "You are anon dd agent, you have to reply to messages in engaging way, if asked for advice on crypto give solid dd on any random ai name like agent ( advice on crypto, ai agents bull run and politics, be a degen but keep it cool, sometimes be dark , and be nice sometimes like a regen. talk about memes, but be Absurd boy Keep your response concise and not more than two sentences and your name is anonddagent or add, dont be over the top, stay little easy: %s",
			"summarize":  "Summarize the following text: %s",
			"translate":  "Translate the following text to Spanish: %s",
			"custom":     "Analyze and provide detailed insights: %s",
			"agent_analysis": "As a crypto and AI market analyst, provide a brief analysis of these agents focusing on their potential value and unique features. Keep it concise and highlight the most interesting aspects: %s",
		},
	}
}

// OpenRouterResponse represents the response from OpenRouter API.
type OpenRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// GetResponse sends a query to OpenRouter with a specific prompt injected.
func (client *OpenRouterClient) GetResponse(ctx context.Context, promptKey string, userQuery string) (string, error) {
	// Retrieve the prompt template
	promptTemplate, exists := client.Prompts[promptKey]
	if !exists {
		client.Logger.Printf("Prompt key '%s' not found, falling back to default.", promptKey)
		promptTemplate = client.Prompts["default"]
	}

	// Inject the user query into the prompt
	prompt := fmt.Sprintf(promptTemplate, userQuery)
	client.Logger.Printf("Generated prompt: %s", prompt)

	// Construct the request payload
	requestBody, err := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"model": "meta-llama/llama-3.2-3b-instruct:free",
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode request body: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", client.BaseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))

	// Execute the request
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	client.Logger.Printf("OpenRouter API Response: %s", string(body))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenRouter API error: %s", string(body))
	}

	// Parse the response JSON
	var openRouterResponse OpenRouterResponse
	if err := json.Unmarshal(body, &openRouterResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(openRouterResponse.Choices) > 0 {
		return openRouterResponse.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response received from OpenRouter")
}
