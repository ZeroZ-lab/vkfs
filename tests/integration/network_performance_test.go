package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNetworkFailures verifies retry logic and error handling
func TestNetworkFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("Zilliz connection timeout during ls returns error with retry suggestion", func(t *testing.T) {
		store := setupZillizWithTimeout(t, 100*time.Millisecond)
		defer cleanupZillizCollection(t, store)

		output, err := runVKFSCommand(ctx, "ls", "/")
		require.Error(t, err)
		assert.Contains(t, output, "connection timeout",
			"should indicate timeout")
		assert.Contains(t, output, "retry",
			"should suggest retry to user")
	})

	t.Run("S3 timeout during cat retries 3x then fails", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		s3Store := setupTimeoutS3(t, 5*time.Second) // Timeout exceeds retry threshold
		defer cleanupZillizCollection(t, store)

		lazyPointer := LazyPointer{
			PageSlug:    "/test/timeout.md",
			ExternalURL: "s3://test-bucket/timeout.md",
			Size:        1000,
		}
		err := store.UpsertLazyPointer(ctx, lazyPointer)
		require.NoError(t, err)

		start := time.Now()
		output, err := runVKFSCommand(ctx, "cat", "/test/timeout.md")
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.Contains(t, output, "retries exhausted")

		// Verify exponential backoff: ~1s + 2s + 4s = ~7s total
		assert.Greater(t, elapsed, 6*time.Second,
			"should have attempted 3 retries with exponential backoff")
		assert.Less(t, elapsed, 10*time.Second,
			"should not exceed reasonable retry window")
	})

	t.Run("OpenAI API timeout during search returns clear error", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		embedder := setupTimeoutEmbedder(t, 5*time.Second)
		defer cleanupZillizCollection(t, store)

		ingestTestFile(t, store, "/test/doc.md", "content")

		output, err := runVKFSCommand(ctx, "search", "query", "/test")
		require.Error(t, err)
		assert.Contains(t, output, "OpenAI API timeout",
			"should indicate embedding provider timeout")
	})

	t.Run("transient network error retries successfully", func(t *testing.T) {
		store := setupZillizWithTransientFailure(t, 2) // Fail first 2 attempts
		defer cleanupZillizCollection(t, store)

		// Should succeed on 3rd attempt
		output, err := runVKFSCommand(ctx, "ls", "/")
		require.NoError(t, err)

		// Verify logs show retry attempts
		logs := captureVKFSLogs(t)
		assert.Contains(t, logs, "retry attempt 1")
		assert.Contains(t, logs, "retry attempt 2")
	})
}

// TestLargeCorpusPerformance verifies grep performance targets
func TestLargeCorpusPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	ctx := context.Background()

	t.Run("10K chunks: grep completes in <200ms", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest 10K chunks across 1K files
		ingestLargeCorpus(t, store, 10000)

		start := time.Now()
		_, err := runVKFSCommand(ctx, "grep", "keyword", "/")
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, elapsed, 200*time.Millisecond,
			"grep on 10K chunks should complete in <200ms")
	})

	t.Run("100K chunks: grep completes in <500ms", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest 100K chunks across 10K files
		ingestLargeCorpus(t, store, 100000)

		start := time.Now()
		_, err := runVKFSCommand(ctx, "grep", "keyword", "/")
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, elapsed, 500*time.Millisecond,
			"grep on 100K chunks should complete in <500ms")
	})

	t.Run("1M chunks: document actual latency", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest 1M chunks across 100K files
		ingestLargeCorpus(t, store, 1000000)

		start := time.Now()
		_, err := runVKFSCommand(ctx, "grep", "keyword", "/")
		elapsed := time.Since(start)

		require.NoError(t, err)
		t.Logf("grep on 1M chunks completed in %v", elapsed)

		// No assertion — just document actual performance
		// If > 500ms, this is expected and acceptable for v1
	})
}

// TestEndToEndWorkflows verifies complete user journeys
func TestEndToEndWorkflows(t *testing.T) {
	ctx := context.Background()

	t.Run("bootstrap → ingest → query workflow", func(t *testing.T) {
		store := setupEmptyZillizCollection(t)
		defer cleanupZillizCollection(t, store)

		// Step 1: vkfs-admin init
		err := runVKFSAdminInit(ctx, store)
		require.NoError(t, err)

		// Step 2: Ingest 10 test documents (~50 chunks)
		for i := 0; i < 10; i++ {
			path := "/test/doc" + string(rune(i)) + ".md"
			content := generateTestContent(5) // 5 chunks per file
			ingestTestFile(t, store, path, content)
		}

		// Step 3: vkfs ls /
		output, err := runVKFSCommand(ctx, "ls", "/")
		require.NoError(t, err)
		assert.Contains(t, output, "test")

		// Step 4: vkfs cat /test/doc1.md
		output, err = runVKFSCommand(ctx, "cat", "/test/doc1.md")
		require.NoError(t, err)
		assert.NotEmpty(t, output)

		// Step 5: vkfs grep "keyword" /test
		output, err = runVKFSCommand(ctx, "grep", "keyword", "/test")
		require.NoError(t, err)
		assert.Contains(t, output, "/test/")

		// Step 6: vkfs search "semantic query" /test
		output, err = runVKFSCommand(ctx, "search", "semantic query", "/test")
		require.NoError(t, err)
		assert.Contains(t, output, "score:")
	})

	t.Run("error recovery: ls before init → init → ls succeeds", func(t *testing.T) {
		store := setupEmptyZillizCollection(t)
		defer cleanupZillizCollection(t, store)

		// Step 1: ls / before init → error
		output, err := runVKFSCommand(ctx, "ls", "/")
		require.Error(t, err)
		assert.Contains(t, output, "Run vkfs-admin init first")

		// Step 2: vkfs-admin init
		err = runVKFSAdminInit(ctx, store)
		require.NoError(t, err)

		// Step 3: ls / after init → success
		output, err = runVKFSCommand(ctx, "ls", "/")
		require.NoError(t, err)
		assert.Empty(t, output, "should return empty directory")
	})

	t.Run("embedding model migration fails fast", func(t *testing.T) {
		store := setupZillizWithEmbedding(t, "text-embedding-3-small")
		defer cleanupZillizCollection(t, store)

		// Step 1: Init KB with OpenAI text-embedding-3-small
		err := runVKFSAdminInit(ctx, store)
		require.NoError(t, err)

		// Step 2: Change config to Cohere embed-english-v3.0
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

		// Step 3: ls / should fail fast with mismatch error
		output, err := runVKFSCommand(ctx, "ls", "/")
		require.Error(t, err)
		assert.Contains(t, output, "Embedding model mismatch",
			"should detect model change and fail fast")
	})
}
