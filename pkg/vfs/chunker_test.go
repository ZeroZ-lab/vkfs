package vfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitFile_Empty(t *testing.T) {
	chunks := SplitFile("empty.md", []byte{}, 1000)
	assert.Nil(t, chunks)
}

func TestSplitFile_DefaultMaxSize(t *testing.T) {
	content := []byte("short text")
	chunks := SplitFile("test.txt", content, 0)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "short text", chunks[0].Text)
}

func TestSplitFile_Markdown(t *testing.T) {
	content := []byte("# Header\n\nParagraph one.\n\nParagraph two.\n\nParagraph three.")
	chunks := SplitFile("doc.md", content, 1000)
	assert.Len(t, chunks, 1)
	// The entire content fits in a single chunk since maxChunkSize=1000
	assert.Contains(t, chunks[0].Text, "# Header")
	assert.Contains(t, chunks[0].Text, "Paragraph three")
}

func TestSplitFile_TextByLines(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4")
	chunks := SplitFile("data.txt", content, 15)
	assert.GreaterOrEqual(t, len(chunks), 2, "should split into multiple chunks")

	// Verify all text is preserved
	var allText string
	for _, c := range chunks {
		allText += c.Text
	}
	assert.Contains(t, allText, "line1")
	assert.Contains(t, allText, "line4")
}

func TestSplitFile_LargeParagraph(t *testing.T) {
	// Single paragraph exceeding maxChunkSize should be split by lines
	longPara := make([]byte, 5000)
	for i := range longPara {
		longPara[i] = 'a'
	}
	// Insert newlines to create lines
	for i := 2000; i < 5000; i += 2000 {
		longPara[i] = '\n'
	}

	chunks := SplitFile("big.md", longPara, 1000)
	assert.GreaterOrEqual(t, len(chunks), 2, "large paragraph should be split")
}

func TestSplitFile_ChunkIDs(t *testing.T) {
	content := []byte("first\n\nsecond\n\nthird")
	chunks := SplitFile("doc.md", content, 10)

	assert.Len(t, chunks, 3)
	// IDs should be deterministic
	chunks2 := SplitFile("doc.md", content, 10)
	assert.Equal(t, chunks[0].ID, chunks2[0].ID)
	assert.Equal(t, chunks[1].ID, chunks2[1].ID)
}

func TestSplitFile_ChunkIndex(t *testing.T) {
	content := []byte("a\n\nb\n\nc")
	chunks := SplitFile("f.md", content, 5)
	for i, c := range chunks {
		assert.Equal(t, i, c.ChunkIndex)
	}
	assert.Equal(t, "f.md", chunks[0].PageSlug)
}

func TestSplitFile_MarkdownExtension(t *testing.T) {
	content := []byte("para one\n\npara two")
	// .markdown extension should also use paragraph splitting
	chunks := SplitFile("doc.markdown", content, 10)
	assert.Len(t, chunks, 2)
}

func TestSplitFile_NonMarkdownUsesLineSplit(t *testing.T) {
	content := []byte("line1\nline2\nline3")
	// .go file should use line-based splitting
	chunks := SplitFile("code.go", content, 5)
	assert.GreaterOrEqual(t, len(chunks), 2)
}
