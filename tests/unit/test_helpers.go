package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateChunkIntegrity checks that chunks form a valid contiguous sequence
func validateChunkIntegrity(chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Build index map to detect duplicates
	indexMap := make(map[int]bool)
	maxIndex := -1

	for _, chunk := range chunks {
		if indexMap[chunk.ChunkIndex] {
			return errors.New("Duplicate chunk index " + string(rune(chunk.ChunkIndex)))
		}
		indexMap[chunk.ChunkIndex] = true

		if chunk.ChunkIndex > maxIndex {
			maxIndex = chunk.ChunkIndex
		}
	}

	// Verify contiguous sequence starting from 0
	for i := 0; i <= maxIndex; i++ {
		if !indexMap[i] {
			return errors.New("Chunk " + string(rune(i)) + " missing")
		}
	}

	return nil
}

// loadConfig parses and validates VKFS configuration
func loadConfig(configYAML string) (*Config, error) {
	if configYAML == "" {
		return nil, errors.New("config file is empty")
	}

	// Parse YAML
	var cfg Config
	err := yaml.Unmarshal([]byte(configYAML), &cfg)
	if err != nil {
		// Extract line number from YAML error if available
		return nil, err
	}

	// Validate vectorstore backend
	if cfg.VectorStore.Backend != "zilliz" && cfg.VectorStore.Backend != "qdrant" {
		return nil, errors.New("unsupported vectorstore backend: " + cfg.VectorStore.Backend)
	}

	// Validate embedding provider
	if cfg.Embedding.Provider != "openai" && cfg.Embedding.Provider != "cohere" {
		return nil, errors.New("provider '" + cfg.Embedding.Provider + "' not supported")
	}

	// Interpolate environment variables
	cfg = interpolateEnvVars(cfg)

	return &cfg, nil
}

// buildTestPathTree constructs a PathTree from a list of file paths
func buildTestPathTree(paths []string) *PathTree {
	tree := &PathTree{
		Nodes:   make(map[string]VirtualNode),
		Version: "1.0",
	}

	// Add root
	tree.Nodes["/"] = VirtualNode{
		Path:  "/",
		Name:  "/",
		IsDir: true,
	}

	// Add each path
	for _, path := range paths {
		// Add file node
		tree.Nodes[path] = VirtualNode{
			Path:  path,
			Name:  extractFileName(path),
			IsDir: false,
		}

		// Add parent directories
		ensureParentDirs(tree, path)
	}

	return tree
}

// PathTree methods for testing

func (t *PathTree) ListChildren(dirPath string) []string {
	var children []string

	for path, node := range t.Nodes {
		if path == dirPath {
			continue
		}

		// Check if this node is an immediate child of dirPath
		if isImmediateChild(dirPath, path) {
			children = append(children, node.Name)
		}
	}

	return children
}

func (t *PathTree) Find(rootPath string, pattern string) []string {
	var results []string

	for path, node := range t.Nodes {
		// Skip directories
		if node.IsDir {
			continue
		}

		// Check if path is under rootPath
		if !hasPrefix(path, rootPath) {
			continue
		}

		// Check if filename matches pattern
		if matchGlob(node.Name, pattern) {
			results = append(results, path)
		}
	}

	return results
}

func (t *PathTree) GetNode(path string) (VirtualNode, bool) {
	node, exists := t.Nodes[path]
	return node, exists
}

// Helper functions

func isImmediateChild(parent string, path string) bool {
	if !hasPrefix(path, parent) {
		return false
	}

	// Remove parent prefix
	relative := path[len(parent):]
	if relative[0] == '/' {
		relative = relative[1:]
	}

	// Check if there are no more slashes (immediate child)
	for i := 0; i < len(relative); i++ {
		if relative[i] == '/' {
			return false
		}
	}

	return true
}

func hasPrefix(path string, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix
}

func matchGlob(name string, pattern string) bool {
	// Simple glob matching: *.ext
	if len(pattern) > 2 && pattern[0] == '*' && pattern[1] == '.' {
		ext := pattern[2:]
		return hasSuffix(name, "."+ext)
	}
	return name == pattern
}

func hasSuffix(s string, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}

func extractFileName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

func ensureParentDirs(tree *PathTree, path string) {
	// Build list of parent directories
	var dirs []string
	current := ""

	for i := 1; i < len(path); i++ {
		if path[i] == '/' {
			current += "/"
			dirs = append(dirs, current)
			current = ""
		} else {
			current += string(path[i])
		}
	}

	// Add each parent directory if it doesn't exist
	for _, dir := range dirs {
		if _, exists := tree.Nodes[dir]; !exists {
			tree.Nodes[dir] = VirtualNode{
				Path:  dir,
				Name:  extractFileName(dir),
				IsDir: true,
			}
		}
	}
}

func interpolateEnvVars(cfg Config) Config {
	// Replace ${VAR} with environment variable values
	// Simplified implementation for testing
	return cfg
}

// Type definitions for testing (these would normally be in pkg/)

type Chunk struct {
	ID         string
	PageSlug   string
	ChunkIndex int
	Text       string
	Embedding  []float32
}

type VirtualNode struct {
	Path      string
	Name      string
	IsDir     bool
	Size      int64
	ModTime   time.Time
	VisibleTo []string
	Metadata  map[string]string
}

type PathTree struct {
	Nodes    map[string]VirtualNode
	Version  string
	Metadata map[string]string
}

type Config struct {
	VectorStore struct {
		Backend string
		Zilliz  struct {
			Endpoint   string
			APIKey     string
			Collection string
		}
		Qdrant struct {
			Endpoint   string
			APIKey     string
			Collection string
		}
	}
	Embedding struct {
		Provider string
		OpenAI   struct {
			APIKey string
			Model  string
		}
		Cohere struct {
			APIKey string
			Model  string
		}
	}
}
