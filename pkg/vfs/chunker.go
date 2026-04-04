package vfs

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

const defaultMaxChunkSize = 2000

// SplitFile splits file content into chunks.
// For Markdown files, splits on paragraph boundaries (double newline).
// For other text files, splits on line boundaries at chunk size limit.
func SplitFile(filename string, content []byte, maxChunkSize int) []Chunk {
	if maxChunkSize <= 0 {
		maxChunkSize = defaultMaxChunkSize
	}

	text := string(content)
	if text == "" {
		return nil
	}

	var segments []string
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".md", ".markdown":
		segments = splitParagraphs(text, maxChunkSize)
	default:
		segments = splitLines(text, maxChunkSize)
	}

	chunks := make([]Chunk, len(segments))
	for i, seg := range segments {
		chunks[i] = Chunk{
			ID:         chunkID(filename, i),
			PageSlug:   filename,
			ChunkIndex: i,
			Text:       seg,
		}
	}

	return chunks
}

// splitParagraphs splits text on paragraph boundaries, respecting max size.
func splitParagraphs(text string, maxSize int) []string {
	paragraphs := strings.Split(text, "\n\n")
	var segments []string
	var current strings.Builder

	for _, p := range paragraphs {
		// If paragraph itself exceeds max size, split it by lines
		if len(p) > maxSize {
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
			segments = append(segments, splitLines(p, maxSize)...)
			continue
		}

		if current.Len()+len(p)+2 > maxSize && current.Len() > 0 {
			segments = append(segments, strings.TrimSpace(current.String()))
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(p)
	}

	if current.Len() > 0 {
		segments = append(segments, strings.TrimSpace(current.String()))
	}

	return segments
}

// splitLines splits text on line boundaries, respecting max size.
func splitLines(text string, maxSize int) []string {
	lines := strings.Split(text, "\n")
	var segments []string
	var current strings.Builder

	for _, line := range lines {
		if current.Len()+len(line)+1 > maxSize && current.Len() > 0 {
			segments = append(segments, strings.TrimSpace(current.String()))
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		segments = append(segments, strings.TrimSpace(current.String()))
	}

	return segments
}

// chunkID generates a deterministic chunk ID from filename and index.
func chunkID(filename string, index int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", filename, index)))
	return fmt.Sprintf("%x", h[:16])
}
