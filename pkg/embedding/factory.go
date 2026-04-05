package embedding

import (
	"fmt"

	"github.com/ZeroZ-lab/vkfs/internal/config"
)

// NewFromConfig creates an EmbeddingProvider based on the configuration.
func NewFromConfig(cfg *config.Config) (EmbeddingProvider, error) {
	switch cfg.Embedding.Provider {
	case "openai":
		p := NewOpenAIProvider(cfg.Embedding.OpenAI.APIKey, cfg.Embedding.OpenAI.Model)
		if cfg.Embedding.OpenAI.BaseURL != "" {
			p.WithBaseURL(cfg.Embedding.OpenAI.BaseURL)
		}
		return p, nil
	case "cohere":
		return NewCohereProvider(cfg.Embedding.Cohere.APIKey, cfg.Embedding.Cohere.Model), nil
	case "siliconflow":
		p := NewOpenAIProvider(cfg.Embedding.SiliconFlow.APIKey, cfg.Embedding.SiliconFlow.Model)
		p.WithBaseURL("https://api.siliconflow.cn/v1/embeddings")
		return p, nil
	case "ollama":
		p := NewOllamaProvider(cfg.Embedding.Ollama.Model)
		if cfg.Embedding.Ollama.BaseURL != "" {
			p.WithBaseURL(cfg.Embedding.Ollama.BaseURL)
		}
		if cfg.Embedding.Ollama.APIKey != "" {
			p.WithAPIKey(cfg.Embedding.Ollama.APIKey)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Embedding.Provider)
	}
}
