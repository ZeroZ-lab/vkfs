package vfs

import (
	"context"
	"fmt"
	"os"
	stdpath "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// maxRegexLength limits pattern length to prevent ReDoS
const maxRegexLength = 1000

// validateRegexPattern checks if a regex pattern is safe to compile
// Returns an error if the pattern is too complex (potential ReDoS)
func validateRegexPattern(pattern string) error {
	if len(pattern) > maxRegexLength {
		return fmt.Errorf("regex pattern too long: %d characters (max %d)", len(pattern), maxRegexLength)
	}

	// Check for dangerous patterns that could cause catastrophic backtracking
	// e.g., nested quantifiers like (a+)+, (a*)*, etc.
	dangerousPatterns := []string{
		`\+\)`,   // )+ patterns
		`\*\)`,   // )* patterns
		`\+\+`,   // ++ patterns
		`\*\*`,   // ** patterns
	}
	for _, dp := range dangerousPatterns {
		if matched, _ := regexp.MatchString(dp, pattern); matched {
			return fmt.Errorf("regex pattern contains potentially dangerous construct")
		}
	}

	return nil
}

// VectorStore interface for vector database operations.
// Implemented by ZillizAdapter, QdrantAdapter, etc.
type VectorStore interface {
	UpsertPathTree(ctx context.Context, tree PathTree) error
	GetPathTree(ctx context.Context) (PathTree, error)
	UpsertChunks(ctx context.Context, chunks []Chunk) error
	GetChunksByPage(ctx context.Context, pageSlug string) ([]Chunk, error)
	DeleteChunksByPage(ctx context.Context, pageSlug string) error
	UpsertLazyPointer(ctx context.Context, pointer LazyPointer) error
	GetLazyPointer(ctx context.Context, pageSlug string) (*LazyPointer, error)
	SearchText(ctx context.Context, pattern string, filter PathFilter, limit int) ([]Chunk, error)
	SearchVector(ctx context.Context, queryVec []float32, filter PathFilter, topK int) ([]SearchHit, error)
	SearchHybrid(ctx context.Context, queryVec []float32, pattern string, filter PathFilter, topK int) ([]SearchHit, error)
}

// ExternalStore interface for lazy pointer storage
type ExternalStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// EmbeddingProvider interface for text-to-vector conversion
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
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

		parent := stdpath.Dir(path)
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
		childPath := stdpath.Join(path, name)
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
		matched, err := stdpath.Match(pattern, node.Name)
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

	// Validate regex pattern for safety (prevent ReDoS)
	if err := validateRegexPattern(pattern); err != nil {
		return nil, fmt.Errorf("unsafe regex pattern: %w", err)
	}

	// Compile regex pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	for _, chunk := range chunks {
		// Split chunk text into lines
		lines := strings.Split(chunk.Text, "\n")

		for lineNum, line := range lines {
			// Check if line matches regex pattern
			if re.MatchString(line) {
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

// IngestResult holds statistics from an ingest operation
type IngestResult struct {
	FilesIngested int
	ChunksCreated int
	BytesRead     int64
}

// Ingest reads files from a local directory and ingests them into the virtual filesystem.
func (fs *VirtualFS) Ingest(ctx context.Context, localDir string, vkfsPath string) (*IngestResult, error) {
	// Normalize vkfs path
	vkfsPath = strings.TrimRight(vkfsPath, "/")
	if vkfsPath == "" {
		vkfsPath = "/"
	}

	// Read local directory
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read local directory: %w", err)
	}

	var allChunks []Chunk
	var newNodes []VirtualNode
	result := &IngestResult{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // skip subdirectories for now
		}

		localPath := filepath.Join(localDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", localPath, err)
		}

		// Read file content
		content, err := os.ReadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", localPath, err)
		}

		// Build vkfs path for this file
		fileVKFSPath := vkfsPath + "/" + entry.Name()

		// Split into chunks
		chunks := SplitFile(fileVKFSPath, content, 0)
		if len(chunks) == 0 {
			continue
		}

		// Embed all chunk texts in batch
		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Text
		}

		embeddings, err := fs.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("failed to embed chunks for %s: %w", entry.Name(), err)
		}

		// Attach embeddings to chunks
		for i := range chunks {
			if i < len(embeddings) {
				chunks[i].Embedding = embeddings[i]
			}
		}

		allChunks = append(allChunks, chunks...)

		// Create VirtualNode for this file
		newNodes = append(newNodes, VirtualNode{
			Path:    fileVKFSPath,
			Name:    entry.Name(),
			IsDir:   false,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})

		result.FilesIngested++
		result.BytesRead += int64(len(content))
	}

	if len(allChunks) == 0 {
		return result, nil
	}

	// Update PathTree with new nodes
	if fs.tree.Nodes == nil {
		fs.tree.Nodes = make(map[string]VirtualNode)
	}

	// Ensure parent directories exist in tree
	ensureDirInTree(&fs.tree, vkfsPath)

	// Add file nodes
	for _, node := range newNodes {
		fs.tree.Nodes[node.Path] = node
	}

	// Persist updated PathTree
	if err := fs.vectorStore.UpsertPathTree(ctx, fs.tree); err != nil {
		return nil, fmt.Errorf("failed to persist PathTree: %w", err)
	}

	// Persist chunks
	if err := fs.vectorStore.UpsertChunks(ctx, allChunks); err != nil {
		return nil, fmt.Errorf("failed to persist chunks: %w", err)
	}

	// Rebuild in-memory indexes
	fs.buildIndexes()

	result.ChunksCreated = len(allChunks)
	return result, nil
}

// ensureDirInTree adds directory nodes to the tree for a given path and all parents.
func ensureDirInTree(tree *PathTree, dirPath string) {
	if dirPath == "/" {
		// Ensure root exists
		if _, ok := tree.Nodes["/"]; !ok {
			tree.Nodes["/"] = VirtualNode{
				Path:  "/",
				Name:  "/",
				IsDir: true,
			}
		}
		return
	}

	parts := strings.Split(strings.Trim(dirPath, "/"), "/")
	current := ""
	for _, part := range parts {
		parent := current
		current = current + "/" + part

		if _, ok := tree.Nodes[current]; !ok {
			tree.Nodes[current] = VirtualNode{
				Path:  current,
				Name:  part,
				IsDir: true,
			}
		}

		// Also ensure parent exists
		if parent == "" {
			ensureDirInTree(tree, "/")
		}
	}
}
