package embedding

import (
	"testing"

	"github.com/ZeroZ-lab/vkfs/internal/config"
)

func TestNewFromConfig_Ollama(t *testing.T) {
	cfg := &config.Config{}
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "nomic-embed-text"
	cfg.Embedding.Ollama.BaseURL = "http://localhost:11434"

	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}

	ollama, ok := provider.(*OllamaProvider)
	if !ok {
		t.Fatal("expected *OllamaProvider")
	}

	if ollama.model != "nomic-embed-text" {
		t.Errorf("expected model nomic-embed-text, got %s", ollama.model)
	}
	if ollama.baseURL != "http://localhost:11434" {
		t.Errorf("expected baseURL http://localhost:11434, got %s", ollama.baseURL)
	}
}

func TestNewFromConfig_Ollama_WithAPIKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Embedding.Provider = "ollama"
	cfg.Embedding.Ollama.Model = "test-model"
	cfg.Embedding.Ollama.BaseURL = "http://localhost:11434"
	cfg.Embedding.Ollama.APIKey = "my-key"

	provider, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}

	ollama := provider.(*OllamaProvider)
	if ollama.apiKey != "my-key" {
		t.Errorf("expected apiKey 'my-key', got %s", ollama.apiKey)
	}
}

func TestNewFromConfig_Unsupported(t *testing.T) {
	cfg := &config.Config{}
	cfg.Embedding.Provider = "unsupported"

	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
