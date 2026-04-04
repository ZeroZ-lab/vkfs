package vectorstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/zhengjianqiao/vkfs/pkg/vfs"
)

// ZillizAdapter implements VectorStore interface for Zilliz Cloud (Milvus)
type ZillizAdapter struct {
	client     client.Client
	collection string
	dimension  int
}

// ZillizConfig holds Zilliz connection configuration
type ZillizConfig struct {
	Endpoint   string
	APIKey     string
	Collection string
	Dimension  int
}

// NewZillizAdapter creates a new Zilliz adapter
func NewZillizAdapter(cfg ZillizConfig) (*ZillizAdapter, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("ZILLIZ_ENDPOINT is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ZILLIZ_API_KEY is required")
	}
	if cfg.Collection == "" {
		return nil, fmt.Errorf("collection name is required")
	}

	// Connect to Zilliz Cloud
	c, err := client.NewClient(context.Background(), client.Config{
		Address: cfg.Endpoint,
		APIKey:  cfg.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Zilliz: %w", err)
	}

	dimension := cfg.Dimension
	if dimension == 0 {
		dimension = 1536 // default for OpenAI text-embedding-3-small
	}

	return &ZillizAdapter{
		client:     c,
		collection: cfg.Collection,
		dimension:  dimension,
	}, nil
}

// Close closes the Zilliz connection
func (z *ZillizAdapter) Close() error {
	return z.client.Close()
}

// UpsertPathTree stores the complete file tree as a special document
func (z *ZillizAdapter) UpsertPathTree(ctx context.Context, tree vfs.PathTree) error {
	// Serialize PathTree to JSON
	treeJSON, err := json.Marshal(tree)
	if err != nil {
		return fmt.Errorf("failed to marshal PathTree: %w", err)
	}

	// Store as special document with ID "__path_tree__"
	// Schema: id (string), doc_type (string), content (string), embedding (vector)
	// For PathTree: doc_type = "path_tree", content = JSON, embedding = zero vector

	// Create zero embedding (dimension must match collection schema)
	dimension := z.dimension
	zeroVec := make([]float32, dimension)

	// Upsert operation
	_, err = z.client.Upsert(
		ctx,
		z.collection,
		"", // partition name (empty = default)
		entity.NewColumnVarChar("id", []string{"__path_tree__"}),
		entity.NewColumnVarChar("doc_type", []string{"path_tree"}),
		entity.NewColumnVarChar("content", []string{string(treeJSON)}),
		entity.NewColumnFloatVector("embedding", dimension, [][]float32{zeroVec}),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert PathTree: %w", err)
	}

	return nil
}

// GetPathTree retrieves the complete file tree
func (z *ZillizAdapter) GetPathTree(ctx context.Context) (vfs.PathTree, error) {
	// Query for the special __path_tree__ document
	expr := `id == "__path_tree__"`

	result, err := z.client.Query(
		ctx,
		z.collection,
		[]string{}, // partitions (empty = all)
		expr,
		[]string{"content"}, // output fields
	)
	if err != nil {
		return vfs.PathTree{}, fmt.Errorf("failed to query PathTree: %w", err)
	}

	if result.Len() == 0 {
		return vfs.PathTree{}, fmt.Errorf("PathTree not found - run vkfs-admin init first")
	}

	// Extract content field
	contentCol := result.GetColumn("content")
	if contentCol == nil {
		return vfs.PathTree{}, fmt.Errorf("content column not found")
	}

	contentVarChar, ok := contentCol.(*entity.ColumnVarChar)
	if !ok {
		return vfs.PathTree{}, fmt.Errorf("content column is not VarChar type")
	}

	if contentVarChar.Len() == 0 {
		return vfs.PathTree{}, fmt.Errorf("PathTree content is empty")
	}

	treeJSON, err := contentVarChar.ValueByIdx(0)
	if err != nil {
		return vfs.PathTree{}, fmt.Errorf("failed to get content value: %w", err)
	}

	// Deserialize JSON to PathTree
	var tree vfs.PathTree
	if err := json.Unmarshal([]byte(treeJSON), &tree); err != nil {
		return vfs.PathTree{}, fmt.Errorf("failed to unmarshal PathTree: %w", err)
	}

	return tree, nil
}

