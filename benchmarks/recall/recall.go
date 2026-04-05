package recall

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZeroZ-lab/vkfs/internal/config"
	"github.com/ZeroZ-lab/vkfs/pkg/embedding"
	"github.com/ZeroZ-lab/vkfs/pkg/vectorstore"
	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

// BenchmarkConfig configures the recall benchmark.
type BenchmarkConfig struct {
	DataDir string
	Config  *config.Config
	TopK    []int
	Search  SearchType
}

// Benchmark runs recall accuracy tests against a VKFS instance.
type Benchmark struct {
	cfg     BenchmarkConfig
	corpus  []CorpusDoc
	queries []Query
}

// NewBenchmark creates a new benchmark instance.
func NewBenchmark(cfg BenchmarkConfig) (*Benchmark, error) {
	if len(cfg.TopK) == 0 {
		return nil, fmt.Errorf("TopK must not be empty")
	}
	for i := 1; i < len(cfg.TopK); i++ {
		if cfg.TopK[i] <= cfg.TopK[i-1] {
			return nil, fmt.Errorf("TopK values must be ascending")
		}
	}

	corpus, err := LoadCorpus(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("load corpus: %w", err)
	}
	queries, err := LoadQueries(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("load queries: %w", err)
	}
	if err := ValidateDataset(corpus, queries); err != nil {
		return nil, fmt.Errorf("validate dataset: %w", err)
	}
	return &Benchmark{cfg: cfg, corpus: corpus, queries: queries}, nil
}

