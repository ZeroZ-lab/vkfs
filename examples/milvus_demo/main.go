// Demo: VKFS with Milvus Cloud (gRPC) + SiliconFlow embeddings
//
// Prerequisites:
//   - Milvus Cloud instance (e.g. Zilliz Cloud Dedicated, or any cloud Milvus with gRPC)
//   - Set MILVUS_ENDPOINT, MILVUS_API_KEY, SILICONFLOW_API_KEY env vars
//
// Zilliz Cloud Dedicated example:
//   endpoint format: "in03-xxxxx.aws-us-east-1.cloud.zilliz.com:19530"
//   (Dedicated instances expose gRPC port 19530, Serverless does not)
//
// Usage:
//
//	export MILVUS_ENDPOINT="in03-xxxxx.aws-us-east-1.cloud.zilliz.com:19530"
//	export MILVUS_API_KEY="your-api-key"
//	export SILICONFLOW_API_KEY="your-siliconflow-key"
//	go run examples/milvus_demo/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/zhengjianqiao/vkfs/pkg/embedding"
	"github.com/zhengjianqiao/vkfs/pkg/vectorstore"
	"github.com/zhengjianqiao/vkfs/pkg/vfs"
)

func main() {
	ctx := context.Background()

	// Configuration from env vars
	milvusEndpoint := os.Getenv("MILVUS_ENDPOINT")
	if milvusEndpoint == "" {
		fmt.Fprintln(os.Stderr, "Set MILVUS_ENDPOINT (e.g. in03-xxxxx.aws-us-east-1.cloud.zilliz.com:19530)")
		os.Exit(1)
	}
	milvusAPIKey := os.Getenv("MILVUS_API_KEY")
	collection := os.Getenv("MILVUS_COLLECTION")
	if collection == "" {
		collection = "vkfs_demo"
	}

	sfAPIKey := os.Getenv("SILICONFLOW_API_KEY")
	if sfAPIKey == "" {
		fmt.Fprintln(os.Stderr, "Set SILICONFLOW_API_KEY")
		os.Exit(1)
	}

	fmt.Println("=== VKFS Milvus Cloud Demo ===")
	fmt.Printf("Endpoint:   %s\n", milvusEndpoint)
	fmt.Printf("Collection: %s\n\n", collection)

	// Create embedding provider
	embedder := embedding.NewOpenAIProvider(sfAPIKey, "BAAI/bge-m3")
	embedder.WithBaseURL("https://api.siliconflow.cn/v1/embeddings")
	dimension := embedder.Dimension()
	fmt.Printf("Embedding: BAAI/bge-m3 (dim=%d)\n\n", dimension)

	// Create Milvus adapter (gRPC)
	store, err := vectorstore.NewZillizAdapter(vectorstore.ZillizConfig{
		Endpoint:   milvusEndpoint,
		APIKey:     milvusAPIKey,
		Collection: collection,
		Dimension:  dimension,
	})
	if err != nil {
		fatal("connect to Milvus Cloud", err)
	}
	defer store.Close()

	// Initialize PathTree
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

	fs := vfs.NewVirtualFS(store, nil, embedder)
	if err := fs.Init(ctx); err != nil {
		fatal("init filesystem", err)
	}
	fmt.Println("PathTree initialized\n")

	// Ingest sample data
	sampleDir := "examples/sample_data"
	if _, err := os.Stat(sampleDir); os.IsNotExist(err) {
		sampleDir = "../../examples/sample_data"
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

	// ls
	fmt.Println("--- Step 3: ls / ---")
	nodes, err := fs.Ls("/")
	if err != nil {
		fatal("ls", err)
	}
	for _, n := range nodes {
		if n.IsDir {
			fmt.Printf("  %s/\n", n.Name)
		} else {
			fmt.Printf("  %s\n", n.Name)
		}
	}
	fmt.Println()

	// ls /docs
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

	// cat
	fmt.Println("--- Step 5: cat /docs/readme.md ---")
	content, err := fs.Cat(ctx, "/docs/readme.md")
	if err != nil {
		fatal("cat", err)
	}
	if len(content) > 200 {
		fmt.Printf("  %s...\n\n", content[:200])
	} else {
		fmt.Printf("  %s\n\n", content)
	}

	// find
	fmt.Println("--- Step 6: find / -name '*.md' ---")
	results, err := fs.Find("/", "*.md")
	if err != nil {
		fatal("find", err)
	}
	for _, r := range results {
		fmt.Printf("  %s\n", r)
	}
	fmt.Println()

	// search
	fmt.Println("--- Step 7: search 'file system architecture' /docs ---")
	hits, err := fs.Search(ctx, "file system architecture", "/docs", 3)
	if err != nil {
		fatal("search", err)
	}
	for _, h := range hits {
		fmt.Printf("  %s (score: %.4f)\n", h.Chunk.PageSlug, h.Score)
	}
	if len(hits) == 0 {
		fmt.Println("  (no results)")
	}

	fmt.Println("\n=== Done ===")
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", msg, err)
	os.Exit(1)
}
