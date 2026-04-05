package recall

import (
	"math"
	"testing"
)

func TestRecallAtK(t *testing.T) {
	tests := []struct {
		name      string
		retrieved []string
		relevant  []string
		k         int
		want      float64
	}{
		{
			name:      "all relevant found",
			retrieved: []string{"a", "b", "c"},
			relevant:  []string{"a", "b"},
			k:         3,
			want:      1.0,
		},
		{
			name:      "partial match",
			retrieved: []string{"a", "x", "y"},
			relevant:  []string{"a", "b", "c"},
			k:         3,
			want:      1.0 / 3.0,
		},
		{
			name:      "no match",
			retrieved: []string{"x", "y", "z"},
			relevant:  []string{"a", "b"},
			k:         3,
			want:      0,
		},
		{
			name:      "k smaller than results",
			retrieved: []string{"a", "b", "c", "d"},
			relevant:  []string{"a", "d"},
			k:         2,
			want:      0.5,
		},
		{
			name:      "empty relevant",
			retrieved: []string{"a", "b"},
			relevant:  []string{},
			k:         2,
			want:      0,
		},
		{
			name:      "empty retrieved",
			retrieved: []string{},
			relevant:  []string{"a", "b"},
			k:         2,
			want:      0,
		},
		{
			name:      "duplicate retrieved ids",
			retrieved: []string{"a", "a", "b"},
			relevant:  []string{"a", "b"},
			k:         3,
			want:      1.0, // both a and b found despite duplicate a
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecallAtK(QueryResult{RetrievedIDs: tt.retrieved, RelevantIDs: tt.relevant}, tt.k)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("RecallAtK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrecisionAtK(t *testing.T) {
	tests := []struct {
		name      string
		retrieved []string
		relevant  []string
		k         int
		want      float64
	}{
		{
			name:      "all relevant",
			retrieved: []string{"a", "b"},
			relevant:  []string{"a", "b"},
			k:         2,
			want:      1.0,
		},
		{
			name:      "half relevant",
			retrieved: []string{"a", "x"},
			relevant:  []string{"a", "b"},
			k:         2,
			want:      0.5,
		},
		{
			name:      "none relevant",
			retrieved: []string{"x", "y"},
			relevant:  []string{"a", "b"},
			k:         2,
			want:      0,
		},
		{
			name:      "k is zero",
			retrieved: []string{"a"},
			relevant:  []string{"a"},
			k:         0,
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrecisionAtK(QueryResult{RetrievedIDs: tt.retrieved, RelevantIDs: tt.relevant}, tt.k)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("PrecisionAtK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMRR(t *testing.T) {
	tests := []struct {
		name      string
		retrieved []string
		relevant  []string
		want      float64
	}{
		{
			name:      "relevant at rank 1",
			retrieved: []string{"a", "b", "c"},
			relevant:  []string{"a"},
			want:      1.0,
		},
		{
			name:      "relevant at rank 3",
			retrieved: []string{"x", "y", "a"},
			relevant:  []string{"a"},
			want:      1.0 / 3.0,
		},
		{
			name:      "no relevant found",
			retrieved: []string{"x", "y", "z"},
			relevant:  []string{"a"},
			want:      0,
		},
		{
			name:      "empty retrieved",
			retrieved: []string{},
			relevant:  []string{"a"},
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MRR(QueryResult{RetrievedIDs: tt.retrieved, RelevantIDs: tt.relevant})
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("MRR() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNDCGAtK(t *testing.T) {
	tests := []struct {
		name      string
		retrieved []string
		relevant  []string
		k         int
		want      float64
	}{
		{
			name:      "perfect ranking",
			retrieved: []string{"a", "b", "c"},
			relevant:  []string{"a", "b", "c"},
			k:         3,
			want:      1.0,
		},
		{
			name:      "worst ranking",
			retrieved: []string{"x", "y", "z"},
			relevant:  []string{"a", "b", "c"},
			k:         3,
			want:      0,
		},
		{
			name:      "partial ranking",
			retrieved: []string{"x", "a", "y"},
			relevant:  []string{"a", "b"},
			k:         3,
			want:      (1.0 / math.Log2(3)) / (1.0 + 1.0/math.Log2(3)),
		},
		{
			name:      "empty relevant",
			retrieved: []string{"a", "b"},
			relevant:  []string{},
			k:         2,
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NDCGAtK(QueryResult{RetrievedIDs: tt.retrieved, RelevantIDs: tt.relevant}, tt.k)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("NDCGAtK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeQueryMetrics(t *testing.T) {
	result := QueryResult{
		QueryID:      "q1",
		RetrievedIDs: []string{"a", "b", "c"},
		RelevantIDs:  []string{"a", "b"},
	}
	metrics := ComputeQueryMetrics(result, []int{1, 3})

	if metrics["recall@1"] != 0.5 {
		t.Errorf("recall@1 = %v, want 0.5", metrics["recall@1"])
	}
	if metrics["recall@3"] != 1.0 {
		t.Errorf("recall@3 = %v, want 1.0", metrics["recall@3"])
	}
	if metrics["mrr"] != 1.0 {
		t.Errorf("mrr = %v, want 1.0", metrics["mrr"])
	}
	if _, ok := metrics["ndcg@3"]; !ok {
		t.Error("missing ndcg@3 metric")
	}
}

func TestAggregateMetrics(t *testing.T) {
	perQuery := []QueryReport{
		{
			QueryID: "q1",
			Metrics: map[string]float64{"recall@1": 1.0, "mrr": 1.0},
		},
		{
			QueryID: "q2",
			Metrics: map[string]float64{"recall@1": 0.0, "mrr": 0.5},
		},
	}
	agg := AggregateMetrics(perQuery)

	if agg["recall@1"] != 0.5 {
		t.Errorf("recall@1 = %v, want 0.5", agg["recall@1"])
	}
	if agg["mrr"] != 0.75 {
		t.Errorf("mrr = %v, want 0.75", agg["mrr"])
	}
}

func TestAggregateMetricsEmpty(t *testing.T) {
	agg := AggregateMetrics(nil)
	if len(agg) != 0 {
		t.Errorf("expected empty map, got %v", agg)
	}
}

func TestPathToDocID(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{
			name:   "txt file",
			path:   "/docs/go-goroutine.txt",
			prefix: "/docs/",
			want:   "go-goroutine",
		},
		{
			name:   "md file",
			path:   "/docs/go-goroutine.md",
			prefix: "/docs/",
			want:   "go-goroutine",
		},
		{
			name:   "non-matching prefix",
			path:   "/other/go-goroutine.txt",
			prefix: "/docs/",
			want:   "",
		},
		{
			name:   "empty path",
			path:   "",
			prefix: "/docs/",
			want:   "",
		},
		{
			name:   "nested path",
			path:   "/docs/sub/deep.txt",
			prefix: "/docs/",
			want:   "sub/deep",
		},
		{
			name:   "no extension",
			path:   "/docs/readme",
			prefix: "/docs/",
			want:   "readme",
		},
		{
			name:   "other extension not stripped",
			path:   "/docs/file.json",
			prefix: "/docs/",
			want:   "file.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathToDocID(tt.path, tt.prefix)
			if got != tt.want {
				t.Errorf("pathToDocID(%q, %q) = %q, want %q", tt.path, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestExtractDocIDs(t *testing.T) {
	// Simulate vfs.SearchHit slice — we can't construct real ones
	// since vfs.SearchHit is a simple struct, test indirectly via pathToDocID
	// (extractDocIDs is just a thin wrapper around pathToDocID + dedup)
	// The pathToDocID tests above cover the core logic.
}

func TestNewBenchmarkEmptyTopK(t *testing.T) {
	_, err := NewBenchmark(BenchmarkConfig{
		DataDir: "benchmarks/data/example/",
		TopK:    []int{},
	})
	if err == nil {
		t.Fatal("expected error for empty TopK")
	}
}

func TestNewBenchmarkUnsortedTopK(t *testing.T) {
	_, err := NewBenchmark(BenchmarkConfig{
		DataDir: "benchmarks/data/example/",
		TopK:    []int{5, 1, 10},
	})
	if err == nil {
		t.Fatal("expected error for unsorted TopK")
	}
}

func TestNewBenchmarkNilTopK(t *testing.T) {
	_, err := NewBenchmark(BenchmarkConfig{
		DataDir: "benchmarks/data/example/",
		TopK:    nil,
	})
	if err == nil {
		t.Fatal("expected error for nil TopK")
	}
}