// Run executes the benchmark and returns a report.
func (b *Benchmark) Run(ctx context.Context) (*Report, error) {
	// Create temp dir for corpus files and SQLite DB
	tmpDir, err := os.MkdirTemp("", "vkfs-bench-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write corpus documents as individual files
	corpusDir := filepath.Join(tmpDir, "corpus")
	if err := os.MkdirAll(corpusDir, 0755); err != nil {
		return nil, fmt.Errorf("create corpus dir: %w", err)
	}
	for _, doc := range b.corpus {
		filename := doc.DocID + ".txt"
		content := doc.Text
		if doc.Title != "" {
			content = doc.Title + "\n\n" + content
		}
		if err := os.WriteFile(filepath.Join(corpusDir, filename), []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("write corpus file %s: %w", filename, err)
		}
	}

	// Override SQLite path to use temp dir
	benchCfg := *b.cfg.Config
	benchCfg.VectorStore.SQLite.Path = filepath.Join(tmpDir, "vkfs.db")

	// Create embedding provider
	embedder, err := embedding.NewFromConfig(&benchCfg)
	if err != nil {
		return nil, fmt.Errorf("create embedder: %w", err)
	}

	// Get dimension from embedder (handles negative values from e.g. Ollama)
	dim := embedder.Dimension()
	if dim < 0 {
		dim = 0
	}
	store, err := vectorstore.NewFromConfig(&benchCfg, dim)
	if err != nil {
		return nil, fmt.Errorf("create vector store: %w", err)
	}

	// Close store on exit
	defer func() {
		if closer, ok := store.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	// Seed empty PathTree for fresh DB
	if err := store.UpsertPathTree(ctx, vfs.PathTree{Nodes: make(map[string]vfs.VirtualNode)}); err != nil {
		return nil, fmt.Errorf("seed empty PathTree: %w", err)
	}

	fs := vfs.NewVirtualFS(store, nil, embedder)
	if err := fs.Init(ctx); err != nil {
		return nil, fmt.Errorf("init VirtualFS: %w", err)
	}

	// Ingest corpus
	if _, err := fs.Ingest(ctx, corpusDir, "/docs"); err != nil {
		return nil, fmt.Errorf("ingest corpus: %w", err)
	}

	// Run queries
	maxK := b.cfg.TopK[len(b.cfg.TopK)-1]

	report := &Report{
		Timestamp: time.Now(),
		Dataset:   b.cfg.DataDir,
		NumDocs:   len(b.corpus),
		TopK:      b.cfg.TopK,
	}

	if b.cfg.Search == SearchTypeSemantic || b.cfg.Search == SearchTypeBoth {
		sr, err := b.runSemanticSearch(ctx, fs, maxK)
		if err != nil {
			return nil, fmt.Errorf("semantic search: %w", err)
		}
		report.Semantic = sr
	}

	if b.cfg.Search == SearchTypeText || b.cfg.Search == SearchTypeBoth {
		sr, err := b.runTextSearch(ctx, fs, maxK)
		if err != nil {
			return nil, fmt.Errorf("text search: %w", err)
		}
		report.Text = sr
	}

	return report, nil
}

func (b *Benchmark) runSemanticSearch(ctx context.Context, fs *vfs.VirtualFS, maxK int) (*SearchReport, error) {
	retriever := func(q Query) ([]string, error) {
		hits, err := fs.Search(ctx, q.Query, "/docs", maxK)
		if err != nil {
			return nil, err
		}
		return extractDocIDs(hits, "/docs/"), nil
	}
	return b.runQueries(retriever)
}

func (b *Benchmark) runTextSearch(ctx context.Context, fs *vfs.VirtualFS, maxK int) (*SearchReport, error) {
	// Note: Grep() caps BM25 candidates at 50 (fs.go:359).
	// Results are path-ordered, not relevance-ordered, so position-sensitive
	// metrics (MRR, NDCG) may be misleading for text search.
	retriever := func(q Query) ([]string, error) {
		grepResults, err := fs.Grep(ctx, q.Query, "/docs")
		if err != nil {
			return nil, err
		}
		return extractDocIDsFromPaths(grepResults, "/docs/"), nil
	}
	return b.runQueries(retriever)
}

// retriever extracts deduplicated doc IDs for a single query.
type retriever func(q Query) (retrieved []string, err error)

// runQueries evaluates all queries using the given retriever and builds a SearchReport.
func (b *Benchmark) runQueries(retrieve retriever) (*SearchReport, error) {
	perQuery := make([]QueryReport, 0, len(b.queries))

	for _, q := range b.queries {
		retrieved, err := retrieve(q)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", q.QueryID, err)
		}

		perQuery = append(perQuery, QueryReport{
			QueryID:      q.QueryID,
			Query:        q.Query,
			RelevantIDs:  q.RelevantDocIDs,
			RetrievedIDs: retrieved,
			Metrics:      ComputeQueryMetrics(QueryResult{
				QueryID:      q.QueryID,
				RetrievedIDs: retrieved,
				RelevantIDs:  q.RelevantDocIDs,
			}, b.cfg.TopK),
		})
	}

	return &SearchReport{
		NumQueries: len(perQuery),
		Aggregated: AggregateMetrics(perQuery),
		PerQuery:   perQuery,
	}, nil
}

// extractDocIDs gets deduplicated doc IDs from SearchHit results.
func extractDocIDs(hits []vfs.SearchHit, prefix string) []string {
	ids := make([]string, 0, len(hits))
	seen := make(map[string]bool, len(hits))
	for _, hit := range hits {
		if docID := pathToDocID(hit.Chunk.PageSlug, prefix); docID != "" && !seen[docID] {
			seen[docID] = true
			ids = append(ids, docID)
		}
	}
	return ids
}

// extractDocIDsFromPaths gets deduplicated doc IDs from GrepResult paths.
func extractDocIDsFromPaths(results []vfs.GrepResult, prefix string) []string {
	ids := make([]string, 0, len(results))
	seen := make(map[string]bool, len(results))
	for _, gr := range results {
		if docID := pathToDocID(gr.Path, prefix); docID != "" && !seen[docID] {
			seen[docID] = true
			ids = append(ids, docID)
		}
	}
	return ids
}

// pathToDocID converts a VKFS path like "/docs/doc-001.txt" to doc ID "doc-001".
func pathToDocID(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	name := strings.TrimPrefix(path, prefix)
	name = strings.TrimSuffix(name, ".txt")
	name = strings.TrimSuffix(name, ".md")
	return name
}
