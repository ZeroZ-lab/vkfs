package vectorstore

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

func newTestSQLiteAdapter(t *testing.T) *SQLiteAdapter {
	t.Helper()
	dir := t.TempDir()
	adapter, err := NewSQLiteAdapter(SQLiteConfig{
		Path:      filepath.Join(dir, "test.db"),
		Dimension: 4,
	})
	require.NoError(t, err)
	t.Cleanup(func() { adapter.Close() })
	return adapter
}

// --- PathTree ---

func TestSQLite_UpsertGetPathTree(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	tree := vfs.PathTree{
		Nodes: map[string]vfs.VirtualNode{
			"/": {Path: "/", Name: "/", IsDir: true},
			"/docs": {Path: "/docs", Name: "docs", IsDir: true},
			"/docs/readme.md": {Path: "/docs/readme.md", Name: "readme.md", IsDir: false, Size: 42},
		},
		Version: "1.0",
	}

	err := s.UpsertPathTree(ctx, tree)
	require.NoError(t, err)

	got, err := s.GetPathTree(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.0", got.Version)
	assert.Len(t, got.Nodes, 3)
	assert.Equal(t, vfs.VirtualNode{Path: "/docs/readme.md", Name: "readme.md", IsDir: false, Size: 42}, got.Nodes["/docs/readme.md"])
}

func TestSQLite_GetPathTree_NotFound(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	_, err := s.GetPathTree(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSQLite_UpsertPathTree_Overwrite(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	tree1 := vfs.PathTree{Version: "1.0", Nodes: map[string]vfs.VirtualNode{"/": {Path: "/", IsDir: true}}}
	require.NoError(t, s.UpsertPathTree(ctx, tree1))

	tree2 := vfs.PathTree{Version: "2.0", Nodes: map[string]vfs.VirtualNode{"/": {Path: "/", IsDir: true}, "/a.md": {Path: "/a.md"}}}
	require.NoError(t, s.UpsertPathTree(ctx, tree2))

	got, err := s.GetPathTree(ctx)
	require.NoError(t, err)
	assert.Equal(t, "2.0", got.Version)
	assert.Len(t, got.Nodes, 2)
}

// --- Chunks ---

func TestSQLite_UpsertGetChunks(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	chunks := []vfs.Chunk{
		{ID: "c0", PageSlug: "/docs/a.md", ChunkIndex: 0, Text: "hello world", Embedding: []float32{0.1, 0.2, 0.3, 0.4}},
		{ID: "c1", PageSlug: "/docs/a.md", ChunkIndex: 1, Text: "goodbye world", Embedding: []float32{0.5, 0.6, 0.7, 0.8}},
		{ID: "c2", PageSlug: "/docs/b.md", ChunkIndex: 0, Text: "other file"},
	}
	require.NoError(t, s.UpsertChunks(ctx, chunks))

	got, err := s.GetChunksByPage(ctx, "/docs/a.md")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, 0, got[0].ChunkIndex)
	assert.Equal(t, 1, got[1].ChunkIndex)
	assert.Equal(t, "hello world", got[0].Text)
}

func TestSQLite_UpsertChunks_Empty(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	err := s.UpsertChunks(ctx, nil)
	assert.NoError(t, err)
}

func TestSQLite_DeleteChunksByPage(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	chunks := []vfs.Chunk{
		{ID: "c0", PageSlug: "/a.md", ChunkIndex: 0, Text: "keep"},
		{ID: "c1", PageSlug: "/b.md", ChunkIndex: 0, Text: "delete me"},
	}
	require.NoError(t, s.UpsertChunks(ctx, chunks))

	require.NoError(t, s.DeleteChunksByPage(ctx, "/b.md"))

	got, err := s.GetChunksByPage(ctx, "/b.md")
	require.NoError(t, err)
	assert.Empty(t, got)

	kept, err := s.GetChunksByPage(ctx, "/a.md")
	require.NoError(t, err)
	assert.Len(t, kept, 1)
}

// --- LazyPointer ---

func TestSQLite_UpsertGetLazyPointer(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	ptr := vfs.LazyPointer{
		PageSlug:    "/large/file.md",
		ExternalURL: "s3://bucket/key",
		Size:        2048000,
	}
	require.NoError(t, s.UpsertLazyPointer(ctx, ptr))

	got, err := s.GetLazyPointer(ctx, "/large/file.md")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "s3://bucket/key", got.ExternalURL)
	assert.Equal(t, int64(2048000), got.Size)
}

func TestSQLite_GetLazyPointer_NotFound(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	got, err := s.GetLazyPointer(ctx, "/nonexistent.md")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// --- SearchText ---

func TestSQLite_SearchText(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	chunks := []vfs.Chunk{
		{ID: "c0", PageSlug: "/docs/a.md", ChunkIndex: 0, Text: "Go language concurrency"},
		{ID: "c1", PageSlug: "/docs/b.md", ChunkIndex: 0, Text: "Python data science"},
		{ID: "c2", PageSlug: "/code/main.go", ChunkIndex: 0, Text: "Go http handler"},
	}
	require.NoError(t, s.UpsertChunks(ctx, chunks))

	// Search for "Go" across all files
	results, err := s.SearchText(ctx, "Go", vfs.PathFilter{}, 10)
	require.NoError(t, err)
	assert.Len(t, results, 2) // matches "Go language" and "Go http"

	// Search with path prefix filter
	results, err = s.SearchText(ctx, "Go", vfs.PathFilter{PathPrefix: "/docs"}, 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "/docs/a.md", results[0].PageSlug)
}

func TestSQLite_SearchText_Limit(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	var chunks []vfs.Chunk
	for i := 0; i < 10; i++ {
		chunks = append(chunks, vfs.Chunk{ID: string(rune('a' + i)), PageSlug: "/f.md", ChunkIndex: i, Text: "keyword match"})
	}
	require.NoError(t, s.UpsertChunks(ctx, chunks))

	results, err := s.SearchText(ctx, "keyword", vfs.PathFilter{}, 3)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

// --- SearchVector ---

func TestSQLite_SearchVector(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	// Insert chunks with known vectors
	chunks := []vfs.Chunk{
		{ID: "c0", PageSlug: "/a.md", ChunkIndex: 0, Text: "close", Embedding: []float32{1.0, 0.0, 0.0, 0.0}},
		{ID: "c1", PageSlug: "/b.md", ChunkIndex: 0, Text: "mid", Embedding: []float32{0.7, 0.7, 0.0, 0.0}},
		{ID: "c2", PageSlug: "/c.md", ChunkIndex: 0, Text: "far", Embedding: []float32{0.0, 0.0, 0.0, 1.0}},
	}
	require.NoError(t, s.UpsertChunks(ctx, chunks))

	queryVec := []float32{1.0, 0.0, 0.0, 0.0}
	hits, err := s.SearchVector(ctx, queryVec, vfs.PathFilter{}, 2)
	require.NoError(t, err)
	assert.Len(t, hits, 2)
	// Closest should be [1,0,0,0] with distance 0
	assert.Equal(t, "c0", hits[0].Chunk.ID)
	assert.InDelta(t, 0.0, hits[0].Score, 1e-6)
	// Second should be [0.7,0.7,0,0] with distance sqrt(0.58) ≈ 0.7616
	assert.Equal(t, "c1", hits[1].Chunk.ID)
	assert.InDelta(t, math.Sqrt(0.58), hits[1].Score, 0.01)
}

func TestSQLite_SearchVector_WithFilter(t *testing.T) {
	s := newTestSQLiteAdapter(t)
	ctx := context.Background()

	chunks := []vfs.Chunk{
		{ID: "c0", PageSlug: "/docs/a.md", ChunkIndex: 0, Text: "a", Embedding: []float32{1, 0, 0, 0}},
		{ID: "c1", PageSlug: "/code/b.go", ChunkIndex: 0, Text: "b", Embedding: []float32{0.9, 0.1, 0, 0}},
	}
	require.NoError(t, s.UpsertChunks(ctx, chunks))

	hits, err := s.SearchVector(ctx, []float32{1, 0, 0, 0}, vfs.PathFilter{PathPrefix: "/docs"}, 5)
	require.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, "/docs/a.md", hits[0].Chunk.PageSlug)
}

// --- Vector encoding helpers ---

func TestEncodeDecodeVector(t *testing.T) {
	original := []float32{1.0, -2.5, 0.0, 3.14, -0.001}
	encoded := encodeVector(original)
	decoded := decodeVector(encoded)
	assert.InDeltaSlice(t, original, decoded, 1e-6)
}

func TestL2Distance(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
	}{
		{"identical", []float32{1, 2, 3}, []float32{1, 2, 3}, 0},
		{"unit distance", []float32{0, 0, 0}, []float32{1, 0, 0}, 1},
		{"mismatched length", []float32{1, 2}, []float32{1}, float32(math.Inf(1))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := l2Distance(tt.a, tt.b)
			if math.IsInf(float64(tt.expected), 1) {
				assert.True(t, math.IsInf(float64(got), 1))
			} else {
				assert.InDelta(t, tt.expected, got, 1e-6)
			}
		})
	}
}

// --- NewSQLiteAdapter edge cases ---

func TestNewSQLiteAdapter_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Test with explicit path (not ~)
	adapter, err := NewSQLiteAdapter(SQLiteConfig{Path: dbPath, Dimension: 4})
	require.NoError(t, err)
	adapter.Close()

	// Verify file exists
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestNewSQLiteAdapter_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nested", "deep", "test.db")

	adapter, err := NewSQLiteAdapter(SQLiteConfig{Path: dbPath, Dimension: 4})
	require.NoError(t, err)
	adapter.Close()

	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

// --- Sorting helper test ---

func TestL2Distance_Sorting(t *testing.T) {
	type item struct {
		id   string
		dist float32
	}
	items := []item{
		{"far", l2Distance([]float32{0, 0}, []float32{10, 10})},
		{"close", l2Distance([]float32{0, 0}, []float32{0.1, 0.1})},
		{"mid", l2Distance([]float32{0, 0}, []float32{1, 1})},
	}
	sort.Slice(items, func(i, j int) bool { return items[i].dist < items[j].dist })
	assert.Equal(t, "close", items[0].id)
	assert.Equal(t, "mid", items[1].id)
	assert.Equal(t, "far", items[2].id)
}
