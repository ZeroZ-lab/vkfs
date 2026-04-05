package recall

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// SearchType controls which search modes to benchmark.
type SearchType int

const (
	SearchTypeSemantic SearchType = iota
	SearchTypeText
	SearchTypeBoth
)

func (s SearchType) String() string {
	switch s {
	case SearchTypeSemantic:
		return "semantic"
	case SearchTypeText:
		return "text"
	case SearchTypeBoth:
		return "both"
	default:
		return "unknown"
	}
}

// ParseSearchType converts a string to SearchType.
func ParseSearchType(s string) (SearchType, error) {
	switch s {
	case "semantic":
		return SearchTypeSemantic, nil
	case "text":
		return SearchTypeText, nil
	case "both":
		return SearchTypeBoth, nil
	default:
		return 0, fmt.Errorf("invalid search type %q (must be semantic, text, or both)", s)
	}
}

// SearchReport contains aggregated and per-query results for one search mode.
type SearchReport struct {
	NumQueries int                  `json:"num_queries"`
	Aggregated map[string]float64   `json:"aggregated"`
	PerQuery   []QueryReport        `json:"per_query"`
}

// QueryReport contains results for a single query.
type QueryReport struct {
	QueryID      string             `json:"query_id"`
	Query        string             `json:"query"`
	RelevantIDs  []string           `json:"relevant_ids"`
	RetrievedIDs []string           `json:"retrieved_ids"`
	Metrics      map[string]float64 `json:"metrics"`
}

// Report is the full benchmark report.
type Report struct {
	Timestamp time.Time    `json:"timestamp"`
	Dataset   string       `json:"dataset"`
	NumDocs   int          `json:"num_docs"`
	TopK      []int        `json:"top_k"`
	Semantic  *SearchReport `json:"semantic,omitempty"`
	Text      *SearchReport `json:"text,omitempty"`
}

// PrintSummary writes a human-readable summary to w.
func (r *Report) PrintSummary(w io.Writer) {
	fmt.Fprintf(w, "=== VKFS Recall Benchmark Report ===\n")
	fmt.Fprintf(w, "Timestamp: %s\n", r.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(w, "Dataset:   %s (%d docs)\n", r.Dataset, r.NumDocs)
	fmt.Fprintf(w, "Top-K:     %v\n\n", r.TopK)

	if r.Semantic != nil {
		printSearchReport(w, "Semantic Search", r.Semantic, r.TopK)
	}
	if r.Text != nil {
		printSearchReport(w, "Text Search", r.Text, r.TopK)
	}
}

func printSearchReport(w io.Writer, name string, sr *SearchReport, topKs []int) {
	fmt.Fprintf(w, "--- %s (%d queries) ---\n", name, sr.NumQueries)
	for _, k := range topKs {
		key := fmt.Sprintf("recall@%d", k)
		fmt.Fprintf(w, "  Recall@%-3d: %.3f\n", k, sr.Aggregated[key])
	}
	for _, k := range topKs {
		key := fmt.Sprintf("precision@%d", k)
		fmt.Fprintf(w, "  Precision@%-2d: %.3f\n", k, sr.Aggregated[key])
	}
	fmt.Fprintf(w, "  MRR:     %.3f\n", sr.Aggregated["mrr"])
	for _, k := range topKs {
		key := fmt.Sprintf("ndcg@%d", k)
		fmt.Fprintf(w, "  NDCG@%-3d: %.3f\n", k, sr.Aggregated[key])
	}
	fmt.Fprintln(w)
}

// WriteJSON saves the full report as JSON to the given path.
func (r *Report) WriteJSON(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// ComputeQueryMetrics calculates all metrics for a single query result.
func ComputeQueryMetrics(result QueryResult, topKs []int) map[string]float64 {
	m := make(map[string]float64, len(topKs)*3+1)
	for _, k := range topKs {
		m[fmt.Sprintf("recall@%d", k)] = RecallAtK(result, k)
		m[fmt.Sprintf("precision@%d", k)] = PrecisionAtK(result, k)
		m[fmt.Sprintf("ndcg@%d", k)] = NDCGAtK(result, k)
	}
	m["mrr"] = MRR(result)
	return m
}

// AggregateMetrics computes mean metrics across per-query reports.
func AggregateMetrics(perQuery []QueryReport) map[string]float64 {
	if len(perQuery) == 0 {
		return map[string]float64{}
	}
	sums := make(map[string]float64)
	for _, qr := range perQuery {
		for k, v := range qr.Metrics {
			sums[k] += v
		}
	}
	n := float64(len(perQuery))
	avg := make(map[string]float64, len(sums))
	for k, v := range sums {
		avg[k] = v / n
	}
	return avg
}
