package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zhengjianqiao/vkfs/pkg/vfs"
)

// ZillizRESTAdapter implements VectorStore using Zilliz Cloud REST API (v2).
// Required for Zilliz Serverless instances where gRPC is unavailable.
type ZillizRESTAdapter struct {
	endpoint    string
	apiKey      string
	collection  string
	dimension   int
	client      *http.Client
}

// ZillizRESTConfig holds configuration for the REST-based Zilliz adapter
type ZillizRESTConfig struct {
	Endpoint   string
	APIKey     string
	Collection string
	Dimension  int
}

// NewZillizRESTAdapter creates a new REST-based Zilliz adapter
func NewZillizRESTAdapter(cfg ZillizRESTConfig) (*ZillizRESTAdapter, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if cfg.Collection == "" {
		return nil, fmt.Errorf("collection is required")
	}
	dim := cfg.Dimension
	if dim == 0 {
		dim = 1024
	}

	return &ZillizRESTAdapter{
		endpoint:   strings.TrimRight(cfg.Endpoint, "/"),
		apiKey:     cfg.APIKey,
		collection: cfg.Collection,
		dimension:  dim,
		client:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Close is a no-op for REST adapter
func (z *ZillizRESTAdapter) Close() error { return nil }

func (z *ZillizRESTAdapter) doRequest(method, path string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, z.endpoint+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+z.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(raw))
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error (code %d): %s", result.Code, result.Message)
	}

	return result.Data, nil
}

// UpsertPathTree stores the complete file tree as a special document
func (z *ZillizRESTAdapter) UpsertPathTree(ctx context.Context, tree vfs.PathTree) error {
	treeJSON, err := json.Marshal(tree)
	if err != nil {
		return fmt.Errorf("marshal PathTree: %w", err)
	}

	zeroVec := make([]float32, z.dimension)

	_, err = z.doRequest("POST", "/v2/vectordb/entities/upsert", map[string]interface{}{
		"collectionName": z.collection,
		"data": []map[string]interface{}{
			{
				"id":       "__path_tree__",
				"vector":   zeroVec,
				"doc_type": "path_tree",
				"content":  string(treeJSON),
			},
		},
	})
	return err
}

// GetPathTree retrieves the complete file tree
func (z *ZillizRESTAdapter) GetPathTree(ctx context.Context) (vfs.PathTree, error) {
	data, err := z.doRequest("POST", "/v2/vectordb/entities/query", map[string]interface{}{
		"collectionName": z.collection,
		"filter":         `id == "__path_tree__"`,
		"outputFields":   []string{"content"},
	})
	if err != nil {
		return vfs.PathTree{}, err
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		return vfs.PathTree{}, fmt.Errorf("parse query result: %w", err)
	}

	if len(results) == 0 {
		return vfs.PathTree{}, fmt.Errorf("PathTree not found - run vkfs-admin init first")
	}

	content, ok := results[0]["content"].(string)
	if !ok {
		return vfs.PathTree{}, fmt.Errorf("PathTree content field missing")
	}

	var tree vfs.PathTree
	if err := json.Unmarshal([]byte(content), &tree); err != nil {
		return vfs.PathTree{}, fmt.Errorf("unmarshal PathTree: %w", err)
	}

	return tree, nil
}

// UpsertChunks batch inserts chunks
func (z *ZillizRESTAdapter) UpsertChunks(ctx context.Context, chunks []vfs.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	records := make([]map[string]interface{}, len(chunks))
	for i, c := range chunks {
		records[i] = map[string]interface{}{
			"id":          c.ID,
			"vector":      c.Embedding,
			"doc_type":    "chunk",
			"page_slug":   c.PageSlug,
			"chunk_index": c.ChunkIndex,
			"text":        c.Text,
		}
	}

	_, err := z.doRequest("POST", "/v2/vectordb/entities/upsert", map[string]interface{}{
		"collectionName": z.collection,
		"data":           records,
	})
	return err
}

