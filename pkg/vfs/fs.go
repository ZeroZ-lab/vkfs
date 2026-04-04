package vfs

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// VectorStore interface (forward declaration - actual interface in pkg/vectorstore)
type VectorStore interface {
	UpsertPathTree(ctx context.Context, tree PathTree) error
	GetPathTree(ctx context.Context) (PathTree, error)
	GetChunksByPage(ctx context.Context, pageSlug string) ([]Chunk, error)
	GetLazyPointer(ctx context.Context, pageSlug string) (*LazyPointer, error)
	SearchText(ctx context.Context, pattern string, filter PathFilter, limit int) ([]Chunk, error)
	SearchVector(ctx context.Context, queryVec []float32, filter PathFilter, topK int) ([]SearchHit, error)
	// ... other methods
}

// ExternalStore interface for lazy pointer storage
type ExternalStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// EmbeddingProvider interface for text-to-vector conversion
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// VirtualFS represents the virtual filesystem with in-memory PathTree
type VirtualFS struct {
	vectorStore   VectorStore
	externalStore ExternalStore
	embedder      EmbeddingProvider
	tree          PathTree
	pathIndex     map[string][]string    // dir -> children names
	nodeIndex     map[string]VirtualNode // path -> node
}

// NewVirtualFS creates a new VirtualFS instance
func NewVirtualFS(store VectorStore, extStore ExternalStore, embedder EmbeddingProvider) *VirtualFS {
	return &VirtualFS{
		vectorStore:   store,
		externalStore: extStore,
		embedder:      embedder,
		pathIndex:     make(map[string][]string),
		nodeIndex:     make(map[string]VirtualNode),
	}
}

// Init loads the PathTree from vector database into memory
func (fs *VirtualFS) Init(ctx context.Context) error {
	// Load PathTree from vector database (one network call)
	tree, err := fs.vectorStore.GetPathTree(ctx)
	if err != nil {
		return fmt.Errorf("failed to load PathTree: %w", err)
	}

	fs.tree = tree

	// Build in-memory indexes
	fs.buildIndexes()

	return nil
}

// buildIndexes constructs pathIndex and nodeIndex from tree
func (fs *VirtualFS) buildIndexes() {
	fs.pathIndex = make(map[string][]string)
	fs.nodeIndex = make(map[string]VirtualNode)

	for path, node := range fs.tree.Nodes {
		// Add to nodeIndex
		fs.nodeIndex[path] = node

		// Add to pathIndex (parent -> children)
		if path == "/" {
			continue // root has no parent
		}

		parent := filepath.Dir(path)
		if parent == "." {
			parent = "/"
		}

		fs.pathIndex[parent] = append(fs.pathIndex[parent], node.Name)
	}
}

// Ls lists directory contents (in-memory operation, zero network calls)
func (fs *VirtualFS) Ls(path string) ([]VirtualNode, error) {
	// Normalize path
	if path == "" {
		path = "/"
	}

	// Check if path exists
	node, exists := fs.nodeIndex[path]
	if !exists {
		return nil, fmt.Errorf("path not found: %s", path)
	}

	// Check if it's a directory
	if !node.IsDir {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	// Get children from pathIndex
	childNames := fs.pathIndex[path]
	if len(childNames) == 0 {
		return []VirtualNode{}, nil // empty directory
	}

	// Build result
	result := make([]VirtualNode, 0, len(childNames))
	for _, name := range childNames {
		childPath := filepath.Join(path, name)
		if path == "/" {
			childPath = "/" + name
		}

		if childNode, exists := fs.nodeIndex[childPath]; exists {
			result = append(result, childNode)
		}
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// Stat returns node information (in-memory operation)
func (fs *VirtualFS) Stat(path string) (VirtualNode, error) {
	// Normalize path
	if path == "" {
		path = "/"
	}

	node, exists := fs.nodeIndex[path]
	if !exists {
		return VirtualNode{}, fmt.Errorf("path not found: %s", path)
	}

	return node, nil
}

// Find searches for files matching a glob pattern (in-memory operation)
func (fs *VirtualFS) Find(rootPath string, pattern string) ([]string, error) {
	// Normalize root path
	if rootPath == "" {
		rootPath = "/"
	}

	// Check if root exists
	if _, exists := fs.nodeIndex[rootPath]; !exists {
		return nil, fmt.Errorf("path not found: %s", rootPath)
	}

	var results []string

	// Walk all nodes
	for path, node := range fs.nodeIndex {
		// Skip directories
		if node.IsDir {
			continue
		}

		// Check if path is under rootPath
		if !strings.HasPrefix(path, rootPath) {
			continue
		}

		// Check if filename matches pattern
		matched, err := filepath.Match(pattern, node.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}

		if matched {
			results = append(results, path)
		}
	}

	// Sort results
	sort.Strings(results)

	return results, nil
}

// Cat returns the complete content of a file
func (fs *VirtualFS) Cat(ctx context.Context, path string) (string, error) {
	// Normalize path
	if path == "" {
		return "", fmt.Errorf("path required")
	}

	// Check if path exists
	node, exists := fs.nodeIndex[path]
	if !exists {
		return "", fmt.Errorf("path not found: %s", path)
	}

	// Check if it's a file
	if node.IsDir {
		return "", fmt.Errorf("is a directory: %s", path)
	}

	// Check for lazy pointer first
	lazyPtr, err := fs.vectorStore.GetLazyPointer(ctx, path)
	if err == nil && lazyPtr != nil {
		// File is stored externally (S3)
		if fs.externalStore == nil {
			return "", fmt.Errorf("external store not configured but file uses lazy pointer")
		}

		// Extract key from external URL (s3://bucket/key)
		key := extractKeyFromURL(lazyPtr.ExternalURL)

		// Fetch from external store with retries
		content, err := fs.fetchWithRetry(ctx, key, 3)
		if err != nil {
			return "", fmt.Errorf("failed to fetch from external store: %w", err)
		}

		return string(content), nil
	}

	// Not a lazy pointer - fetch chunks from vector DB
	chunks, err := fs.vectorStore.GetChunksByPage(ctx, path)
	if err != nil {
		return "", fmt.Errorf("failed to get chunks: %w", err)
	}

	if len(chunks) == 0 {
		return "", nil // empty file
	}

	// Validate chunk integrity
	if err := validateChunkIntegrity(chunks); err != nil {
		return "", err
	}

	// Sort by chunk_index
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].ChunkIndex < chunks[j].ChunkIndex
	})

	// Assemble content
	var content strings.Builder
	for _, chunk := range chunks {
		content.WriteString(chunk.Text)
	}

	return content.String(), nil
}

