package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaEmbed(t *testing.T) {
	expected := []float32{0.1, 0.2, 0.3}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/embed" {
			t.Errorf("expected /api/embed, got %s", r.URL.Path)
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if reqBody["model"] != "nomic-embed-text" {
			t.Errorf("expected model nomic-embed-text, got %v", reqBody["model"])
		}

		resp, _ := json.Marshal(map[string]interface{}{
			"embeddings": [][]float32{expected},
		})
		w.Write(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("nomic-embed-text")
	p.WithBaseURL(server.URL)

	result, err := p.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(result) != len(expected) {
		t.Fatalf("expected %d dimensions, got %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("dimension %d: expected %f, got %f", i, expected[i], v)
		}
	}
}

func TestOllamaEmbedBatch(t *testing.T) {
	texts := []string{"text one", "text two", "text three"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		input, ok := reqBody["input"].([]interface{})
		if !ok {
			t.Fatalf("expected input to be an array, got %T", reqBody["input"])
		}
		if len(input) != len(texts) {
			t.Errorf("expected %d inputs, got %d", len(texts), len(input))
		}

		embeddings := make([][]float32, len(input))
		for i := range input {
			embeddings[i] = []float32{float32(i), 0.5}
		}

		resp, _ := json.Marshal(map[string]interface{}{
			"embeddings": embeddings,
		})
		w.Write(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("test-model")
	p.WithBaseURL(server.URL)

	results, err := p.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(results))
	}
}

func TestOllamaEmbedBatch_Empty(t *testing.T) {
	p := NewOllamaProvider("test-model")

	results, err := p.EmbedBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("EmbedBatch with empty input failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 embeddings, got %d", len(results))
	}
}

func TestOllamaEmbed_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("model not found"))
	}))
	defer server.Close()

	p := NewOllamaProvider("bad-model")
	p.WithBaseURL(server.URL)

	_, err := p.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestOllamaEmbed_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	p := NewOllamaProvider("test-model")
	p.WithBaseURL(server.URL)

	_, err := p.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}

func TestOllamaEmbed_EmptyEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, _ := json.Marshal(map[string]interface{}{
			"embeddings": [][]float32{},
		})
		w.Write(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("test-model")
	p.WithBaseURL(server.URL)

	_, err := p.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for empty embeddings")
	}
}

func TestOllamaEmbed_WithAPIKey(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		resp, _ := json.Marshal(map[string]interface{}{
			"embeddings": [][]float32{{0.1}},
		})
		w.Write(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("test-model")
	p.WithBaseURL(server.URL)
	p.WithAPIKey("my-secret-key")

	_, err := p.Embed(context.Background(), "test")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if receivedAuth != "Bearer my-secret-key" {
		t.Errorf("expected 'Bearer my-secret-key', got '%s'", receivedAuth)
	}
}

func TestOllamaEmbed_NoAPIKey(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		resp, _ := json.Marshal(map[string]interface{}{
			"embeddings": [][]float32{{0.1}},
		})
		w.Write(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("test-model")
	p.WithBaseURL(server.URL)

	_, err := p.Embed(context.Background(), "test")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if receivedAuth != "" {
		t.Errorf("expected no Authorization header, got '%s'", receivedAuth)
	}
}

func TestOllamaProvider_Dimension(t *testing.T) {
	p := NewOllamaProvider("test-model")
	if p.Dimension() != -1 {
		t.Errorf("expected Dimension() = -1, got %d", p.Dimension())
	}
}

func TestOllamaWithBaseURL_TrailingSlash(t *testing.T) {
	p := NewOllamaProvider("test-model")
	p.WithBaseURL("http://localhost:11434/")

	if p.baseURL != "http://localhost:11434" {
		t.Errorf("expected trailing slash trimmed, got %s", p.baseURL)
	}
}
