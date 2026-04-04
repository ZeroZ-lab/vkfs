package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zhengjianqiao/vkfs/internal/config"
	"github.com/zhengjianqiao/vkfs/pkg/vectorstore"
	"github.com/zhengjianqiao/vkfs/pkg/vfs"
)

var rootCmd = &cobra.Command{
	Use:   "vkfs-admin",
	Short: "VKFS administration tool",
	Long:  `Administrative commands for VKFS (Virtual Knowledge File System)`,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize VKFS - create empty PathTree with root node",
	Long: `Initialize the VKFS knowledge base by creating an empty PathTree
with a root node in the vector database. Run this once before using vkfs commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := config.LoadDefault()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Create vector store adapter
		var store vectorstore.VectorStore
		switch cfg.VectorStore.Backend {
		case "zilliz":
			adapter, err := vectorstore.NewZillizAdapter(vectorstore.ZillizConfig{
				Endpoint:   cfg.VectorStore.Zilliz.Endpoint,
				APIKey:     cfg.VectorStore.Zilliz.APIKey,
				Collection: cfg.VectorStore.Zilliz.Collection,
			})
			if err != nil {
				return fmt.Errorf("failed to create Zilliz adapter: %w", err)
			}
			defer adapter.Close()
			store = adapter
		case "qdrant":
			return fmt.Errorf("Qdrant adapter not implemented yet")
		default:
			return fmt.Errorf("unsupported vectorstore backend: %s", cfg.VectorStore.Backend)
		}

		// Create empty PathTree with root node
		tree := vfs.PathTree{
			Nodes: map[string]vfs.VirtualNode{
				"/": {
					Path:      "/",
					Name:      "/",
					IsDir:     true,
					Size:      0,
					ModTime:   time.Now(),
					VisibleTo: []string{"*"}, // public by default
					Metadata:  make(map[string]string),
				},
			},
			Version: "1.0",
			Metadata: map[string]string{
				"embedding_model": cfg.Embedding.Provider + ":" + getEmbeddingModel(cfg),
				"created_at":      time.Now().Format(time.RFC3339),
			},
		}

		// Store PathTree in vector database
		ctx := context.Background()
		if err := store.UpsertPathTree(ctx, tree); err != nil {
			return fmt.Errorf("failed to initialize PathTree: %w", err)
		}

		fmt.Println("✓ VKFS initialized successfully")
		fmt.Printf("  Backend: %s\n", cfg.VectorStore.Backend)
		fmt.Printf("  Collection: %s\n", getCollectionName(cfg))
		fmt.Printf("  Embedding: %s\n", tree.Metadata["embedding_model"])
		fmt.Println("\nYou can now use vkfs commands:")
		fmt.Println("  vkfs ls /")
		fmt.Println("  vkfs stat /")

		return nil
	},
}

func getEmbeddingModel(cfg *config.Config) string {
	switch cfg.Embedding.Provider {
	case "openai":
		return cfg.Embedding.OpenAI.Model
	case "cohere":
		return cfg.Embedding.Cohere.Model
	default:
		return "unknown"
	}
}

func getCollectionName(cfg *config.Config) string {
	switch cfg.VectorStore.Backend {
	case "zilliz":
		return cfg.VectorStore.Zilliz.Collection
	case "qdrant":
		return cfg.VectorStore.Qdrant.Collection
	default:
		return "unknown"
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
