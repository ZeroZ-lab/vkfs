package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

func TestLoad_Ollama_ValidMinimal(t *testing.T) {
	path := writeTestConfig(t, `
vectorstore:
  backend: sqlite
embedding:
  provider: ollama
  ollama:
    model: nomic-embed-text
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Embedding.Provider != "ollama" {
		t.Errorf("expected provider ollama, got %s", cfg.Embedding.Provider)
	}
	if cfg.Embedding.Ollama.Model != "nomic-embed-text" {
		t.Errorf("expected model nomic-embed-text, got %s", cfg.Embedding.Ollama.Model)
	}
	if cfg.Embedding.Ollama.BaseURL != "http://localhost:11434" {
		t.Errorf("expected default base_url http://localhost:11434, got %s", cfg.Embedding.Ollama.BaseURL)
	}
}

func TestLoad_Ollama_WithBaseURL(t *testing.T) {
	path := writeTestConfig(t, `
vectorstore:
  backend: sqlite
embedding:
  provider: ollama
  ollama:
    model: mxbai-embed-large
    base_url: "http://192.168.1.100:11434"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Embedding.Ollama.BaseURL != "http://192.168.1.100:11434" {
		t.Errorf("expected custom base_url, got %s", cfg.Embedding.Ollama.BaseURL)
	}
}

func TestLoad_Ollama_MissingModel(t *testing.T) {
	path := writeTestConfig(t, `
vectorstore:
  backend: sqlite
embedding:
  provider: ollama
  ollama:
    base_url: "http://localhost:11434"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestLoad_Ollama_WithAPIKey(t *testing.T) {
	os.Setenv("TEST_OLLAMA_KEY", "secret123")
	defer os.Unsetenv("TEST_OLLAMA_KEY")

	path := writeTestConfig(t, `
vectorstore:
  backend: sqlite
embedding:
  provider: ollama
  ollama:
    model: nomic-embed-text
    api_key: "${TEST_OLLAMA_KEY}"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Embedding.Ollama.APIKey != "secret123" {
		t.Errorf("expected api_key 'secret123', got %s", cfg.Embedding.Ollama.APIKey)
	}
}

func TestLoad_Ollama_EnvInterpolation(t *testing.T) {
	os.Setenv("TEST_OLLAMA_MODEL", "my-custom-model")
	defer os.Unsetenv("TEST_OLLAMA_MODEL")

	path := writeTestConfig(t, `
vectorstore:
  backend: sqlite
embedding:
  provider: ollama
  ollama:
    model: "${TEST_OLLAMA_MODEL}"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Embedding.Ollama.Model != "my-custom-model" {
		t.Errorf("expected model from env, got %s", cfg.Embedding.Ollama.Model)
	}
}

func TestLoad_Ollama_EmptyConfig(t *testing.T) {
	path := writeTestConfig(t, `
vectorstore:
  backend: sqlite
embedding:
  provider: ollama
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing ollama config")
	}
}

func TestValidate_UnsupportedProvider(t *testing.T) {
	cfg := &Config{}
	cfg.VectorStore.Backend = "sqlite"
	cfg.Embedding.Provider = "invalid_provider"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