// GetChunksByPage retrieves all chunks for a file (used for cat)
func (z *ZillizAdapter) GetChunksByPage(ctx context.Context, pageSlug string) ([]vfs.Chunk, error) {
	// Query for all chunks with matching page_slug
	expr := fmt.Sprintf(`page_slug == "%s" && doc_type == "chunk"`, pageSlug)

	result, err := z.client.Query(
		ctx,
		z.collection,
		[]string{}, // partitions
		expr,
		[]string{"id", "page_slug", "chunk_index", "text"}, // output fields
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}

	if result.Len() == 0 {
		return []vfs.Chunk{}, nil
	}

	// Extract fields
	idCol := result.GetColumn("id")
	if idCol == nil {
		return nil, fmt.Errorf("id column not found")
	}
	pageSlugCol := result.GetColumn("page_slug")
	if pageSlugCol == nil {
		return nil, fmt.Errorf("page_slug column not found")
	}
	chunkIndexCol := result.GetColumn("chunk_index")
	if chunkIndexCol == nil {
		return nil, fmt.Errorf("chunk_index column not found")
	}
	textCol := result.GetColumn("text")
	if textCol == nil {
		return nil, fmt.Errorf("text column not found")
	}

	chunks := make([]vfs.Chunk, result.Len())
	for i := 0; i < result.Len(); i++ {
		id, _ := idCol.(*entity.ColumnVarChar).ValueByIdx(i)
		slug, _ := pageSlugCol.(*entity.ColumnVarChar).ValueByIdx(i)
		idx, _ := chunkIndexCol.(*entity.ColumnInt64).ValueByIdx(i)
		text, _ := textCol.(*entity.ColumnVarChar).ValueByIdx(i)

		chunks[i] = vfs.Chunk{
			ID:         id,
			PageSlug:   slug,
			ChunkIndex: int(idx),
			Text:       text,
		}
	}

	return chunks, nil
}

// Stub implementations for remaining interface methods
// TODO: implement in Week 2

func (z *ZillizAdapter) UpsertChunks(ctx context.Context, chunks []vfs.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	n := len(chunks)
	ids := make([]string, n)
	docTypes := make([]string, n)
	pageSlugs := make([]string, n)
	chunkIndices := make([]int64, n)
	texts := make([]string, n)
	embeddings := make([][]float32, n)
	dimension := 1536

	for i, chunk := range chunks {
		ids[i] = chunk.ID
		docTypes[i] = "chunk"
		pageSlugs[i] = chunk.PageSlug
		chunkIndices[i] = int64(chunk.ChunkIndex)
		texts[i] = chunk.Text
		if len(chunk.Embedding) > 0 {
			embeddings[i] = chunk.Embedding
			dimension = len(chunk.Embedding)
		} else {
			embeddings[i] = make([]float32, dimension)
		}
	}

	_, err := z.client.Upsert(
		ctx,
		z.collection,
		"",
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnVarChar("doc_type", docTypes),
		entity.NewColumnVarChar("page_slug", pageSlugs),
		entity.NewColumnInt64("chunk_index", chunkIndices),
		entity.NewColumnVarChar("text", texts),
		entity.NewColumnFloatVector("embedding", dimension, embeddings),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert chunks: %w", err)
	}

	return nil
}

func (z *ZillizAdapter) DeleteChunksByPage(ctx context.Context, pageSlug string) error {
	return fmt.Errorf("not implemented yet")
}

func (z *ZillizAdapter) UpsertLazyPointer(ctx context.Context, pointer vfs.LazyPointer) error {
	return fmt.Errorf("not implemented yet")
}

