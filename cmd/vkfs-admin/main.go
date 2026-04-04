package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/ZeroZ-lab/vkfs/internal/config"
	"github.com/ZeroZ-lab/vkfs/pkg/embedding"
	"github.com/ZeroZ-lab/vkfs/pkg/vectorstore"
	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
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
		cfg, err := config.LoadDefault()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		embedder, err := embedding.NewFromConfig(cfg)
		if err != nil {
			return err
		}

		store, err := vectorstore.NewFromConfig(cfg, embedder.Dimension())
		if err != nil {
			return err
		}

		tree := vfs.PathTree{
			Nodes: map[string]vfs.VirtualNode{
				"/": {
					Path:      "/",
					Name:      "/",
					IsDir:     true,
					Size:      0,
					ModTime:   time.Now(),
					VisibleTo: []string{"*"},
					Metadata:  make(map[string]string),
				},
			},
			Version: "1.0",
			Metadata: map[string]string{
				"embedding_model": cfg.Embedding.Provider + ":" + getEmbeddingModel(cfg),
				"created_at":      time.Now().Format(time.RFC3339),
			},
		}

		ctx := context.Background()
		if err := store.UpsertPathTree(ctx, tree); err != nil {
			return fmt.Errorf("failed to initialize PathTree: %w", err)
		}

		fmt.Println("VKFS initialized successfully")
		fmt.Printf("  Backend: %s\n", cfg.VectorStore.Backend)
		fmt.Printf("  Collection: %s\n", cfg.VectorStore.Zilliz.Collection)
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
	case "siliconflow":
		return cfg.Embedding.SiliconFlow.Model
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
