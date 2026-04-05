package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ZeroZ-lab/vkfs/benchmarks/recall"
	"github.com/ZeroZ-lab/vkfs/internal/config"
	"github.com/ZeroZ-lab/vkfs/pkg/embedding"
	"github.com/ZeroZ-lab/vkfs/pkg/vectorstore"
	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
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

var ingestCmd = &cobra.Command{
	Use:   "ingest <local-dir> <vkfs-path>",
	Short: "Ingest files from local directory into virtual filesystem",
	Long:  `Read files from a local directory, split into chunks, embed, and store in the vector database.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		localDir := args[0]
		vkfsPath := args[1]

		// Validate local directory
		info, err := os.Stat(localDir)
		if err != nil {
			return fmt.Errorf("failed to stat local directory: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", localDir)
		}

		fs, err := initFS()
		if err != nil {
			return err
		}

		result, err := fs.Ingest(context.Background(), localDir, vkfsPath)
		if err != nil {
			return err
		}

		fmt.Printf("Ingested %d files (%d chunks, %d bytes) into %s\n",
			result.FilesIngested, result.ChunksCreated, result.BytesRead, vkfsPath)

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

var benchCmd = &cobra.Command{
	Use:   "bench --data-dir <path> --config <path>",
	Short: "Run recall accuracy benchmark",
	Long:  `Run recall accuracy benchmark against VKFS using a dataset of documents and queries with ground truth relevance.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, _ := cmd.Flags().GetString("data-dir")
		configPath, _ := cmd.Flags().GetString("config")
		topKStr, _ := cmd.Flags().GetString("top-k")
		searchType, _ := cmd.Flags().GetString("search-type")
		output, _ := cmd.Flags().GetString("output")

		if dataDir == "" {
			return fmt.Errorf("--data-dir is required")
		}
		if configPath == "" {
			return fmt.Errorf("--config is required")
		}

		// Parse top-k
		topKs := []int{1, 3, 5, 10}
		if topKStr != "" {
			parts := strings.Split(topKStr, ",")
			topKs = make([]int, 0, len(parts))
			for _, p := range parts {
				k, err := strconv.Atoi(strings.TrimSpace(p))
				if err != nil {
					return fmt.Errorf("invalid top-k value %q: %w", p, err)
				}
				topKs = append(topKs, k)
			}
		}

		// Parse search type
		st, err := recall.ParseSearchType(searchType)
		if err != nil {
			return err
		}

		// Load config
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// Create and run benchmark
		bench, err := recall.NewBenchmark(recall.BenchmarkConfig{
			DataDir: dataDir,
			Config:  cfg,
			TopK:    topKs,
			Search:  st,

		})
		if err != nil {
			return fmt.Errorf("setup benchmark: %w", err)
		}

		report, err := bench.Run(context.Background())
		if err != nil {
			return fmt.Errorf("run benchmark: %w", err)
		}

		report.PrintSummary(os.Stdout)

		if output != "" {
			if err := report.WriteJSON(output); err != nil {
				return fmt.Errorf("write report: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Report saved to: %s\n", output)
		}

		return nil
	},
}

// initFS initializes VirtualFS with config and vector store
func initFS() (*vfs.VirtualFS, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	embedder, err := embedding.NewFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	store, err := vectorstore.NewFromConfig(cfg, embedder.Dimension())
	if err != nil {
		return nil, err
	}

	fs := vfs.NewVirtualFS(store, nil, embedder)

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
	benchCmd.Flags().String("data-dir", "", "path to dataset directory with corpus.jsonl and queries.jsonl")
	benchCmd.Flags().String("config", "", "path to VKFS config file")
	benchCmd.Flags().String("top-k", "1,3,5,10", "comma-separated K values for metrics")
	benchCmd.Flags().String("search-type", "both", "search type: semantic, text, or both")
	benchCmd.Flags().String("output", "", "output JSON report path")
	benchCmd.Flags().Bool("verbose", false, "show per-query details")

	// Add commands
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(catCmd)
	rootCmd.AddCommand(statCmd)
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(grepCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(benchCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
