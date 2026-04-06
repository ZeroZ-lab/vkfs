package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

// TestChunkIntegrity verifies chunk validation logic in isolation
func TestChunkIntegrity(t *testing.T) {
	t.Run("valid contiguous chunks passes validation", func(t *testing.T) {
		chunks := []vfs.Chunk{
			{ChunkIndex: 0, Text: "a"},
			{ChunkIndex: 1, Text: "b"},
			{ChunkIndex: 2, Text: "c"},
		}
		err := validateChunkIntegrity(chunks)
		require.NoError(t, err)
	})

	t.Run("non-contiguous chunks fails with clear error", func(t *testing.T) {
		// Chunks: 0, 1, 3, 5 — gaps at 2 and 4
		chunks := []vfs.Chunk{
			{ChunkIndex: 0, Text: "a"},
			{ChunkIndex: 1, Text: "b"},
			{ChunkIndex: 3, Text: "d"},
			{ChunkIndex: 5, Text: "f"},
		}
		err := validateChunkIntegrity(chunks)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Chunk 2 missing",
			"should identify the first missing chunk")
	})

	t.Run("chunks not starting from 0 fails with clear error", func(t *testing.T) {
		chunks := []vfs.Chunk{
			{ChunkIndex: 5, Text: "a"},
			{ChunkIndex: 6, Text: "b"},
		}
		err := validateChunkIntegrity(chunks)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Chunk 0 missing",
			"should detect missing chunk 0 even when sequence is otherwise contiguous")
	})

	t.Run("duplicate chunk index fails with clear error", func(t *testing.T) {
		chunks := []vfs.Chunk{
			{ChunkIndex: 0, Text: "a"},
			{ChunkIndex: 1, Text: "b"},
			{ChunkIndex: 2, Text: "c"},
			{ChunkIndex: 2, Text: "c-duplicate"},
			{ChunkIndex: 3, Text: "d"},
		}
		err := validateChunkIntegrity(chunks)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Duplicate chunk index 2")
	})

	t.Run("empty chunks list is valid", func(t *testing.T) {
		err := validateChunkIntegrity([]vfs.Chunk{})
		require.NoError(t, err)
	})

	t.Run("single chunk at index 0 is valid", func(t *testing.T) {
		chunks := []vfs.Chunk{
			{ChunkIndex: 0, Text: "only chunk"},
		}
		err := validateChunkIntegrity(chunks)
		require.NoError(t, err)
	})
}

// TestPathTreeOperations verifies in-memory path tree operations
func TestPathTreeOperations(t *testing.T) {
	t.Run("ls on directory returns immediate children only", func(t *testing.T) {
		tree := buildTestPathTree([]string{
			"/a/b.md",
			"/a/c.md",
			"/a/d/e.md",
		})

		children := listChildren(tree, "/a")
		assert.ElementsMatch(t, children, []string{"b.md", "c.md", "d"},
			"should return files and immediate subdirs")
		assert.NotContains(t, children, "e.md",
			"should not return nested files")
	})

	t.Run("find with glob pattern filters correctly", func(t *testing.T) {
		tree := buildTestPathTree([]string{
			"/docs/readme.md",
			"/docs/guide.pdf",
			"/code/main.go",
			"/code/types.go",
		})

		results := findPaths(tree, "/", "*.md")
		assert.ElementsMatch(t, results, []string{"/docs/readme.md"})

		results = findPaths(tree, "/", "*.go")
		assert.ElementsMatch(t, results, []string{"/code/main.go", "/code/types.go"})
	})

	t.Run("node lookup is case-sensitive", func(t *testing.T) {
		tree := buildTestPathTree([]string{"/test/File.md"})

		node, exists := getNode(tree, "/test/File.md")
		assert.True(t, exists)
		assert.Equal(t, "File.md", node.Name)

		_, exists = getNode(tree, "/test/file.md")
		assert.False(t, exists, "path lookup should be case-sensitive")
	})
}

// TestConfigLoading verifies config parsing and validation
func TestConfigLoading(t *testing.T) {
	t.Run("empty config file returns clear error", func(t *testing.T) {
		_, err := loadConfig("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config file is empty")
	})

	t.Run("invalid YAML syntax returns error with line number", func(t *testing.T) {
		config := `
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: [invalid yaml
`
		_, err := loadConfig(config)
		require.Error(t, err)
		// Should include line number for debugging
		assert.Regexp(t, `line \d+`, err.Error(),
			"error should include line number for invalid YAML")
	})

	t.Run("unsupported embedding provider returns clear error", func(t *testing.T) {
		config := `
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: "https://example.zilliz.com"
    api_key: "key"
    collection: "vkfs"
embedding:
  provider: xyz
`
		_, err := loadConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "provider 'xyz' not supported")
	})

	t.Run("valid config with environment variable interpolation", func(t *testing.T) {
		t.Setenv("ZILLIZ_API_KEY", "test-api-key")
		t.Setenv("OPENAI_API_KEY", "test-openai-key")

		config := `
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: "https://example.zilliz.com"
    api_key: "${ZILLIZ_API_KEY}"
    collection: "vkfs_chunks"
embedding:
  provider: openai
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"
`
		cfg, err := loadConfig(config)
		require.NoError(t, err)
		assert.Equal(t, "test-api-key", cfg.VectorStore.Zilliz.APIKey)
		assert.Equal(t, "test-openai-key", cfg.Embedding.OpenAI.APIKey)
	})

	t.Run("unsupported vectorstore backend returns clear error", func(t *testing.T) {
		config := `
vectorstore:
  backend: pinecone
`
		_, err := loadConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pinecone")
	})
}