func (z *ZillizAdapter) GetLazyPointer(ctx context.Context, pageSlug string) (*vfs.LazyPointer, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (z *ZillizAdapter) SearchText(ctx context.Context, pattern string, filter vfs.PathFilter, limit int) ([]vfs.Chunk, error) {
	// Build filter expression
	expr := `doc_type == "chunk"`

	// Add path prefix filter if specified
	if filter.PathPrefix != "" && filter.PathPrefix != "/" {
		expr += fmt.Sprintf(` && page_slug like "%s%%"`, filter.PathPrefix)
	}

	// Perform BM25 text search
	// Note: Milvus 2.4+ supports full-text search with BM25
	// For now, we'll use a simple query and filter in memory
	// TODO: Implement proper BM25 when Milvus SDK supports it

	result, err := z.client.Query(
		ctx,
		z.collection,
		[]string{}, // partitions
		expr,
		[]string{"id", "page_slug", "chunk_index", "text"}, // output fields
		client.WithLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search text: %w", err)
	}

	if result.Len() == 0 {
		return []vfs.Chunk{}, nil
	}

	// Extract fields
	idCol := result.GetColumn("id")
	if idCol == nil {
		return nil, fmt.Errorf("id column not found")
	}
	pageSlugCol := result.GetColumn("page_slug")
	if pageSlugCol == nil {
		return nil, fmt.Errorf("page_slug column not found")
	}
	chunkIndexCol := result.GetColumn("chunk_index")
	if chunkIndexCol == nil {
		return nil, fmt.Errorf("chunk_index column not found")
	}
	textCol := result.GetColumn("text")
	if textCol == nil {
		return nil, fmt.Errorf("text column not found")
	}

	chunks := make([]vfs.Chunk, result.Len())
	for i := 0; i < result.Len(); i++ {
		id, _ := idCol.(*entity.ColumnVarChar).ValueByIdx(i)
		slug, _ := pageSlugCol.(*entity.ColumnVarChar).ValueByIdx(i)
		idx, _ := chunkIndexCol.(*entity.ColumnInt64).ValueByIdx(i)
		text, _ := textCol.(*entity.ColumnVarChar).ValueByIdx(i)

		chunks[i] = vfs.Chunk{
			ID:         id,
			PageSlug:   slug,
			ChunkIndex: int(idx),
			Text:       text,
		}
	}

	return chunks, nil
}

func (z *ZillizAdapter) SearchVector(ctx context.Context, queryVec []float32, filter vfs.PathFilter, topK int) ([]vfs.SearchHit, error) {
	// Build filter expression
	expr := `doc_type == "chunk"`

	// Add path prefix filter if specified
	if filter.PathPrefix != "" && filter.PathPrefix != "/" {
		expr += fmt.Sprintf(` && page_slug like "%s%%"`, filter.PathPrefix)
	}

	// Perform vector search
	sp, err := entity.NewIndexFlatSearchParam()
	if err != nil {
		return nil, fmt.Errorf("failed to create search param: %w", err)
	}

	result, err := z.client.Search(
		ctx,
		z.collection,
		[]string{}, // partitions
		expr,
		[]string{"id", "page_slug", "chunk_index", "text"}, // output fields
		[]entity.Vector{entity.FloatVector(queryVec)},
		"embedding", // vector field name
		entity.L2,   // metric type
		topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search vector: %w", err)
	}

	if len(result) == 0 {
		return []vfs.SearchHit{}, nil
	}

	// Extract results
	hits := make([]vfs.SearchHit, 0)
	for _, resultSet := range result {
		for i := 0; i < resultSet.ResultCount; i++ {
			id, _ := resultSet.IDs.GetAsString(i)
			score := resultSet.Scores[i]

			// Get fields
			fields := resultSet.Fields
			var pageSlug, text string
			var chunkIndex int64

			for _, field := range fields {
				switch field.Name() {
				case "page_slug":
					if col, ok := field.(*entity.ColumnVarChar); ok {
						pageSlug, _ = col.ValueByIdx(i)
					}
				case "chunk_index":
					if col, ok := field.(*entity.ColumnInt64); ok {
						chunkIndex, _ = col.ValueByIdx(i)
					}
				case "text":
					if col, ok := field.(*entity.ColumnVarChar); ok {
						text, _ = col.ValueByIdx(i)
					}
				}
			}

			hits = append(hits, vfs.SearchHit{
				Chunk: vfs.Chunk{
					ID:         id,
					PageSlug:   pageSlug,
					ChunkIndex: int(chunkIndex),
					Text:       text,
				},
				Score: score,
			})
		}
	}

	return hits, nil
}

func (z *ZillizAdapter) SearchHybrid(ctx context.Context, queryVec []float32, pattern string, filter vfs.PathFilter, topK int) ([]vfs.SearchHit, error) {
	return nil, fmt.Errorf("not implemented yet")
}
