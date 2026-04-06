package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

// TestFileNavigation verifies ls command behavior across various scenarios
func TestFileNavigation(t *testing.T) {
	ctx := context.Background()

	t.Run("ls /nonexistent returns path not found error", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		output, err := runVKFSCommand(ctx, "ls", "/nonexistent")
		require.Error(t, err)
		assert.Contains(t, output, "path not found",
			"should return clear error for missing path")
	})

	t.Run("ls /path/to/file.md returns not a directory error", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest a file at /test/doc.md
		ingestTestFile(t, store, "/test/doc.md", "content")

		output, err := runVKFSCommand(ctx, "ls", "/test/doc.md")
		require.Error(t, err)
		assert.Contains(t, output, "not a directory",
			"should reject ls on file path")
	})

	t.Run("ls /path/to/dir returns list of child nodes", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest files: /test/a.md, /test/b.md, /test/subdir/c.md
		ingestTestFile(t, store, "/test/a.md", "content a")
		ingestTestFile(t, store, "/test/b.md", "content b")
		ingestTestFile(t, store, "/test/subdir/c.md", "content c")

		output, err := runVKFSCommand(ctx, "ls", "/test")
		require.NoError(t, err)

		assert.Contains(t, output, "a.md")
		assert.Contains(t, output, "b.md")
		assert.Contains(t, output, "subdir")
		assert.NotContains(t, output, "c.md", "should not show nested files")
	})
}

// TestCatCommand verifies cat behavior with lazy pointers and chunks
func TestCatCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("cat with lazy pointer fetches from S3", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		s3Store := setupMockS3(t).(*MockS3Store) // need Put method
		defer cleanupZillizCollection(t, store)

		// Upload large file to S3 and create lazy pointer
		largeContent := generateLargeContent(2 * 1024 * 1024) // 2MB
		s3Key := "test/large-file.md"
		err := s3Store.Put(ctx, s3Key, []byte(largeContent))
		require.NoError(t, err)

		lazyPointer := vfs.LazyPointer{
			PageSlug:    "/test/large-file.md",
			ExternalURL: "s3://test-bucket/" + s3Key,
			Size:        int64(len(largeContent)),
		}
		err = store.UpsertLazyPointer(ctx, lazyPointer)
		require.NoError(t, err)

		// Cat should fetch from S3
		output, err := runVKFSCommand(ctx, "cat", "/test/large-file.md")
		require.NoError(t, err)
		assert.Equal(t, largeContent, output)
	})

	t.Run("S3 unavailable retries 3 times then fails", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		_ = setupFailingS3(t, 5) // Fail 5 times (more than 3 retries)
		defer cleanupZillizCollection(t, store)

		lazyPointer := vfs.LazyPointer{
			PageSlug:    "/test/unavailable.md",
			ExternalURL: "s3://test-bucket/unavailable.md",
			Size:        1000,
		}
		err := store.UpsertLazyPointer(ctx, lazyPointer)
		require.NoError(t, err)

		output, err := runVKFSCommand(ctx, "cat", "/test/unavailable.md")
		require.Error(t, err)
		assert.Contains(t, output, "retries exhausted",
			"should indicate retry failure")
	})

	t.Run("cat without lazy pointer fetches and validates chunks", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest file with 5 chunks (index 0-4)
		chunks := []vfs.Chunk{
			{ID: "1", PageSlug: "/test/doc.md", ChunkIndex: 0, Text: "chunk 0\n"},
			{ID: "2", PageSlug: "/test/doc.md", ChunkIndex: 1, Text: "chunk 1\n"},
			{ID: "3", PageSlug: "/test/doc.md", ChunkIndex: 2, Text: "chunk 2\n"},
			{ID: "4", PageSlug: "/test/doc.md", ChunkIndex: 3, Text: "chunk 3\n"},
			{ID: "5", PageSlug: "/test/doc.md", ChunkIndex: 4, Text: "chunk 4\n"},
		}
		err := store.UpsertChunks(ctx, chunks)
		require.NoError(t, err)

		output, err := runVKFSCommand(ctx, "cat", "/test/doc.md")
		require.NoError(t, err)
		assert.Equal(t, "chunk 0\nchunk 1\nchunk 2\nchunk 3\nchunk 4\n", output)
	})

	t.Run("missing chunk fails with clear error", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest chunks with gap: 0, 1, 3, 4 (missing 2)
		chunks := []vfs.Chunk{
			{ID: "1", PageSlug: "/test/doc.md", ChunkIndex: 0, Text: "chunk 0\n"},
			{ID: "2", PageSlug: "/test/doc.md", ChunkIndex: 1, Text: "chunk 1\n"},
			{ID: "4", PageSlug: "/test/doc.md", ChunkIndex: 3, Text: "chunk 3\n"},
			{ID: "5", PageSlug: "/test/doc.md", ChunkIndex: 4, Text: "chunk 4\n"},
		}
		err := store.UpsertChunks(ctx, chunks)
		require.NoError(t, err)

		output, err := runVKFSCommand(ctx, "cat", "/test/doc.md")
		require.Error(t, err)
		assert.Contains(t, output, "Chunk 2 missing",
			"should identify missing chunk index")
	})

	t.Run("duplicate chunk index fails with clear error", func(t *testing.T) {
		store := setupZillizWithSampleData(t)
		defer cleanupZillizCollection(t, store)

		// Ingest chunks with duplicate index 2
		chunks := []vfs.Chunk{
			{ID: "1", PageSlug: "/test/doc.md", ChunkIndex: 0, Text: "chunk 0\n"},
			{ID: "2", PageSlug: "/test/doc.md", ChunkIndex: 1, Text: "chunk 1\n"},
			{ID: "3", PageSlug: "/test/doc.md", ChunkIndex: 2, Text: "chunk 2a\n"},
			{ID: "4", PageSlug: "/test/doc.md", ChunkIndex: 2, Text: "chunk 2b\n"},
			{ID: "5", PageSlug: "/test/doc.md", ChunkIndex: 3, Text: "chunk 3\n"},
		}
		err := store.UpsertChunks(ctx, chunks)
		require.NoError(t, err)

		output, err := runVKFSCommand(ctx, "cat", "/test/doc.md")
		require.Error(t, err)
		assert.Contains(t, output, "Duplicate chunk index 2",
			"should detect duplicate chunk")
	})
}
