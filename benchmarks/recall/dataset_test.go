package recall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCorpus(t *testing.T) {
	dir := t.TempDir()
	corpus := `{"doc_id":"d1","title":"Test","text":"Hello world"}
{"doc_id":"d2","title":"Test 2","text":"Goodbye world"}
`
	if err := os.WriteFile(filepath.Join(dir, "corpus.jsonl"), []byte(corpus), 0644); err != nil {
		t.Fatal(err)
	}

	docs, err := LoadCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
	if docs[0].DocID != "d1" || docs[1].DocID != "d2" {
		t.Errorf("unexpected doc IDs: %v", docs)
	}
}

func TestLoadQueries(t *testing.T) {
	dir := t.TempDir()
	queries := `{"query_id":"q1","query":"hello","relevant_doc_ids":["d1"]}
{"query_id":"q2","query":"goodbye","relevant_doc_ids":["d2"]}
`
	if err := os.WriteFile(filepath.Join(dir, "queries.jsonl"), []byte(queries), 0644); err != nil {
		t.Fatal(err)
	}

	qs, err := LoadQueries(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(qs))
	}
}

func TestValidateDataset(t *testing.T) {
	corpus := []CorpusDoc{
		{DocID: "d1", Title: "T1", Text: "text1"},
		{DocID: "d2", Title: "T2", Text: "text2"},
	}
	queries := []Query{
		{QueryID: "q1", Query: "hello", RelevantDocIDs: []string{"d1"}},
		{QueryID: "q2", Query: "world", RelevantDocIDs: []string{"d1", "d2"}},
	}

	if err := ValidateDataset(corpus, queries); err != nil {
		t.Fatalf("valid dataset failed: %v", err)
	}
}

func TestValidateDatasetDuplicateDocID(t *testing.T) {
	corpus := []CorpusDoc{
		{DocID: "d1", Title: "T1", Text: "text1"},
		{DocID: "d1", Title: "T1 dup", Text: "text1 dup"},
	}
	queries := []Query{
		{QueryID: "q1", Query: "hello", RelevantDocIDs: []string{"d1"}},
	}

	if err := ValidateDataset(corpus, queries); err == nil {
		t.Fatal("expected error for duplicate doc_id")
	}
}

func TestValidateDatasetMissingRelevantDoc(t *testing.T) {
	corpus := []CorpusDoc{
		{DocID: "d1", Title: "T1", Text: "text1"},
	}
	queries := []Query{
		{QueryID: "q1", Query: "hello", RelevantDocIDs: []string{"d99"}},
	}

	if err := ValidateDataset(corpus, queries); err == nil {
		t.Fatal("expected error for missing relevant doc")
	}
}

func TestValidateDatasetEmptyQuery(t *testing.T) {
	corpus := []CorpusDoc{
		{DocID: "d1", Title: "T1", Text: "text1"},
	}
	queries := []Query{
		{QueryID: "q1", Query: "", RelevantDocIDs: []string{"d1"}},
	}

	if err := ValidateDataset(corpus, queries); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestValidateDatasetNoRelevantDocs(t *testing.T) {
	corpus := []CorpusDoc{
		{DocID: "d1", Title: "T1", Text: "text1"},
	}
	queries := []Query{
		{QueryID: "q1", Query: "hello", RelevantDocIDs: []string{}},
	}

	if err := ValidateDataset(corpus, queries); err == nil {
		t.Fatal("expected error for empty relevant doc IDs")
	}
}

func TestValidateDatasetEmptyDocID(t *testing.T) {
	corpus := []CorpusDoc{
		{DocID: "", Title: "T1", Text: "text1"},
	}
	queries := []Query{
		{QueryID: "q1", Query: "hello", RelevantDocIDs: []string{"d1"}},
	}

	if err := ValidateDataset(corpus, queries); err == nil {
		t.Fatal("expected error for empty doc_id")
	}
}

func TestValidateDatasetUnsafeDocID(t *testing.T) {
	tests := []struct {
		name  string
		docID string
		ok    bool
	}{
		{"hyphen", "my-doc", true},
		{"underscore", "my_doc", true},
		{"alphanumeric", "doc123", true},
		{"slash", "my/doc", false},
		{"dot", "my.doc", false},
		{"space", "my doc", false},
		{"parent", "../etc/passwd", false},
		{"null", "doc\x00id", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corpus := []CorpusDoc{
				{DocID: tt.docID, Text: "content"},
			}
			queries := []Query{
				{QueryID: "q1", Query: "hello", RelevantDocIDs: []string{tt.docID}},
			}
			// May also fail on missing relevant doc if docID was rejected
			err := ValidateDataset(corpus, queries)
			if tt.ok && err != nil {
				t.Errorf("expected docID %q to be valid, got error: %v", tt.docID, err)
			}
			if !tt.ok && err == nil {
				t.Errorf("expected docID %q to be rejected", tt.docID)
			}
		})
	}
}

func TestLoadCorpusMissingFile(t *testing.T) {
	_, err := LoadCorpus(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadCorpusEmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "corpus.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCorpus(dir)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}
