package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBootstrapFlow verifies the complete bootstrap sequence
func TestBootstrapFlow(t *testing.T) {
	ctx := context.Background()

	t.Run("init creates empty PathTree with root node", func(t *testing.T) {
		store := setupEmptyZillizCollection(t)
		defer cleanupZillizCollection(t, store)

		// Run vkfs-admin init
		err := runVKFSAdminInit(ctx, store)
		require.NoError(t, err, "vkfs-admin init should succeed")

		// Verify PathTree exists with root node
		tree, err := store.GetPathTree(ctx)
		require.NoError(t, err)
		assert.NotNil(t, tree.Nodes["/"], "root node should exist")
		assert.True(t, tree.Nodes["/"].IsDir, "root should be a directory")
	})

	t.Run("ls / after init returns empty directory", func(t *testing.T) {
		store := setupEmptyZillizCollection(t)
		defer cleanupZillizCollection(t, store)

		err := runVKFSAdminInit(ctx, store)
		require.NoError(t, err)

		// Run vkfs ls /
		output, err := runVKFSCommand(ctx, "ls", "/")
		require.NoError(t, err)
		assert.Empty(t, output, "root directory should be empty after init")
	})

	t.Run("ls / before init returns clear error", func(t *testing.T) {
		store := setupEmptyZillizCollection(t)
		defer cleanupZillizCollection(t, store)

		// Run vkfs ls / WITHOUT init
		output, err := runVKFSCommand(ctx, "ls", "/")
		require.Error(t, err, "should fail before init")
		assert.Contains(t, output, "Run vkfs-admin init first",
			"error message should guide user to init")
	})
}

// TestConfigValidation verifies fail-fast behavior on invalid config
func TestConfigValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("missing ZILLIZ_API_KEY fails at startup", func(t *testing.T) {
		// Unset API key
		t.Setenv("ZILLIZ_API_KEY", "")

		_, err := runVKFSCommand(ctx, "ls", "/")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ZILLIZ_API_KEY",
			"error should mention missing API key")
	})

	t.Run("invalid Zilliz endpoint fails with clear error", func(t *testing.T) {
		config := `
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: "https://invalid-endpoint-does-not-exist.example.com"
    api_key: "test-key"
    collection: "vkfs_chunks"
`
		writeTestConfig(t, config)

		_, err := runVKFSCommand(ctx, "ls", "/")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection",
			"error should indicate connection failure")
	})

	t.Run("embedding model mismatch fails fast", func(t *testing.T) {
		store := setupZillizWithEmbedding(t, "text-embedding-3-small")
		defer cleanupZillizCollection(t, store)

		// Change config to different model
		config := `
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: "${ZILLIZ_ENDPOINT}"
    api_key: "${ZILLIZ_API_KEY}"
    collection: "vkfs_chunks"
embedding:
  provider: cohere
  cohere:
    api_key: "${COHERE_API_KEY}"
    model: "embed-english-v3.0"
`
		writeTestConfig(t, config)

		_, err := runVKFSCommand(ctx, "ls", "/")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Embedding model mismatch",
			"should detect model mismatch")
	})
}