// validateChunkIntegrity checks that chunks form a valid contiguous sequence
func validateChunkIntegrity(chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Build index map to detect duplicates and gaps
	indexMap := make(map[int]bool)
	maxIndex := -1

	for _, chunk := range chunks {
		if indexMap[chunk.ChunkIndex] {
			return fmt.Errorf("Duplicate chunk index %d", chunk.ChunkIndex)
		}
		indexMap[chunk.ChunkIndex] = true

		if chunk.ChunkIndex > maxIndex {
			maxIndex = chunk.ChunkIndex
		}
	}

	// Verify contiguous sequence starting from 0
	for i := 0; i <= maxIndex; i++ {
		if !indexMap[i] {
			return fmt.Errorf("Chunk %d missing", i)
		}
	}

	return nil
}

// fetchWithRetry fetches from external store with exponential backoff
func (fs *VirtualFS) fetchWithRetry(ctx context.Context, key string, maxRetries int) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		content, err := fs.externalStore.Get(ctx, key)
		if err == nil {
			return content, nil
		}

		lastErr = err

		// Exponential backoff: 1s, 2s, 4s
		if attempt < maxRetries-1 {
			backoff := (1 << attempt) * 1000 // milliseconds
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(backoff) * time.Millisecond):
				// continue to next retry
			}
		}
	}

	return nil, fmt.Errorf("retries exhausted after %d attempts: %w", maxRetries, lastErr)
}

// extractKeyFromURL extracts the key from s3://bucket/key format
func extractKeyFromURL(url string) string {
	// Simple extraction: s3://bucket/key -> key
	parts := strings.SplitN(url, "://", 2)
	if len(parts) != 2 {
		return url
	}

	// Remove bucket prefix
	pathParts := strings.SplitN(parts[1], "/", 2)
	if len(pathParts) != 2 {
		return parts[1]
	}

	return pathParts[1]
}

// Grep searches for pattern in files (two-stage: BM25 coarse + regex fine)
func (fs *VirtualFS) Grep(ctx context.Context, pattern string, rootPath string) ([]GrepResult, error) {
	// Normalize root path
	if rootPath == "" {
		rootPath = "/"
	}

	// Check if root exists
	if _, exists := fs.nodeIndex[rootPath]; !exists {
		return nil, fmt.Errorf("path not found: %s", rootPath)
	}

	// Stage 1: BM25 coarse filter (top-50 candidates from vector DB)
	filter := PathFilter{
		PathPrefix: rootPath,
		Groups:     []string{"*"}, // TODO: implement ACL
	}

	chunks, err := fs.vectorStore.SearchText(ctx, pattern, filter, 50)
	if err != nil {
		return nil, fmt.Errorf("BM25 search failed: %w", err)
	}

	// Stage 2: Regex fine filter (in-memory)
	var results []GrepResult

	for _, chunk := range chunks {
		// Split chunk text into lines
		lines := strings.Split(chunk.Text, "\n")

		for lineNum, line := range lines {
			// Check if line matches pattern (simple substring match for now)
			// TODO: support full regex
			if strings.Contains(line, pattern) {
				results = append(results, GrepResult{
					Path:    chunk.PageSlug,
					LineNum: lineNum + 1, // 1-indexed
					Line:    line,
				})
			}
		}
	}

	return results, nil
}

// Search performs semantic search using embeddings
func (fs *VirtualFS) Search(ctx context.Context, query string, rootPath string, topK int) ([]SearchHit, error) {
	// Normalize root path
	if rootPath == "" {
		rootPath = "/"
	}

	// Check if root exists
	if _, exists := fs.nodeIndex[rootPath]; !exists {
		return nil, fmt.Errorf("path not found: %s", rootPath)
	}

	// Check if embedder is configured
	if fs.embedder == nil {
		return nil, fmt.Errorf("embedding provider not configured")
	}

	// Convert query to embedding
	queryVec, err := fs.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Perform vector search
	filter := PathFilter{
		PathPrefix: rootPath,
		Groups:     []string{"*"}, // TODO: implement ACL
	}

	hits, err := fs.vectorStore.SearchVector(ctx, queryVec, filter, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	return hits, nil
}
