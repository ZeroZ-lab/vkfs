package vectorstore

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

// SQLiteAdapter implements VectorStore using a local SQLite database.
// Vectors are stored as BLOBs; similarity search uses brute-force L2 distance in Go.
type SQLiteAdapter struct {
	db        *sql.DB
	dimension int
}

// SQLiteConfig holds configuration for the SQLite adapter
type SQLiteConfig struct {
	Path      string
	Dimension int
}

// NewSQLiteAdapter creates a new SQLite-based vector store
func NewSQLiteAdapter(cfg SQLiteConfig) (*SQLiteAdapter, error) {
	path := cfg.Path
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = filepath.Join(home, ".vkfs", "vkfs.db")
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = home + path[1:]
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	adapter := &SQLiteAdapter{db: db, dimension: cfg.Dimension}
	if err := adapter.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return adapter, nil
}

func (s *SQLiteAdapter) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		doc_type TEXT NOT NULL,
		page_slug TEXT DEFAULT '',
		chunk_index INTEGER DEFAULT 0,
		text TEXT DEFAULT '',
		content TEXT DEFAULT '',
		external_url TEXT DEFAULT '',
		size INTEGER DEFAULT 0,
		vector BLOB DEFAULT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_doc_type ON documents(doc_type);
	CREATE INDEX IF NOT EXISTS idx_doc_page ON documents(doc_type, page_slug);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *SQLiteAdapter) Close() error {
	return s.db.Close()
}

