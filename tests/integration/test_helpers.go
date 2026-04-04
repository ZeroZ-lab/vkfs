package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Test helper functions for setting up test fixtures

// setupEmptyZillizCollection creates a fresh empty Zilliz collection for testing
func setupEmptyZillizCollection(t *testing.T) VectorStore {
	t.Helper()

	endpoint := os.Getenv("ZILLIZ_ENDPOINT")
	apiKey := os.Getenv("ZILLIZ_API_KEY")
	if endpoint == "" || apiKey == "" {
		t.Skip("ZILLIZ_ENDPOINT and ZILLIZ_API_KEY required for integration tests")
	}

	collectionName := "vkfs_test_" + t.Name() + "_" + time.Now().Format("20060102150405")

	adapter, err := NewZillizAdapter(ZillizConfig{
		Endpoint:   endpoint,
		APIKey:     apiKey,
		Collection: collectionName,
	})
	require.NoError(t, err)

	return adapter
}

// cleanupZillizCollection drops the test collection
func cleanupZillizCollection(t *testing.T, store VectorStore) {
	t.Helper()
	if adapter, ok := store.(*ZillizAdapter); ok {
		_ = adapter.DropCollection(context.Background())
	}
}

// setupZillizWithSampleData creates a collection with initialized PathTree
func setupZillizWithSampleData(t *testing.T) VectorStore {
	t.Helper()
	store := setupEmptyZillizCollection(t)

	// Initialize with empty PathTree
	tree := PathTree{
		Nodes: map[string]VirtualNode{
			"/": {
				Path:  "/",
				Name:  "/",
				IsDir: true,
			},
		},
		Version: "1.0",
	}
	err := store.UpsertPathTree(context.Background(), tree)
	require.NoError(t, err)

	return store
}

// setupZillizWithEmbedding creates a collection with specific embedding model metadata
func setupZillizWithEmbedding(t *testing.T, modelName string) VectorStore {
	t.Helper()
	store := setupZillizWithSampleData(t)

	// Store embedding model in PathTree metadata
	tree, err := store.GetPathTree(context.Background())
	require.NoError(t, err)

	if tree.Metadata == nil {
		tree.Metadata = make(map[string]string)
	}
	tree.Metadata["embedding_model"] = modelName

	err = store.UpsertPathTree(context.Background(), tree)
	require.NoError(t, err)

	return store
}

// setupZillizWithTimeout creates a store that simulates connection timeouts
func setupZillizWithTimeout(t *testing.T, timeout time.Duration) VectorStore {
	t.Helper()
	// Implementation would wrap real adapter with timeout simulation
	// For now, return mock
	return &MockTimeoutStore{timeout: timeout}
}

// setupZillizWithTransientFailure creates a store that fails N times then succeeds
func setupZillizWithTransientFailure(t *testing.T, failCount int) VectorStore {
	t.Helper()
	return &MockTransientFailureStore{
		failCount:    failCount,
		attemptCount: 0,
		realStore:    setupZillizWithSampleData(t),
	}
}

// setupMockS3 creates a mock S3 store for testing lazy pointers
func setupMockS3(t *testing.T) ExternalStore {
	t.Helper()
	return &MockS3Store{
		storage: make(map[string][]byte),
	}
}

// setupFailingS3 creates an S3 store that fails N times
func setupFailingS3(t *testing.T, failCount int) ExternalStore {
	t.Helper()
	return &MockFailingS3Store{
		failCount:    failCount,
		attemptCount: 0,
	}
}

// setupTimeoutS3 creates an S3 store that times out
func setupTimeoutS3(t *testing.T, timeout time.Duration) ExternalStore {
	t.Helper()
	return &MockTimeoutS3Store{timeout: timeout}
}

// setupMockEmbedder creates a mock embedding provider
func setupMockEmbedder(t *testing.T) EmbeddingProvider {
	t.Helper()
	return &MockEmbedder{
		dimension: 1536, // OpenAI text-embedding-3-small dimension
	}
}

// setupTimeoutEmbedder creates an embedder that times out
func setupTimeoutEmbedder(t *testing.T, timeout time.Duration) EmbeddingProvider {
	t.Helper()
	return &MockTimeoutEmbedder{timeout: timeout}
}

// runVKFSAdminInit runs vkfs-admin init command
func runVKFSAdminInit(ctx context.Context, store VectorStore) error {
	// Initialize empty PathTree with root node
	tree := PathTree{
		Nodes: map[string]VirtualNode{
			"/": {
				Path:      "/",
				Name:      "/",
				IsDir:     true,
				ModTime:   time.Now(),
				VisibleTo: []string{"*"},
			},
		},
		Version: "1.0",
	}
	return store.UpsertPathTree(ctx, tree)
}

