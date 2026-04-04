package vectorstore

import (
	"fmt"

	"github.com/zhengjianqiao/vkfs/internal/config"
	"github.com/zhengjianqiao/vkfs/pkg/vfs"
)

// NewFromConfig creates a VectorStore based on the configuration.
// The dimension parameter is typically obtained from the embedding provider.
func NewFromConfig(cfg *config.Config, dimension int) (vfs.VectorStore, error) {
	switch cfg.VectorStore.Backend {
	case "zilliz":
		adapter, err := NewZillizRESTAdapter(ZillizRESTConfig{
			Endpoint:   cfg.VectorStore.Zilliz.Endpoint,
			APIKey:     cfg.VectorStore.Zilliz.APIKey,
			Collection: cfg.VectorStore.Zilliz.Collection,
			Dimension:  dimension,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Zilliz REST adapter: %w", err)
		}
		return adapter, nil
	case "milvus":
		adapter, err := NewZillizAdapter(ZillizConfig{
			Endpoint:   cfg.VectorStore.Milvus.Endpoint,
			APIKey:     cfg.VectorStore.Milvus.APIKey,
			Collection: cfg.VectorStore.Milvus.Collection,
			Dimension:  dimension,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Milvus adapter: %w", err)
		}
		return adapter, nil
	case "qdrant":
		return nil, fmt.Errorf("Qdrant adapter not implemented yet")
	case "sqlite":
		adapter, err := NewSQLiteAdapter(SQLiteConfig{
			Path:      cfg.VectorStore.SQLite.Path,
			Dimension: dimension,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create SQLite adapter: %w", err)
		}
		return adapter, nil
	default:
		return nil, fmt.Errorf("unsupported vectorstore backend: %s", cfg.VectorStore.Backend)
	}
}