// GetChunksByPage retrieves all chunks for a file
func (z *ZillizRESTAdapter) GetChunksByPage(ctx context.Context, pageSlug string) ([]vfs.Chunk, error) {
	data, err := z.doRequest("POST", "/v2/vectordb/entities/query", map[string]interface{}{
		"collectionName": z.collection,
		"filter":         fmt.Sprintf(`page_slug == "%s" && doc_type == "chunk"`, pageSlug),
		"outputFields":   []string{"id", "page_slug", "chunk_index", "text"},
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse query result: %w", err)
	}

	chunks := make([]vfs.Chunk, len(results))
	for i, r := range results {
		chunks[i] = vfs.Chunk{
			ID:         jsonStr(r, "id"),
			PageSlug:   jsonStr(r, "page_slug"),
			ChunkIndex: jsonInt(r, "chunk_index"),
			Text:       jsonStr(r, "text"),
		}
	}

	return chunks, nil
}

// DeleteChunksByPage deletes all chunks for a file
func (z *ZillizRESTAdapter) DeleteChunksByPage(ctx context.Context, pageSlug string) error {
	_, err := z.doRequest("POST", "/v2/vectordb/entities/delete", map[string]interface{}{
		"collectionName": z.collection,
		"filter":         fmt.Sprintf(`page_slug == "%s" && doc_type == "chunk"`, pageSlug),
	})
	return err
}

// UpsertLazyPointer stores external storage reference
func (z *ZillizRESTAdapter) UpsertLazyPointer(ctx context.Context, pointer vfs.LazyPointer) error {
	zeroVec := make([]float32, z.dimension)

	_, err := z.doRequest("POST", "/v2/vectordb/entities/upsert", map[string]interface{}{
		"collectionName": z.collection,
		"data": []map[string]interface{}{
			{
				"id":           "lazy_" + pointer.PageSlug,
				"vector":       zeroVec,
				"doc_type":     "lazy_pointer",
				"page_slug":    pointer.PageSlug,
				"external_url": pointer.ExternalURL,
				"size":         pointer.Size,
			},
		},
	})
	return err
}

// GetLazyPointer checks if a file is a lazy pointer
func (z *ZillizRESTAdapter) GetLazyPointer(ctx context.Context, pageSlug string) (*vfs.LazyPointer, error) {
	data, err := z.doRequest("POST", "/v2/vectordb/entities/query", map[string]interface{}{
		"collectionName": z.collection,
		"filter":         fmt.Sprintf(`id == "lazy_%s" && doc_type == "lazy_pointer"`, pageSlug),
		"outputFields":   []string{"page_slug", "external_url", "size"},
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, nil // not found
	}

	if len(results) == 0 {
		return nil, nil
	}

	r := results[0]
	return &vfs.LazyPointer{
		PageSlug:    jsonStr(r, "page_slug"),
		ExternalURL: jsonStr(r, "external_url"),
		Size:        int64(jsonFloat(r, "size")),
	}, nil
}

// SearchText performs text search (BM25-like via query filter)
func (z *ZillizRESTAdapter) SearchText(ctx context.Context, pattern string, filter vfs.PathFilter, limit int) ([]vfs.Chunk, error) {
	expr := `doc_type == "chunk"`
	if filter.PathPrefix != "" && filter.PathPrefix != "/" {
		expr += fmt.Sprintf(` && page_slug like "%s%%"`, filter.PathPrefix)
	}

	data, err := z.doRequest("POST", "/v2/vectordb/entities/query", map[string]interface{}{
		"collectionName": z.collection,
		"filter":         expr,
		"outputFields":   []string{"id", "page_slug", "chunk_index", "text"},
		"limit":          limit,
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse query result: %w", err)
	}

	chunks := make([]vfs.Chunk, len(results))
	for i, r := range results {
		chunks[i] = vfs.Chunk{
			ID:         jsonStr(r, "id"),
			PageSlug:   jsonStr(r, "page_slug"),
			ChunkIndex: jsonInt(r, "chunk_index"),
			Text:       jsonStr(r, "text"),
		}
	}

	return chunks, nil
}

// SearchVector performs vector similarity search
func (z *ZillizRESTAdapter) SearchVector(ctx context.Context, queryVec []float32, filter vfs.PathFilter, topK int) ([]vfs.SearchHit, error) {
	expr := `doc_type == "chunk"`
	if filter.PathPrefix != "" && filter.PathPrefix != "/" {
		expr += fmt.Sprintf(` && page_slug like "%s%%"`, filter.PathPrefix)
	}

	data, err := z.doRequest("POST", "/v2/vectordb/entities/search", map[string]interface{}{
		"collectionName": z.collection,
		"data":           [][]float32{queryVec},
		"annsField":      "vector",
		"filter":         expr,
		"outputFields":   []string{"id", "page_slug", "chunk_index", "text"},
		"limit":          topK,
		"searchParams": map[string]interface{}{
			"metric_type": "L2",
		},
	})
	if err != nil {
		return nil, err
	}

	// Search returns flat array: [{...}, {...}]
	var results []map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse search result: %w", err)
	}

	hits := make([]vfs.SearchHit, 0, len(results))
	for _, r := range results {
		hits = append(hits, vfs.SearchHit{
			Chunk: vfs.Chunk{
				ID:         jsonStr(r, "id"),
				PageSlug:   jsonStr(r, "page_slug"),
				ChunkIndex: jsonInt(r, "chunk_index"),
				Text:       jsonStr(r, "text"),
			},
			Score: float32(jsonFloat(r, "distance")),
		})
	}

	return hits, nil
}

// SearchHybrid is not yet implemented
func (z *ZillizRESTAdapter) SearchHybrid(ctx context.Context, queryVec []float32, pattern string, filter vfs.PathFilter, topK int) ([]vfs.SearchHit, error) {
	return nil, fmt.Errorf("not implemented yet")
}

// JSON helper functions
func jsonStr(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func jsonInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func jsonFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}
