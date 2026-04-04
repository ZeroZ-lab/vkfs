package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zhengjianqiao/vkfs/internal/config"
	"github.com/zhengjianqiao/vkfs/pkg/embedding"
	"github.com/zhengjianqiao/vkfs/pkg/vectorstore"
	"github.com/zhengjianqiao/vkfs/pkg/vfs"
)

var rootCmd = &cobra.Command{
	Use:   "vkfs",
	Short: "Virtual Knowledge File System - filesystem interface to vector databases",
	Long: `VKFS provides Unix-like filesystem commands (ls, cat, grep, find)
over vector databases (Zilliz/Qdrant) for AI agents.`,
}

var lsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List directory contents",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/"
		if len(args) > 0 {
			path = args[0]
		}

		fs, err := initFS()
		if err != nil {
			return err
		}

		nodes, err := fs.Ls(path)
		if err != nil {
			return err
		}

		// Print results
		for _, node := range nodes {
			if node.IsDir {
				fmt.Printf("%s/\n", node.Name)
			} else {
				fmt.Printf("%s\n", node.Name)
			}
		}

		return nil
	},
}

var statCmd = &cobra.Command{
	Use:   "stat <path>",
	Short: "Display file or directory information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		fs, err := initFS()
		if err != nil {
			return err
		}

		node, err := fs.Stat(path)
		if err != nil {
			return err
		}

		// Print node info
		fmt.Printf("Path: %s\n", node.Path)
		fmt.Printf("Name: %s\n", node.Name)
		if node.IsDir {
			fmt.Printf("Type: directory\n")
		} else {
			fmt.Printf("Type: file\n")
			fmt.Printf("Size: %d bytes\n", node.Size)
		}
		fmt.Printf("Modified: %s\n", node.ModTime.Format("2006-01-02 15:04:05"))

		return nil
	},
}

var catCmd = &cobra.Command{
	Use:   "cat <path>",
	Short: "Display file contents",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		fs, err := initFS()
		if err != nil {
			return err
		}

		content, err := fs.Cat(context.Background(), path)
		if err != nil {
			return err
		}

		fmt.Print(content)
		return nil
	},
}

var findCmd = &cobra.Command{
	Use:   "find <path> -name <pattern>",
	Short: "Search for files matching a pattern",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("path required")
		}

		path := args[0]
		pattern, _ := cmd.Flags().GetString("name")
		if pattern == "" {
			return fmt.Errorf("-name flag required")
		}

		fs, err := initFS()
		if err != nil {
			return err
		}

		results, err := fs.Find(path, pattern)
		if err != nil {
			return err
		}

		// Print results
		for _, result := range results {
			fmt.Println(result)
		}

		return nil
	},
}

var grepCmd = &cobra.Command{
	Use:   "grep <pattern> <path>",
	Short: "Search for pattern in files",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]
		path := args[1]

		fs, err := initFS()
		if err != nil {
			return err
		}

		results, err := fs.Grep(context.Background(), pattern, path)
		if err != nil {
			return err
		}

		// Print results in format: path:line — matched text
		for _, result := range results {
			fmt.Printf("%s:%d — %s\n", result.Path, result.LineNum, result.Line)
		}

		return nil
	},
}

var searchCmd = &cobra.Command{
	Use:   "search <query> <path>",
	Short: "Semantic search for query in files",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		path := args[1]

		topK, _ := cmd.Flags().GetInt("top-k")

		fs, err := initFS()
		if err != nil {
			return err
		}

		results, err := fs.Search(context.Background(), query, path, topK)
		if err != nil {
			return err
		}

		// Print results with scores
		for _, result := range results {
			fmt.Printf("%s (score: %.4f)\n", result.Chunk.PageSlug, result.Score)
			fmt.Printf("  %s\n\n", result.Chunk.Text)
		}

		return nil
	},
}

// initFS initializes VirtualFS with config and vector store
func initFS() (*vfs.VirtualFS, error) {
	// Load config
	cfg, err := config.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create vector store adapter
	var store vfs.VectorStore
	switch cfg.VectorStore.Backend {
	case "zilliz":
		adapter, err := vectorstore.NewZillizAdapter(vectorstore.ZillizConfig{
			Endpoint:   cfg.VectorStore.Zilliz.Endpoint,
			APIKey:     cfg.VectorStore.Zilliz.APIKey,
			Collection: cfg.VectorStore.Zilliz.Collection,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Zilliz adapter: %w", err)
		}
		store = adapter
	case "qdrant":
		return nil, fmt.Errorf("Qdrant adapter not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported vectorstore backend: %s", cfg.VectorStore.Backend)
	}

	// Create embedding provider
	var embedder vfs.EmbeddingProvider
	switch cfg.Embedding.Provider {
	case "openai":
		embedder = embedding.NewOpenAIProvider(cfg.Embedding.OpenAI.APIKey, cfg.Embedding.OpenAI.Model)
	case "cohere":
		embedder = embedding.NewCohereProvider(cfg.Embedding.Cohere.APIKey, cfg.Embedding.Cohere.Model)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Embedding.Provider)
	}

	// Create VirtualFS
	fs := vfs.NewVirtualFS(store, nil, embedder) // nil external store for now

	// Initialize (load PathTree from vector DB)
	ctx := context.Background()
	if err := fs.Init(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize VirtualFS: %w", err)
	}

	return fs, nil
}

func init() {
	// Add flags
	findCmd.Flags().StringP("name", "n", "", "filename pattern (glob)")
	searchCmd.Flags().Int("top-k", 10, "number of results to return")

	// Add commands
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(catCmd)
	rootCmd.AddCommand(statCmd)
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(grepCmd)
	rootCmd.AddCommand(searchCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
