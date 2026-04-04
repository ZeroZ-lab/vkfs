package embedding

import "context"

// EmbeddingProvider converts text to vector embeddings
type EmbeddingProvider interface {
	// Embed converts a single text to embedding vector
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch converts multiple texts to embeddings (used for ingest)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding vector dimension
	Dimension() int
}
