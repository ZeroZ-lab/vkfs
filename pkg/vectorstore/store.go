package vectorstore

import (
	"context"

	"github.com/zhengjianqiao/vkfs/pkg/vfs"
)

// VectorStore is the core interface for vector database operations.
// The vector database is the single source of truth for file tree, chunks, and metadata.
type VectorStore interface {
	// ===== PathTree Operations =====
	// UpsertPathTree stores the complete file tree (special document, like Mintlify's __path_tree__)
	UpsertPathTree(ctx context.Context, tree vfs.PathTree) error

	// GetPathTree retrieves the complete file tree (loaded into memory at startup)
	GetPathTree(ctx context.Context) (vfs.PathTree, error)

	// ===== Chunk Operations (regular files) =====
	// UpsertChunks batch inserts chunks (used for ingest)
	UpsertChunks(ctx context.Context, chunks []vfs.Chunk) error

	// GetChunksByPage retrieves all chunks for a file (used for cat)
	GetChunksByPage(ctx context.Context, pageSlug string) ([]vfs.Chunk, error)

	// DeleteChunksByPage deletes all chunks for a file
	DeleteChunksByPage(ctx context.Context, pageSlug string) error

	// ===== LazyPointer Operations (large files) =====
	// UpsertLazyPointer stores external storage reference for large files
	UpsertLazyPointer(ctx context.Context, pointer vfs.LazyPointer) error

	// GetLazyPointer checks if a file is a lazy pointer
	GetLazyPointer(ctx context.Context, pageSlug string) (*vfs.LazyPointer, error)

	// ===== Search Operations =====
	// SearchText performs text search (BM25 or full-text index, used for grep)
	SearchText(ctx context.Context, pattern string, filter vfs.PathFilter, limit int) ([]vfs.Chunk, error)

	// SearchVector performs vector search (semantic search, used for search command)
	SearchVector(ctx context.Context, queryVec []float32, filter vfs.PathFilter, topK int) ([]vfs.SearchHit, error)

	// SearchHybrid performs hybrid search (text + vector)
	SearchHybrid(ctx context.Context, queryVec []float32, pattern string, filter vfs.PathFilter, topK int) ([]vfs.SearchHit, error)
}
