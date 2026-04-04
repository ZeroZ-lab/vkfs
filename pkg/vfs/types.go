package vfs

import "time"

// VirtualNode represents a node in the virtual filesystem
type VirtualNode struct {
	Path      string            `json:"path"`
	Name      string            `json:"name"`
	IsDir     bool              `json:"is_dir"`
	Size      int64             `json:"size"`
	ModTime   time.Time         `json:"mod_time"`
	VisibleTo []string          `json:"visible_to"` // ACL groups
	Metadata  map[string]string `json:"metadata"`   // Extended metadata
}

// PathTree represents the complete file tree (stored in vector DB as special document)
type PathTree struct {
	Nodes    map[string]VirtualNode `json:"nodes"`    // path -> node
	Version  string                 `json:"version"`  // tree version
	Metadata map[string]string      `json:"metadata"` // e.g., embedding_model
}

// Chunk represents a document chunk
type Chunk struct {
	ID         string            `json:"id"`
	PageSlug   string            `json:"page_slug"`   // file path (for cat query)
	ChunkIndex int               `json:"chunk_index"` // for sorting/assembly
	Text       string            `json:"text"`
	Embedding  []float32         `json:"-"`           // vector (not serialized to JSON)
	Metadata   map[string]string `json:"metadata"`    // extended metadata
}

// LazyPointer represents external storage reference for large files
type LazyPointer struct {
	PageSlug    string `json:"page_slug"`
	ExternalURL string `json:"external_url"` // s3://bucket/key
	Size        int64  `json:"size"`
}

// SearchHit represents a retrieval result
type SearchHit struct {
	Chunk Chunk   `json:"chunk"`
	Score float32 `json:"score"`
}

// PathFilter limits search scope
type PathFilter struct {
	PathPrefix string   `json:"path_prefix"` // limit to path prefix
	Groups     []string `json:"groups"`      // user's ACL groups
}

// GrepResult represents a grep match
type GrepResult struct {
	Path    string
	LineNum int
	Line    string
}
