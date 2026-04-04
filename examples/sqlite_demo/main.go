// Demo: VKFS end-to-end with SQLite backend (zero external dependencies)
//
// This example shows the complete VKFS workflow:
// 1. Create a VirtualFS with SQLite backend
// 2. Initialize with an empty PathTree
// 3. Ingest sample files
// 4. Run ls, stat, cat, find, grep, search commands
//
// Run:
//
//	go run examples/sqlite_demo/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZeroZ-lab/vkfs/pkg/vectorstore"
	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

// mockEmbedder returns deterministic fake embeddings for demo purposes.
// In production, use embedding.NewFromConfig(cfg) for real embeddings.
type mockEmbedder struct {
	dim int
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, m.dim)
	for i := range text {
		vec[i%m.dim] += float32(text[i]) / 1000.0
	}
	return vec, nil
}

func (m *mockEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := m.Embed(context.Background(), t)
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}

func main() {
	ctx := context.Background()

	// 1. Create temporary SQLite database
	tmpDir, err := os.MkdirTemp("", "vkfs-demo-*")
	if err != nil {
		fatal("create temp dir", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "vkfs.db")
	fmt.Printf("=== VKFS SQLite Demo ===\n")
	fmt.Printf("Database: %s\n\n", dbPath)

	// 2. Create adapters
	dimension := 128 // small for demo; real models use 1024+

	store, err := vectorstore.NewSQLiteAdapter(vectorstore.SQLiteConfig{
		Path:      dbPath,
		Dimension: dimension,
	})
	if err != nil {
		fatal("create SQLite adapter", err)
	}
	defer store.Close()

	embedder := &mockEmbedder{dim: dimension}

	fs := vfs.NewVirtualFS(store, nil, embedder)

	// 3. Initialize PathTree
	fmt.Println("--- Step 1: Init ---")
	tree := vfs.PathTree{
		Nodes: map[string]vfs.VirtualNode{
			"/": {
				Path:      "/",
				Name:      "/",
				IsDir:     true,
				ModTime:   time.Now(),
				VisibleTo: []string{"*"},
				Metadata:  map[string]string{},
			},
		},
		Version:  "1.0",
		Metadata: map[string]string{"created_at": time.Now().Format(time.RFC3339)},
	}
	if err := store.UpsertPathTree(ctx, tree); err != nil {
		fatal("upsert path tree", err)
	}
	if err := fs.Init(ctx); err != nil {
		fatal("init filesystem", err)
	}
	fmt.Println("Initialized empty VKFS\n")

	// 4. Ingest sample files (use the examples/sample_data/ directory)
	sampleDir := filepath.Join("examples", "sample_data")
	if _, err := os.Stat(sampleDir); os.IsNotExist(err) {
		// Try relative to repo root
		sampleDir = filepath.Join("..", "..", "examples", "sample_data")
	}

	if _, err := os.Stat(sampleDir); err == nil {
		fmt.Println("--- Step 2: Ingest ---")
		result, err := fs.Ingest(ctx, sampleDir, "/docs")
		if err != nil {
			fatal("ingest", err)
		}
		fmt.Printf("Ingested %d files (%d chunks, %d bytes)\n\n",
			result.FilesIngested, result.ChunksCreated, result.BytesRead)
	}

	// 5. ls
	fmt.Println("--- Step 3: ls / ---")
	nodes, err := fs.Ls("/")
	if err != nil {
		fatal("ls /", err)
	}
	for _, n := range nodes {
		if n.IsDir {
			fmt.Printf("  %s/\n", n.Name)
		} else {
			fmt.Printf("  %s (%d bytes)\n", n.Name, n.Size)
		}
	}
	fmt.Println()

	// 6. ls /docs
	fmt.Println("--- Step 4: ls /docs ---")
	nodes, err = fs.Ls("/docs")
	if err != nil {
		fatal("ls /docs", err)
	}
	for _, n := range nodes {
		if n.IsDir {
			fmt.Printf("  %s/\n", n.Name)
		} else {
			fmt.Printf("  %s (%d bytes)\n", n.Name, n.Size)
		}
	}
	fmt.Println()

	// 7. stat
	fmt.Println("--- Step 5: stat /docs/readme.md ---")
	node, err := fs.Stat("/docs/readme.md")
	if err != nil {
		fatal("stat", err)
	}
	fmt.Printf("  Path: %s\n", node.Path)
	fmt.Printf("  Type: file\n")
	fmt.Printf("  Size: %d bytes\n", node.Size)
	fmt.Printf("  Modified: %s\n\n", node.ModTime.Format("2006-01-02 15:04:05"))

	// 8. cat
	fmt.Println("--- Step 6: cat /docs/readme.md ---")
	content, err := fs.Cat(ctx, "/docs/readme.md")
	if err != nil {
		fatal("cat", err)
	}
	// Print first 200 chars
	if len(content) > 200 {
		fmt.Printf("  %s...\n\n", content[:200])
	} else {
		fmt.Printf("  %s\n\n", content)
	}

	// 9. find
	fmt.Println("--- Step 7: find / -name '*.md' ---")
	results, err := fs.Find("/", "*.md")
	if err != nil {
		fatal("find", err)
	}
	for _, r := range results {
		fmt.Printf("  %s\n", r)
	}
	fmt.Println()

	// 10. grep
	fmt.Println("--- Step 8: grep 'vector' /docs ---")
	grepResults, err := fs.Grep(ctx, "vector", "/docs")
	if err != nil {
		fatal("grep", err)
	}
	for _, r := range grepResults {
		trimmed := strings.TrimSpace(r.Line)
		if trimmed != "" {
			fmt.Printf("  %s:%d — %s\n", r.Path, r.LineNum, trimmed)
		}
	}
	fmt.Println()

	// 11. search (semantic)
	fmt.Println("--- Step 9: search 'how does search work' /docs ---")
	queryVec, _ := embedder.Embed(ctx, "how does search work")
	hits, err := fs.Search(ctx, "how does search work", "/docs", 3)
	if err != nil {
		fatal("search", err)
	}
	_ = queryVec // avoid unused var
	for _, h := range hits {
		fmt.Printf("  %s (score: %.4f)\n", h.Chunk.PageSlug, h.Score)
		preview := strings.Split(h.Chunk.Text, "\n")[0]
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		fmt.Printf("    %s\n", preview)
	}
	fmt.Println()

	// 12. Verify persistence: reload from DB
	fmt.Println("--- Step 10: Verify persistence ---")
	store2, err := vectorstore.NewSQLiteAdapter(vectorstore.SQLiteConfig{
		Path:      dbPath,
		Dimension: dimension,
	})
	if err != nil {
		fatal("reopen SQLite", err)
	}
	defer store2.Close()

	fs2 := vfs.NewVirtualFS(store2, nil, embedder)
	if err := fs2.Init(ctx); err != nil {
		fatal("re-init filesystem", err)
	}

	nodes2, err := fs2.Ls("/docs")
	if err != nil {
		fatal("ls after reload", err)
	}
	fmt.Printf("  Reloaded from DB: %d files in /docs\n", len(nodes2))

	fmt.Println("\n=== Demo complete ===")
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", msg, err)
	os.Exit(1)
}