// UpsertPathTree stores the complete file tree as a special document
func (s *SQLiteAdapter) UpsertPathTree(ctx context.Context, tree vfs.PathTree) error {
	treeJSON, err := json.Marshal(tree)
	if err != nil {
		return fmt.Errorf("marshal PathTree: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO documents (id, doc_type, content, vector)
		VALUES (?, 'path_tree', ?, NULL)
	`, "__path_tree__", string(treeJSON))
	return err
}

// GetPathTree retrieves the complete file tree
func (s *SQLiteAdapter) GetPathTree(ctx context.Context) (vfs.PathTree, error) {
	var content string
	err := s.db.QueryRowContext(ctx,
		"SELECT content FROM documents WHERE id = ?", "__path_tree__",
	).Scan(&content)
	if err == sql.ErrNoRows {
		return vfs.PathTree{}, fmt.Errorf("PathTree not found - run vkfs-admin init first")
	}
	if err != nil {
		return vfs.PathTree{}, err
	}

	var tree vfs.PathTree
	if err := json.Unmarshal([]byte(content), &tree); err != nil {
		return vfs.PathTree{}, fmt.Errorf("unmarshal PathTree: %w", err)
	}
	return tree, nil
}

// UpsertChunks batch inserts chunks with their vectors
func (s *SQLiteAdapter) UpsertChunks(ctx context.Context, chunks []vfs.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO documents (id, doc_type, page_slug, chunk_index, text, vector)
		VALUES (?, 'chunk', ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range chunks {
		var vecBlob interface{}
		if len(c.Embedding) > 0 {
			vecBlob = encodeVector(c.Embedding)
		}
		if _, err := stmt.ExecContext(ctx, c.ID, c.PageSlug, c.ChunkIndex, c.Text, vecBlob); err != nil {
			return fmt.Errorf("upsert chunk %s: %w", c.ID, err)
		}
	}

	return tx.Commit()
}

// GetChunksByPage retrieves all chunks for a file
func (s *SQLiteAdapter) GetChunksByPage(ctx context.Context, pageSlug string) ([]vfs.Chunk, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, page_slug, chunk_index, text
		FROM documents WHERE page_slug = ? AND doc_type = 'chunk'
		ORDER BY chunk_index
	`, pageSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []vfs.Chunk
	for rows.Next() {
		var c vfs.Chunk
		if err := rows.Scan(&c.ID, &c.PageSlug, &c.ChunkIndex, &c.Text); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// DeleteChunksByPage deletes all chunks for a file
func (s *SQLiteAdapter) DeleteChunksByPage(ctx context.Context, pageSlug string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM documents WHERE page_slug = ? AND doc_type = 'chunk'", pageSlug,
	)
	return err
}

// UpsertLazyPointer stores external storage reference
func (s *SQLiteAdapter) UpsertLazyPointer(ctx context.Context, pointer vfs.LazyPointer) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO documents (id, doc_type, page_slug, external_url, size, vector)
		VALUES (?, 'lazy_pointer', ?, ?, ?, NULL)
	`, "lazy_"+pointer.PageSlug, pointer.PageSlug, pointer.ExternalURL, pointer.Size)
	return err
}

// GetLazyPointer checks if a file is a lazy pointer
func (s *SQLiteAdapter) GetLazyPointer(ctx context.Context, pageSlug string) (*vfs.LazyPointer, error) {
	var p vfs.LazyPointer
	err := s.db.QueryRowContext(ctx, `
		SELECT page_slug, external_url, size
		FROM documents WHERE id = ? AND doc_type = 'lazy_pointer'
	`, "lazy_"+pageSlug).Scan(&p.PageSlug, &p.ExternalURL, &p.Size)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SearchText performs text search with optional path prefix filter
func (s *SQLiteAdapter) SearchText(ctx context.Context, pattern string, filter vfs.PathFilter, limit int) ([]vfs.Chunk, error) {
	query := `
		SELECT id, page_slug, chunk_index, text
		FROM documents WHERE doc_type = 'chunk'
	`
	args := []interface{}{}

	if filter.PathPrefix != "" && filter.PathPrefix != "/" {
		query += " AND page_slug LIKE ?"
		args = append(args, filter.PathPrefix+"%")
	}

	if pattern != "" {
		query += " AND text LIKE ?"
		args = append(args, "%"+pattern+"%")
	}

	query += " ORDER BY page_slug, chunk_index LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []vfs.Chunk
	for rows.Next() {
		var c vfs.Chunk
		if err := rows.Scan(&c.ID, &c.PageSlug, &c.ChunkIndex, &c.Text); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// SearchVector performs vector similarity search using brute-force L2 distance
func (s *SQLiteAdapter) SearchVector(ctx context.Context, queryVec []float32, filter vfs.PathFilter, topK int) ([]vfs.SearchHit, error) {
	query := `
		SELECT id, page_slug, chunk_index, text, vector
		FROM documents WHERE doc_type = 'chunk' AND vector IS NOT NULL
	`
	args := []interface{}{}

	if filter.PathPrefix != "" && filter.PathPrefix != "/" {
		query += " AND page_slug LIKE ?"
		args = append(args, filter.PathPrefix+"%")
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		chunk    vfs.Chunk
		distance float32
	}
	var candidates []candidate

	for rows.Next() {
		var id, pageSlug, text string
		var chunkIndex int
		var vecBlob []byte
		if err := rows.Scan(&id, &pageSlug, &chunkIndex, &text, &vecBlob); err != nil {
			return nil, err
		}

		vec := decodeVector(vecBlob)
		dist := l2Distance(queryVec, vec)

		candidates = append(candidates, candidate{
			chunk: vfs.Chunk{
				ID:         id,
				PageSlug:   pageSlug,
				ChunkIndex: chunkIndex,
				Text:       text,
			},
			distance: dist,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	if topK > len(candidates) {
		topK = len(candidates)
	}

	hits := make([]vfs.SearchHit, topK)
	for i := 0; i < topK; i++ {
		hits[i] = vfs.SearchHit{
			Chunk: candidates[i].chunk,
			Score: candidates[i].distance,
		}
	}
	return hits, nil
}

// SearchHybrid performs hybrid search (text + vector)
// TODO: Full implementation - combine text search scores (BM25) with vector scores using RRF or similar
func (s *SQLiteAdapter) SearchHybrid(ctx context.Context, queryVec []float32, pattern string, filter vfs.PathFilter, topK int) ([]vfs.SearchHit, error) {
	// Simplified implementation: just do vector search for now
	// A full implementation would combine text search scores with vector scores
	return s.SearchVector(ctx, queryVec, filter, topK)
}

// Vector encoding helpers

func encodeVector(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func decodeVector(data []byte) []float32 {
	n := len(data) / 4
	vec := make([]float32, n)
	for i := 0; i < n; i++ {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return vec
}

func l2Distance(a, b []float32) float32 {
	if len(a) != len(b) {
		return float32(math.Inf(1))
	}
	var sum float32
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return float32(math.Sqrt(float64(sum)))
}
