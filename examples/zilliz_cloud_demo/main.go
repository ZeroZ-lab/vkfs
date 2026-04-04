// Demo: VKFS with Zilliz Cloud backend + SiliconFlow embeddings
//
// Prerequisites:
//   - Set ZILLIZ_API_KEY and SILICONFLOW_API_KEY env vars
//   - Run `vkfs-admin init` once to create the collection
//
// Usage:
//
//	export ZILLIZ_API_KEY="your-key"
//	export SILICONFLOW_API_KEY="your-key"
//	go run examples/zilliz_cloud_demo/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ZeroZ-lab/vkfs/internal/config"
	"github.com/ZeroZ-lab/vkfs/pkg/embedding"
	"github.com/ZeroZ-lab/vkfs/pkg/vectorstore"
	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

func main() {
	ctx := context.Background()

	// Load config from ~/.vkfs/config.yaml
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Create ~/.vkfs/config.yaml first (see examples/config_example.yaml)")
		os.Exit(1)
	}

	// Create embedding provider (auto-detects dimension)
	embedder, err := embedding.NewFromConfig(cfg)
	if err != nil {
		fatal("create embedder", err)
	}

	// Create vector store (auto-detects dimension from embedder)
	store, err := vectorstore.NewFromConfig(cfg, embedder.Dimension())
	if err != nil {
		fatal("create vector store", err)
	}
	// Create and init VirtualFS
	fs := vfs.NewVirtualFS(store, nil, embedder)
	if err := fs.Init(ctx); err != nil {
		fatal("init filesystem", err)
	}

	// ls /
	fmt.Println("=== VKFS Zilliz Cloud Demo ===\n")

	fmt.Println("--- ls / ---")
	nodes, err := fs.Ls("/")
	if err != nil {
		fatal("ls", err)
	}
	if len(nodes) == 0 {
		fmt.Println("  (empty - run vkfs-admin init and vkfs ingest)")
	}
	for _, n := range nodes {
		if n.IsDir {
			fmt.Printf("  %s/\n", n.Name)
		} else {
			fmt.Printf("  %s\n", n.Name)
		}
	}

	// Semantic search example
	fmt.Println("\n--- search 'virtual filesystem' / ---")
	hits, err := fs.Search(ctx, "virtual filesystem", "/", 5)
	if err != nil {
		fmt.Printf("  Search error (no data?): %v\n", err)
	} else {
		for _, h := range hits {
			fmt.Printf("  %s (score: %.4f)\n", h.Chunk.PageSlug, h.Score)
		}
		if len(hits) == 0 {
			fmt.Println("  (no results - ingest some files first)")
		}
	}

	fmt.Println("\n=== Done ===")
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", msg, err)
	os.Exit(1)
}
