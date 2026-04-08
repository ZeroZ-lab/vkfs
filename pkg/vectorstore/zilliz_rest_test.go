package vectorstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

func newTestRESTAdapter(t *testing.T) (*ZillizRESTAdapter, *http.ServeMux) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	adapter, err := NewZillizRESTAdapter(ZillizRESTConfig{
		Endpoint:   server.URL,
		APIKey:     "test-key",
		Collection: "test_col",
		Dimension: 4,
	})
	require.NoError(t, err)
	t.Cleanup(server.Close)
	return adapter, mux
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 0,
		"data": v,
	})
}

func TestZillizREST_UpsertPathTree(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	tree := vfs.PathTree{
		Nodes: map[string]vfs.VirtualNode{
			"/": {Path: "/", Name: "/", IsDir: true},
		},
		Version: "1.0",
	}

	mux.HandleFunc("/v2/vectordb/entities/upsert", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,map[string]interface{}{"upsertCount": 1})
	})

	err := adapter.UpsertPathTree(ctx, tree)
	assert.NoError(t, err)
}

func TestZillizREST_GetPathTree(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	expected := vfs.PathTree{
		Nodes:   map[string]vfs.VirtualNode{"/": {Path: "/", IsDir: true}},
		Version: "1.0",
	}
	treeJSON, _ := json.Marshal(expected)

	mux.HandleFunc("/v2/vectordb/entities/query", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,[]map[string]interface{}{
			{"content": string(treeJSON)},
		})
	})

	got, err := adapter.GetPathTree(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.0", got.Version)
}

func TestZillizREST_GetPathTree_NotFound(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	mux.HandleFunc("/v2/vectordb/entities/query", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,[]interface{}{})
	})

	_, err := adapter.GetPathTree(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestZillizREST_UpsertChunks(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	chunks := []vfs.Chunk{
		{ID: "c0", PageSlug: "/a.md", ChunkIndex: 0, Text: "hello", Embedding: []float32{1, 0, 0, 0}},
	}

	mux.HandleFunc("/v2/vectordb/entities/upsert", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,map[string]interface{}{"upsertCount": 1})
	})

	err := adapter.UpsertChunks(ctx, chunks)
	require.NoError(t, err)
}

func TestZillizREST_UpsertChunks_Empty(t *testing.T) {
	adapter, _ := newTestRESTAdapter(t)
	ctx := context.Background()
	err := adapter.UpsertChunks(ctx, nil)
	assert.NoError(t, err)
}

func TestZillizREST_GetChunksByPage(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	mux.HandleFunc("/v2/vectordb/entities/query", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,[]map[string]interface{}{
			{"id": "c0", "page_slug": "/a.md", "chunk_index": float64(0), "text": "hello"},
			{"id": "c1", "page_slug": "/a.md", "chunk_index": float64(1), "text": "world"},
		})
	})

	got, err := adapter.GetChunksByPage(ctx, "/a.md")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, 0, got[0].ChunkIndex)
	assert.Equal(t, 1, got[1].ChunkIndex)
}

func TestZillizREST_DeleteChunksByPage(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	mux.HandleFunc("/v2/vectordb/entities/delete", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,map[string]interface{}{"deleteCount": 2})
	})

	err := adapter.DeleteChunksByPage(ctx, "/a.md")
	require.NoError(t, err)
}

func TestZillizREST_UpsertGetLazyPointer(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	pointer := vfs.LazyPointer{
		PageSlug:    "/large/file.md",
		ExternalURL: "s3://bucket/key",
		Size:        1024,
	}

	mux.HandleFunc("/v2/vectordb/entities/upsert", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,map[string]interface{}{"upsertCount": 1})
	})

	err := adapter.UpsertLazyPointer(ctx, pointer)
	require.NoError(t, err)

	mux.HandleFunc("/v2/vectordb/entities/query", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,[]map[string]interface{}{
			{"page_slug": "/large/file.md", "external_url": "s3://bucket/key", "size": float64(1024)},
		})
	})

	got, err := adapter.GetLazyPointer(ctx, "/large/file.md")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "s3://bucket/key", got.ExternalURL)
}

func TestZillizREST_GetLazyPointer_NotFound(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	mux.HandleFunc("/v2/vectordb/entities/query", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,[]interface{}{})
	})

	got, err := adapter.GetLazyPointer(ctx, "/nonexistent.md")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestZillizREST_SearchText(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	mux.HandleFunc("/v2/vectordb/entities/query", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,[]map[string]interface{}{
			{"id": "c0", "page_slug": "/a.md", "chunk_index": float64(0), "text": "Go language"},
		})
	})

	results, err := adapter.SearchText(ctx, "Go", vfs.PathFilter{}, 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Go language", results[0].Text)
}

func TestZillizREST_SearchVector(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	mux.HandleFunc("/v2/vectordb/entities/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w,[]map[string]interface{}{
			{"id": "c0", "page_slug": "/a.md", "chunk_index": float64(0), "text": "close", "distance": float64(0.1)},
		})
	})

	hits, err := adapter.SearchVector(ctx, []float32{1, 0, 0, 0}, vfs.PathFilter{}, 5)
	require.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, "c0", hits[0].Chunk.ID)
}

func TestZillizREST_SearchHybrid_NotImplemented(t *testing.T) {
	adapter, _ := newTestRESTAdapter(t)
	ctx := context.Background()

	_, err := adapter.SearchHybrid(ctx, []float32{1, 0, 0, 0}, "test", vfs.PathFilter{}, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestZillizREST_APIError(t *testing.T) {
	adapter, mux := newTestRESTAdapter(t)
	ctx := context.Background()

	mux.HandleFunc("/v2/vectordb/entities/query", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    500,
				"message": "internal error",
			})
	})

	_, err := adapter.GetPathTree(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestZillizREST_NewAdapter_Validation(t *testing.T) {
	_, err := NewZillizRESTAdapter(ZillizRESTConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")

	_, err = NewZillizRESTAdapter(ZillizRESTConfig{Endpoint: "http://localhost"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")

	_, err = NewZillizRESTAdapter(ZillizRESTConfig{Endpoint: "http://localhost", APIKey: "key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collection")
}