// runVKFSCommand executes a vkfs CLI command and returns output
func runVKFSCommand(ctx context.Context, command string, args ...string) (string, error) {
	// This would exec the actual vkfs binary
	// For now, stub implementation
	panic("implement me: exec vkfs CLI binary")
}

// ingestTestFile ingests a file into the vector store
func ingestTestFile(t *testing.T, store VectorStore, path string, content string) {
	t.Helper()

	// Split content into chunks (simple line-based chunking for tests)
	lines := splitIntoChunks(content, 100) // 100 chars per chunk
	chunks := make([]Chunk, len(lines))

	for i, line := range lines {
		chunks[i] = Chunk{
			ID:         generateChunkID(path, i),
			PageSlug:   path,
			ChunkIndex: i,
			Text:       line,
			Embedding:  generateMockEmbedding(line),
		}
	}

	err := store.UpsertChunks(context.Background(), chunks)
	require.NoError(t, err)

	// Update PathTree to include this file
	updatePathTreeWithFile(t, store, path, int64(len(content)))
}

// ingestLargeCorpus ingests N chunks for performance testing
func ingestLargeCorpus(t *testing.T, store VectorStore, chunkCount int) {
	t.Helper()

	chunksPerFile := 10
	fileCount := chunkCount / chunksPerFile

	for i := 0; i < fileCount; i++ {
		path := generateTestPath(i)
		content := generateTestContent(chunksPerFile)
		ingestTestFile(t, store, path, content)
	}
}

// writeTestConfig writes a test config file
func writeTestConfig(t *testing.T, config string) {
	t.Helper()
	configPath := t.TempDir() + "/config.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	require.NoError(t, err)
	t.Setenv("VKFS_CONFIG", configPath)
}

// captureVKFSLogs captures log output from vkfs commands
func captureVKFSLogs(t *testing.T) string {
	t.Helper()
	// Implementation would capture stderr/log file
	return ""
}

// extractScore extracts the score value for a given path from search output
func extractScore(t *testing.T, output string, path string) float64 {
	t.Helper()
	// Parse output to extract score for the given path
	// Format: "/path/to/file.md — score: 0.85"
	panic("implement me: parse search output")
}

// Helper functions for test data generation

func generateLargeContent(size int) string {
	content := make([]byte, size)
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}
	return string(content)
}

func splitIntoChunks(content string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, content[i:end])
	}
	return chunks
}

func generateChunkID(path string, index int) string {
	return path + "#" + string(rune(index))
}

func generateMockEmbedding(text string) []float32 {
	// Simple deterministic embedding for testing
	embedding := make([]float32, 1536)
	for i := range embedding {
		embedding[i] = float32(len(text)+i) / 1000.0
	}
	return embedding
}

func generateTestPath(index int) string {
	return "/test/file" + string(rune(index)) + ".md"
}

func generateTestContent(chunkCount int) string {
	content := ""
	for i := 0; i < chunkCount; i++ {
		content += "This is test chunk " + string(rune(i)) + " with keyword content.\n"
	}
	return content
}

func updatePathTreeWithFile(t *testing.T, store VectorStore, path string, size int64) {
	t.Helper()

	tree, err := store.GetPathTree(context.Background())
	require.NoError(t, err)

	// Add file node to tree
	node := VirtualNode{
		Path:      path,
		Name:      extractFileName(path),
		IsDir:     false,
		Size:      size,
		ModTime:   time.Now(),
		VisibleTo: []string{"*"},
	}
	tree.Nodes[path] = node

	// Ensure parent directories exist
	ensureParentDirs(tree, path)

	err = store.UpsertPathTree(context.Background(), tree)
	require.NoError(t, err)
}

func extractFileName(path string) string {
	// Extract filename from path: "/test/doc.md" → "doc.md"
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

func ensureParentDirs(tree *PathTree, path string) {
	// Ensure all parent directories exist in tree
	// "/test/subdir/file.md" → ensure "/test" and "/test/subdir" exist
	parts := []string{}
	current := ""
	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			parts = append(parts, current+"/")
			current = ""
		} else {
			current += string(path[i])
		}
	}

	for _, dir := range parts {
		if _, exists := tree.Nodes[dir]; !exists {
			tree.Nodes[dir] = VirtualNode{
				Path:      dir,
				Name:      extractFileName(dir),
				IsDir:     true,
				ModTime:   time.Now(),
				VisibleTo: []string{"*"},
			}
		}
	}
}
