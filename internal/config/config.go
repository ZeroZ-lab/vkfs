package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the VKFS configuration
type Config struct {
	VectorStore struct {
		Backend string `yaml:"backend"` // "zilliz" or "qdrant"
		Zilliz  struct {
			Endpoint   string `yaml:"endpoint"`
			APIKey     string `yaml:"api_key"`
			Collection string `yaml:"collection"`
		} `yaml:"zilliz"`
		Qdrant struct {
			Endpoint   string `yaml:"endpoint"`
			APIKey     string `yaml:"api_key"`
			Collection string `yaml:"collection"`
		} `yaml:"qdrant"`
	} `yaml:"vectorstore"`

	Embedding struct {
		Provider string `yaml:"provider"` // "openai" or "cohere"
		OpenAI   struct {
			APIKey string `yaml:"api_key"`
			Model  string `yaml:"model"`
		} `yaml:"openai"`
		Cohere struct {
			APIKey string `yaml:"api_key"`
			Model  string `yaml:"model"`
		} `yaml:"cohere"`
	} `yaml:"embedding"`

	ExternalStore struct {
		Backend string `yaml:"backend"` // "s3" or "local"
		S3      struct {
			Bucket string `yaml:"bucket"`
			Region string `yaml:"region"`
		} `yaml:"s3"`
		Local struct {
			Path string `yaml:"path"`
		} `yaml:"local"`
	} `yaml:"external_store"`

	Cache struct {
		Enabled   bool `yaml:"enabled"`
		MaxSizeMB int  `yaml:"max_size_mb"`
	} `yaml:"cache"`
}

// Load loads configuration from file path
func Load(path string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("config file is empty")
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Interpolate environment variables
	interpolateEnvVars(&cfg)

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadDefault loads config from default locations
func LoadDefault() (*Config, error) {
	// Check VKFS_CONFIG env var first
	if path := os.Getenv("VKFS_CONFIG"); path != "" {
		return Load(path)
	}

	// Try ~/.vkfs/config.yaml
	home, err := os.UserHomeDir()
	if err == nil {
		path := home + "/.vkfs/config.yaml"
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}

	return nil, fmt.Errorf("no config file found - set VKFS_CONFIG or create ~/.vkfs/config.yaml")
}

// Validate checks configuration validity
func (c *Config) Validate() error {
	// Validate vectorstore backend
	if c.VectorStore.Backend != "zilliz" && c.VectorStore.Backend != "qdrant" {
		return fmt.Errorf("unsupported vectorstore backend: %s (must be 'zilliz' or 'qdrant')", c.VectorStore.Backend)
	}

	// Validate backend-specific config
	if c.VectorStore.Backend == "zilliz" {
		if c.VectorStore.Zilliz.Endpoint == "" {
			return fmt.Errorf("zilliz.endpoint is required")
		}
		if c.VectorStore.Zilliz.APIKey == "" {
			return fmt.Errorf("zilliz.api_key is required (set ZILLIZ_API_KEY)")
		}
		if c.VectorStore.Zilliz.Collection == "" {
			return fmt.Errorf("zilliz.collection is required")
		}
	}

	if c.VectorStore.Backend == "qdrant" {
		if c.VectorStore.Qdrant.Endpoint == "" {
			return fmt.Errorf("qdrant.endpoint is required")
		}
		if c.VectorStore.Qdrant.APIKey == "" {
			return fmt.Errorf("qdrant.api_key is required (set QDRANT_API_KEY)")
		}
		if c.VectorStore.Qdrant.Collection == "" {
			return fmt.Errorf("qdrant.collection is required")
		}
	}

	// Validate embedding provider
	if c.Embedding.Provider != "openai" && c.Embedding.Provider != "cohere" {
		return fmt.Errorf("provider '%s' not supported (must be 'openai' or 'cohere')", c.Embedding.Provider)
	}

	// Validate embedding provider config
	if c.Embedding.Provider == "openai" {
		if c.Embedding.OpenAI.APIKey == "" {
			return fmt.Errorf("openai.api_key is required (set OPENAI_API_KEY)")
		}
		if c.Embedding.OpenAI.Model == "" {
			c.Embedding.OpenAI.Model = "text-embedding-3-small" // default
		}
	}

	if c.Embedding.Provider == "cohere" {
		if c.Embedding.Cohere.APIKey == "" {
			return fmt.Errorf("cohere.api_key is required (set COHERE_API_KEY)")
		}
		if c.Embedding.Cohere.Model == "" {
			c.Embedding.Cohere.Model = "embed-english-v3.0" // default
		}
	}

	return nil
}

// interpolateEnvVars replaces ${VAR} with environment variable values
func interpolateEnvVars(cfg *Config) {
	// VectorStore
	cfg.VectorStore.Zilliz.Endpoint = expandEnv(cfg.VectorStore.Zilliz.Endpoint)
	cfg.VectorStore.Zilliz.APIKey = expandEnv(cfg.VectorStore.Zilliz.APIKey)
	cfg.VectorStore.Zilliz.Collection = expandEnv(cfg.VectorStore.Zilliz.Collection)

	cfg.VectorStore.Qdrant.Endpoint = expandEnv(cfg.VectorStore.Qdrant.Endpoint)
	cfg.VectorStore.Qdrant.APIKey = expandEnv(cfg.VectorStore.Qdrant.APIKey)
	cfg.VectorStore.Qdrant.Collection = expandEnv(cfg.VectorStore.Qdrant.Collection)

	// Embedding
	cfg.Embedding.OpenAI.APIKey = expandEnv(cfg.Embedding.OpenAI.APIKey)
	cfg.Embedding.OpenAI.Model = expandEnv(cfg.Embedding.OpenAI.Model)

	cfg.Embedding.Cohere.APIKey = expandEnv(cfg.Embedding.Cohere.APIKey)
	cfg.Embedding.Cohere.Model = expandEnv(cfg.Embedding.Cohere.Model)

	// ExternalStore
	cfg.ExternalStore.S3.Bucket = expandEnv(cfg.ExternalStore.S3.Bucket)
	cfg.ExternalStore.S3.Region = expandEnv(cfg.ExternalStore.S3.Region)
	cfg.ExternalStore.Local.Path = expandEnv(cfg.ExternalStore.Local.Path)
}

// expandEnv replaces ${VAR} or $VAR with environment variable value
func expandEnv(s string) string {
	return os.Expand(s, func(key string) string {
		// Handle ${VAR} and $VAR
		return os.Getenv(strings.TrimSpace(key))
	})
}
