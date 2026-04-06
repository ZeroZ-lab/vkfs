package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGrepCommand verifies two-stage grep filtering (BM25 + regex)
func TestGrepCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("grep with matches returns formatted output", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest files with keyword "termination"
		ingestTestFile(t, store, "/legal/contract.md", "This agreement includes a termination clause.\nThe termination period is 30 days.")
		ingestTestFile(t, store, "/legal/policy.md", "No termination without cause.")

		output, err := runVKFSCommand(ctx, "grep", "termination", "/legal")
		require.NoError(t, err)

		// Verify format: path:line — matched text
		assert.Contains(t, output, "/legal/contract.md:1 — This agreement includes a termination clause.")
		assert.Contains(t, output, "/legal/contract.md:2 — The termination period is 30 days.")
		assert.Contains(t, output, "/legal/policy.md:1 — No termination without cause.")
	})

	t.Run("grep with no matches returns empty", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		ingestTestFile(t, store, "/test/doc.md", "This document has no matching keywords.")

		output, err := runVKFSCommand(ctx, "grep", "nonexistent", "/test")
		require.NoError(t, err)
		assert.Empty(t, output, "should return empty for no matches")

		// Verify BM25 was called (check logs)
		logs := captureVKFSLogs(t)
		assert.Contains(t, logs, "BM25 coarse filter",
			"should log BM25 call even when no results")
	})

	t.Run("grep uses BM25 coarse filter then regex fine filter", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest 100 files, 50 contain "test", 10 contain "test-123"
		for i := 0; i < 50; i++ {
			ingestTestFile(t, store, "/data/file"+string(rune(i))+".md", "This is a test document.")
		}
		for i := 50; i < 60; i++ {
			ingestTestFile(t, store, "/data/file"+string(rune(i))+".md", "This is test-123 document.")
		}
		for i := 60; i < 100; i++ {
			ingestTestFile(t, store, "/data/file"+string(rune(i))+".md", "No keyword here.")
		}

		// Grep for regex pattern "test-\d+"
		output, err := runVKFSCommand(ctx, "grep", "test-\\d+", "/data")
		require.NoError(t, err)

		// Should only match test-123 files (10 results)
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Equal(t, 10, len(lines), "should match only test-123 files")

		// Verify logs show BM25 returned top-50, then regex filtered to 10
		logs := captureVKFSLogs(t)
		assert.Contains(t, logs, "BM25 returned 50 candidates")
		assert.Contains(t, logs, "Regex filtered to 10 results")
	})
}

// TestSearchCommand verifies semantic search via embedding + vector search
func TestSearchCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("search embeds query and returns top-K results with scores", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		_ = setupMockEmbedder(t) // will be used by runVKFSCommand
		defer cleanupZillizCollection(t, store)

		// Ingest semantically related documents
		ingestTestFile(t, store, "/kb/auth.md", "User authentication uses JWT tokens.")
		ingestTestFile(t, store, "/kb/security.md", "Security best practices for token storage.")
		ingestTestFile(t, store, "/kb/unrelated.md", "This document is about cats.")

		output, err := runVKFSCommand(ctx, "search", "how to authenticate users", "/kb")
		require.NoError(t, err)

		// Verify top results are auth-related with scores
		assert.Contains(t, output, "/kb/auth.md")
		assert.Contains(t, output, "score:")
		assert.Contains(t, output, "/kb/security.md")

		// Unrelated doc should have lower score or not appear in top-K
		if strings.Contains(output, "/kb/unrelated.md") {
			// If it appears, its score should be significantly lower
			authScore := extractScore(t, output, "/kb/auth.md")
			unrelatedScore := extractScore(t, output, "/kb/unrelated.md")
			assert.Greater(t, authScore, unrelatedScore*2,
				"auth doc should have much higher score than unrelated doc")
		}
	})

	t.Run("search with empty query returns error or all results", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		ingestTestFile(t, store, "/test/doc.md", "content")

		output, err := runVKFSCommand(ctx, "search", "", "/test")

		// Behavior depends on implementation choice
		// Either: error for empty query
		if err != nil {
			assert.Contains(t, err.Error(), "empty query")
		} else {
			// Or: returns all results (verify this is intentional)
			assert.Contains(t, output, "/test/doc.md")
		}
	})
}
