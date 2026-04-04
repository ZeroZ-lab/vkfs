package vfs

import (
	"testing"
)

func TestValidateChunkIntegrity(t *testing.T) {
	tests := []struct {
		name    string
		chunks  []Chunk
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty chunks",
			chunks:  []Chunk{},
			wantErr: false,
		},
		{
			name: "valid contiguous chunks",
			chunks: []Chunk{
				{ChunkIndex: 0, Text: "a"},
				{ChunkIndex: 1, Text: "b"},
				{ChunkIndex: 2, Text: "c"},
			},
			wantErr: false,
		},
		{
			name: "missing chunk in middle",
			chunks: []Chunk{
				{ChunkIndex: 0, Text: "a"},
				{ChunkIndex: 1, Text: "b"},
				{ChunkIndex: 3, Text: "d"},
			},
			wantErr: true,
			errMsg:  "Chunk 2 missing",
		},
		{
			name: "duplicate chunk index",
			chunks: []Chunk{
				{ChunkIndex: 0, Text: "a"},
				{ChunkIndex: 1, Text: "b"},
				{ChunkIndex: 1, Text: "b-duplicate"},
			},
			wantErr: true,
			errMsg:  "Duplicate chunk index 1",
		},
		{
			name: "chunks not starting from 0",
			chunks: []Chunk{
				{ChunkIndex: 5, Text: "a"},
				{ChunkIndex: 6, Text: "b"},
			},
			wantErr: true,
			errMsg:  "Chunk 0 missing",
		},
		{
			name: "single chunk at index 0",
			chunks: []Chunk{
				{ChunkIndex: 0, Text: "only chunk"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChunkIntegrity(tt.chunks)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateChunkIntegrity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("validateChunkIntegrity() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestExtractKeyFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "s3 URL with bucket and key",
			url:  "s3://my-bucket/path/to/file.txt",
			want: "path/to/file.txt",
		},
		{
			name: "s3 URL with nested path",
			url:  "s3://bucket/a/b/c/file.md",
			want: "a/b/c/file.md",
		},
		{
			name: "plain path",
			url:  "path/to/file.txt",
			want: "path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractKeyFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractKeyFromURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
