package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaProvider implements EmbeddingProvider for Ollama native API
type OllamaProvider struct {
	baseURL   string
	model     string
	apiKey    string
	dimension int
	client    *http.Client
}

// NewOllamaProvider creates a new Ollama embedding provider
func NewOllamaProvider(model string) *OllamaProvider {
	return &OllamaProvider{
		model:     model,
		baseURL:   "http://localhost:11434",
		dimension: -1, // -1 means unknown, user should set via WithDimension
		client:    &http.Client{},
	}
}

// WithBaseURL sets a custom Ollama API base URL
func (p *OllamaProvider) WithBaseURL(url string) *OllamaProvider {
	p.baseURL = strings.TrimRight(url, "/")
	return p
}

// WithAPIKey sets an optional API key for authenticated Ollama instances
func (p *OllamaProvider) WithAPIKey(key string) *OllamaProvider {
	p.apiKey = key
	return p
}

// WithDimension sets the embedding dimension for the model
// This is required since Ollama models can have different dimensions
func (p *OllamaProvider) WithDimension(dim int) *OllamaProvider {
	p.dimension = dim
	return p
}

// Embed converts a single text to embedding
func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned from Ollama")
	}

	return embeddings[0], nil
}

// EmbedBatch converts multiple texts to embeddings using /api/embed batch support
func (p *OllamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	reqBody := map[string]interface{}{
		"model": p.model,
		"input": texts,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/embed", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var respData struct {
		Embeddings [][]float32 `json:"embeddings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(respData.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned from Ollama")
	}

	return respData.Embeddings, nil
}

// Dimension returns the embedding dimension (-1 = unknown, user must set via WithDimension)
func (p *OllamaProvider) Dimension() int {
	return p.dimension
}
