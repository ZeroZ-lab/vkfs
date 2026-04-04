package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAIProvider implements EmbeddingProvider for OpenAI-compatible APIs
type OpenAIProvider struct {
	apiKey   string
	model    string
	baseURL  string
	client   *http.Client
}

// NewOpenAIProvider creates a new OpenAI embedding provider
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = "text-embedding-3-small" // default
	}

	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.openai.com/v1/embeddings",
		client:  &http.Client{},
	}
}

// WithBaseURL sets a custom API base URL (for OpenAI-compatible providers)
func (p *OpenAIProvider) WithBaseURL(url string) *OpenAIProvider {
	p.baseURL = url
	return p
}

// Embed converts a single text to embedding
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embeddings[0], nil
}

// EmbedBatch converts multiple texts to embeddings
func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Prepare request
	reqBody := map[string]interface{}{
		"input": texts,
		"model": p.model,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var respData struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract embeddings
	embeddings := make([][]float32, len(respData.Data))
	for i, item := range respData.Data {
		embeddings[i] = item.Embedding
	}

	return embeddings, nil
}

// Dimension returns the embedding dimension
func (p *OpenAIProvider) Dimension() int {
	switch p.model {
	case "text-embedding-3-large":
		return 3072
	case "BAAI/bge-m3":
		return 1024
	default:
		return 1536
	}
}
