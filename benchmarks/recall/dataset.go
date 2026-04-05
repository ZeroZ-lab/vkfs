package recall

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var safeDocIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// CorpusDoc represents a document in the benchmark corpus.
type CorpusDoc struct {
	DocID string `json:"doc_id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Query represents a search query with ground truth relevance.
type Query struct {
	QueryID            string   `json:"query_id"`
	Query              string   `json:"query"`
	RelevantDocIDs     []string `json:"relevant_doc_ids"`
	RelevantChunkTexts []string `json:"relevant_chunk_texts,omitempty"`
}

// LoadCorpus reads corpus.jsonl from the data directory.
func LoadCorpus(dataDir string) ([]CorpusDoc, error) {
	return loadJSONL[CorpusDoc](filepath.Join(dataDir, "corpus.jsonl"))
}

// LoadQueries reads queries.jsonl from the data directory.
func LoadQueries(dataDir string) ([]Query, error) {
	return loadJSONL[Query](filepath.Join(dataDir, "queries.jsonl"))
}

// ValidateDataset checks corpus and queries for consistency.
func ValidateDataset(corpus []CorpusDoc, queries []Query) error {
	docIDs := make(map[string]bool, len(corpus))
	for _, doc := range corpus {
		if doc.DocID == "" {
			return fmt.Errorf("doc_id cannot be empty")
		}
		if !safeDocIDRe.MatchString(doc.DocID) {
			return fmt.Errorf("doc_id %q contains unsafe characters (only [a-zA-Z0-9_-] allowed)", doc.DocID)
		}
		if docIDs[doc.DocID] {
			return fmt.Errorf("duplicate doc_id: %s", doc.DocID)
		}
		docIDs[doc.DocID] = true
		if doc.Text == "" {
			return fmt.Errorf("doc %s has empty text", doc.DocID)
		}
	}

	for _, q := range queries {
		if q.QueryID == "" {
			return fmt.Errorf("query_id cannot be empty")
		}
		if q.Query == "" {
			return fmt.Errorf("query %s has empty query text", q.QueryID)
		}
		if len(q.RelevantDocIDs) == 0 {
			return fmt.Errorf("query %s has no relevant doc IDs", q.QueryID)
		}
		for _, relID := range q.RelevantDocIDs {
			if !docIDs[relID] {
				return fmt.Errorf("query %s references non-existent doc_id: %s", q.QueryID, relID)
			}
		}
	}

	return nil
}

func loadJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var items []T
	decoder := json.NewDecoder(bufio.NewReader(f))
	line := 0
	for decoder.More() {
		line++
		var item T
		if err := decoder.Decode(&item); err != nil {
			return nil, fmt.Errorf("parse %s line %d: %w", path, line, err)
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no records found in %s", path)
	}
	return items, nil
}
