package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CohereProvider implements EmbeddingProvider for Cohere
type CohereProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewCohereProvider creates a new Cohere embedding provider
func NewCohereProvider(apiKey, model string) *CohereProvider {
	if model == "" {
		model = "embed-english-v3.0" // default
	}

	return &CohereProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

// Embed converts a single text to embedding
func (p *CohereProvider) Embed(ctx context.Context, text string) ([]float32, error) {
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
func (p *CohereProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Prepare request
	reqBody := map[string]interface{}{
		"texts":      texts,
		"model":      p.model,
		"input_type": "search_document", // for indexing documents
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.cohere.ai/v1/embed", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cohere API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Cohere API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var respData struct {
		Embeddings [][]float32 `json:"embeddings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return respData.Embeddings, nil
}

// Dimension returns the embedding dimension
func (p *CohereProvider) Dimension() int {
	// embed-english-v3.0: 1024
	// embed-multilingual-v3.0: 1024
	return 1024
}
