package vfs

import (
	"context"
	"sort"
	"testing"
	"time"
)

// mockVectorStore is a test double for VectorStore
type mockVectorStore struct {
	tree   PathTree
	chunks map[string][]Chunk
	ptrs   map[string]*LazyPointer
}

func (m *mockVectorStore) UpsertPathTree(_ context.Context, tree PathTree) error {
	m.tree = tree
	return nil
}

func (m *mockVectorStore) GetPathTree(_ context.Context) (PathTree, error) {
	return m.tree, nil
}

func (m *mockVectorStore) GetChunksByPage(_ context.Context, pageSlug string) ([]Chunk, error) {
	return m.chunks[pageSlug], nil
}

func (m *mockVectorStore) GetLazyPointer(_ context.Context, pageSlug string) (*LazyPointer, error) {
	return m.ptrs[pageSlug], nil
}

func (m *mockVectorStore) SearchText(_ context.Context, pattern string, filter PathFilter, limit int) ([]Chunk, error) {
	var result []Chunk
	for _, chunks := range m.chunks {
		for _, chunk := range chunks {
			if filter.PathPrefix != "/" && !contains(chunk.PageSlug, filter.PathPrefix) {
				continue
			}
			if contains(chunk.Text, pattern) {
				result = append(result, chunk)
			}
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockVectorStore) SearchVector(_ context.Context, _ []float32, _ PathFilter, topK int) ([]SearchHit, error) {
	return []SearchHit{}, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

func newTestFS() *VirtualFS {
	tree := PathTree{
		Nodes: map[string]VirtualNode{
			"/": {Path: "/", Name: "/", IsDir: true, ModTime: time.Now()},
			"/docs": {Path: "/docs", Name: "docs", IsDir: true, ModTime: time.Now()},
			"/docs/readme.md": {Path: "/docs/readme.md", Name: "readme.md", IsDir: false, Size: 100, ModTime: time.Now()},
			"/docs/guide.md": {Path: "/docs/guide.md", Name: "guide.md", IsDir: false, Size: 200, ModTime: time.Now()},
			"/code": {Path: "/code", Name: "code", IsDir: true, ModTime: time.Now()},
			"/code/main.go": {Path: "/code/main.go", Name: "main.go", IsDir: false, Size: 50, ModTime: time.Now()},
		},
		Version: "1.0",
	}

	store := &mockVectorStore{
		tree:   tree,
		chunks: make(map[string][]Chunk),
		ptrs:   make(map[string]*LazyPointer),
	}

	fs := NewVirtualFS(store, nil, nil)
	fs.tree = tree
	fs.buildIndexes()

	return fs
}

func TestLs(t *testing.T) {
	fs := newTestFS()

	t.Run("ls root returns immediate children", func(t *testing.T) {
		nodes, err := fs.Ls("/")
		if err != nil {
			t.Fatalf("Ls() error: %v", err)
		}

		names := make([]string, len(nodes))
		for i, n := range nodes {
			names[i] = n.Name
		}
		sort.Strings(names)

		if len(names) != 2 || names[0] != "code" || names[1] != "docs" {
			t.Errorf("Ls('/') = %v, want [code docs]", names)
		}
	})

	t.Run("ls docs returns files in docs", func(t *testing.T) {
		nodes, err := fs.Ls("/docs")
		if err != nil {
			t.Fatalf("Ls() error: %v", err)
		}

		names := make([]string, len(nodes))
		for i, n := range nodes {
			names[i] = n.Name
		}
		sort.Strings(names)

		if len(names) != 2 || names[0] != "guide.md" || names[1] != "readme.md" {
			t.Errorf("Ls('/docs') = %v, want [guide.md readme.md]", names)
		}
	})

	t.Run("ls nonexistent path returns error", func(t *testing.T) {
		_, err := fs.Ls("/nonexistent")
		if err == nil {
			t.Error("Ls('/nonexistent') expected error, got nil")
		}
		if err.Error() != "path not found: /nonexistent" {
			t.Errorf("Ls() error = %v, want 'path not found: /nonexistent'", err)
		}
	})

	t.Run("ls file returns not a directory error", func(t *testing.T) {
		_, err := fs.Ls("/docs/readme.md")
		if err == nil {
			t.Error("Ls('/docs/readme.md') expected error, got nil")
		}
		if err.Error() != "not a directory: /docs/readme.md" {
			t.Errorf("Ls() error = %v, want 'not a directory: /docs/readme.md'", err)
		}
	})
}

func TestStat(t *testing.T) {
	fs := newTestFS()

	t.Run("stat existing file", func(t *testing.T) {
		node, err := fs.Stat("/docs/readme.md")
		if err != nil {
			t.Fatalf("Stat() error: %v", err)
		}

		if node.Name != "readme.md" || node.IsDir {
			t.Errorf("Stat() = %+v, unexpected values", node)
		}
	})

	t.Run("stat nonexistent returns error", func(t *testing.T) {
		_, err := fs.Stat("/nonexistent")
		if err == nil {
			t.Error("Stat('/nonexistent') expected error, got nil")
		}
	})
}

func TestFind(t *testing.T) {
	fs := newTestFS()

	t.Run("find .md files returns markdown files", func(t *testing.T) {
		results, err := fs.Find("/", "*.md")
		if err != nil {
			t.Fatalf("Find() error: %v", err)
		}

		sort.Strings(results)
		if len(results) != 2 || results[0] != "/docs/guide.md" || results[1] != "/docs/readme.md" {
			t.Errorf("Find('/', '*.md') = %v, want [/docs/guide.md /docs/readme.md]", results)
		}
	})

	t.Run("find .go files under /code", func(t *testing.T) {
		results, err := fs.Find("/code", "*.go")
		if err != nil {
			t.Fatalf("Find() error: %v", err)
		}

		if len(results) != 1 || results[0] != "/code/main.go" {
			t.Errorf("Find('/code', '*.go') = %v, want [/code/main.go]", results)
		}
	})
}

func TestCatWithChunks(t *testing.T) {
	fs := newTestFS()
	store := fs.vectorStore.(*mockVectorStore)

	// Ingest 3 chunks for readme.md
	store.chunks["/docs/readme.md"] = []Chunk{
		{ID: "1", PageSlug: "/docs/readme.md", ChunkIndex: 0, Text: "# Readme\n"},
		{ID: "2", PageSlug: "/docs/readme.md", ChunkIndex: 1, Text: "First paragraph.\n"},
		{ID: "3", PageSlug: "/docs/readme.md", ChunkIndex: 2, Text: "Second paragraph.\n"},
	}

	t.Run("cat assembles chunks in order", func(t *testing.T) {
		content, err := fs.Cat(context.Background(), "/docs/readme.md")
		if err != nil {
			t.Fatalf("Cat() error: %v", err)
		}

		want := "# Readme\nFirst paragraph.\nSecond paragraph.\n"
		if content != want {
			t.Errorf("Cat() = %q, want %q", content, want)
		}
	})
}

func TestCatChunkIntegrity(t *testing.T) {
	fs := newTestFS()
	store := fs.vectorStore.(*mockVectorStore)

	t.Run("cat fails on missing chunk", func(t *testing.T) {
		store.chunks["/docs/guide.md"] = []Chunk{
			{ID: "1", PageSlug: "/docs/guide.md", ChunkIndex: 0, Text: "a"},
			{ID: "2", PageSlug: "/docs/guide.md", ChunkIndex: 1, Text: "b"},
			// missing chunk 2
			{ID: "4", PageSlug: "/docs/guide.md", ChunkIndex: 3, Text: "d"},
		}

		_, err := fs.Cat(context.Background(), "/docs/guide.md")
		if err == nil {
			t.Error("Cat() expected error for missing chunk, got nil")
		}
		if err.Error() != "Chunk 2 missing" {
			t.Errorf("Cat() error = %q, want 'Chunk 2 missing'", err.Error())
		}
	})
}

func TestGrepFindsMatches(t *testing.T) {
	fs := newTestFS()
	store := fs.vectorStore.(*mockVectorStore)

	store.chunks["/docs/readme.md"] = []Chunk{
		{ID: "1", PageSlug: "/docs/readme.md", ChunkIndex: 0, Text: "This is about authentication.\nLogin with JWT."},
	}
	store.chunks["/docs/guide.md"] = []Chunk{
		{ID: "2", PageSlug: "/docs/guide.md", ChunkIndex: 0, Text: "No match here.\nNothing relevant."},
	}

	results, err := fs.Grep(context.Background(), "authentication", "/docs")
	if err != nil {
		t.Fatalf("Grep() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Grep() = %d results, want 1", len(results))
	}
	if results[0].Path != "/docs/readme.md" {
		t.Errorf("Grep() path = %q, want '/docs/readme.md'", results[0].Path)
	}
	if results[0].LineNum != 1 {
		t.Errorf("Grep() lineNum = %d, want 1", results[0].LineNum)
	}
}
