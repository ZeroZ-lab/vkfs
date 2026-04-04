<div align="center">

# VKFS

**Virtual Knowledge File System**

Unix-like filesystem commands over vector databases, built for AI agents.

[![Go Reference](https://pkg.go.dev/badge/github.com/ZeroZ-lab/vkfs.svg)](https://pkg.go.dev/github.com/ZeroZ-lab/vkfs)
[![Go Report Card](https://goreportcard.com/badge/github.com/ZeroZ-lab/vkfs)](https://goreportcard.com/report/github.com/ZeroZ-lab/vkfs)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

</div>

---

VKFS gives AI agents a filesystem interface to vector databases. Instead of dealing with embeddings, vector queries, and metadata filters directly, agents use familiar commands: `ls`, `cat`, `grep`, `find`, `search`.

```bash
vkfs ingest ./docs /docs          # import files
vkfs ls /docs                     # list contents
vkfs cat /docs/readme.md          # read file
vkfs search "deployment guide" /  # semantic search
```

## Why VKFS

AI agents need to navigate and search large knowledge bases. Current options require agents to understand vector DB APIs, embedding models, and chunk retrieval logic. VKFS abstracts all of that behind a Unix-like filesystem:

- **Zero-learning-curve interface** — `ls`, `cat`, `grep` work exactly as expected
- **Semantic search built in** — `search "query" /path` handles embedding + vector similarity
- **Pluggable backends** — SQLite for local dev, Zilliz/Milvus for production
- **Pluggable embeddings** — OpenAI, Cohere, SiliconFlow, or any OpenAI-compatible API
- **Single binary** — No runtime dependencies, pure Go

## Architecture

```
┌──────────────────────────────────────────────────┐
│                    CLI / API                      │
│          ls | cat | grep | find | search          │
├──────────────────────────────────────────────────┤
│                  VirtualFS                        │
│  ┌─────────────┐  ┌────────────┐  ┌───────────┐ │
│  │  PathTree    │  │  Chunker   │  │  Search   │ │
│  │ (in-memory)  │  │            │  │           │ │
│  └─────────────┘  └────────────┘  └───────────┘ │
├──────────────┬──────────────┬────────────────────┤
│ VectorStore  │   Embedding  │   ExternalStore    │
│  Interface   │   Provider   │     (S3/Local)     │
├──────────────┼──────────────┤                    │
│ SQLite       │   OpenAI     │                    │
│ Zilliz REST  │   Cohere     │                    │
│ Milvus gRPC  │ SiliconFlow  │                    │
│ Qdrant (WIP) │              │                    │
└──────────────┴──────────────┴────────────────────┘
```

**Key design decisions:**

- **Vector DB is the single source of truth** — PathTree, chunks, and metadata all live in the vector store. No separate metadata database.
- **In-memory PathTree** — Loaded once at startup with a single network call. `ls`, `find`, `stat` are zero-latency.
- **Chunk-based storage** — Files are split into paragraph-aware chunks (Markdown) or line-aware chunks (other text). Chunks are embedded and stored individually for granular retrieval.
- **Two-stage grep** — BM25/text coarse filter from the vector store, then regex fine filter in memory.
- **Lazy pointers** — Files exceeding a size threshold are stored in S3 with a pointer in the vector DB.

## Quick Start

### 1. Install

```bash
git clone https://github.com/ZeroZ-lab/vkfs.git
cd vkfs
make build
```

Binaries: `bin/vkfs`, `bin/vkfs-admin`

### 2. Configure

Create `~/.vkfs/config.yaml`. The simplest option is SQLite (no cloud services needed):

```yaml
vectorstore:
  backend: sqlite
  sqlite:
    path: "~/.vkfs/vkfs.db"

embedding:
  provider: siliconflow
  siliconflow:
    api_key: "${SILICONFLOW_API_KEY}"
    model: "BAAI/bge-m3"
```

For production, use Zilliz Cloud Serverless:

```yaml
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: "https://in03-xxx.serverless.aws-eu-central-1.cloud.zilliz.com"
    api_key: "${ZILLIZ_API_KEY}"
    collection: "vkfs_prod"

embedding:
  provider: openai
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"
```

Set environment variables:

```bash
export SILICONFLOW_API_KEY="your-key"
# or
export ZILLIZ_API_KEY="your-key"
export OPENAI_API_KEY="your-key"
```

See [examples/config_example.yaml](examples/config_example.yaml) for all backend and provider combinations.

### 3. Initialize

```bash
bin/vkfs-admin init
```

### 4. Use

```bash
# Import local files
bin/vkfs ingest ./my-docs /docs

# Navigate
bin/vkfs ls /
bin/vkfs ls /docs
bin/vkfs stat /docs/readme.md

# Read
bin/vkfs cat /docs/readme.md

# Search
bin/vkfs find / -name "*.md"
bin/vkfs grep "authentication" /docs
bin/vkfs search "how to deploy" /docs --top-k 5
```

## CLI Reference

### `vkfs` — Filesystem Commands

| Command | Usage | Network Calls | Description |
|---------|-------|---------------|-------------|
| `ls` | `vkfs ls [path]` | 0 | List directory contents (in-memory) |
| `stat` | `vkfs stat <path>` | 0 | Show file/directory metadata (in-memory) |
| `find` | `vkfs find <path> -name <pattern>` | 0 | Find files matching glob pattern (in-memory) |
| `cat` | `vkfs cat <path>` | 1 | Reassemble and display file from chunks |
| `grep` | `vkfs grep <pattern> <path>` | 1 | Text search (BM25 + regex two-stage filter) |
| `search` | `vkfs search <query> <path>` | 2 | Semantic search (embed + vector similarity) |
| `ingest` | `vkfs ingest <dir> <vkfs-path>` | N | Import local files: read, chunk, embed, store |

### `vkfs-admin` — Administration

| Command | Usage | Description |
|---------|-------|-------------|
| `init` | `vkfs-admin init` | Create empty PathTree with root `/` in vector DB |

## Supported Backends

### Vector Stores

| Backend | Type | Status | Use Case |
|---------|------|--------|----------|
| **SQLite** | Local | Stable | Development, single-machine, offline |
| **Zilliz Cloud Serverless** | Cloud (REST) | Stable | Production, serverless scale |
| **Milvus Cloud Dedicated** | Cloud (gRPC) | Beta | Production, low-latency gRPC |
| **Qdrant** | Cloud/Self-hosted | Planned | Alternative vector DB |

### Embedding Providers

| Provider | Models | Dimensions | Notes |
|----------|--------|------------|-------|
| **SiliconFlow** | `BAAI/bge-m3` | 1024 | Cost-effective, multilingual |
| **OpenAI** | `text-embedding-3-small` | 1536 | Default, high quality |
| **OpenAI** | `text-embedding-3-large` | 3072 | Maximum quality |
| **Cohere** | `embed-english-v3.0` | 1024 | English-focused |
| **OpenAI-compatible** | Any | Auto-detect | Via `base_url` config |

## Go API

VKFS is designed to be used as a Go library:

```go
package main

import (
    "context"

    "github.com/ZeroZ-lab/vkfs/internal/config"
    "github.com/ZeroZ-lab/vkfs/pkg/embedding"
    "github.com/ZeroZ-lab/vkfs/pkg/vectorstore"
    "github.com/ZeroZ-lab/vkfs/pkg/vfs"
)

func main() {
    ctx := context.Background()

    // Load config
    cfg, _ := config.LoadDefault()

    // Create providers (factory pattern)
    embedder, _ := embedding.NewFromConfig(cfg)
    store, _ := vectorstore.NewFromConfig(cfg, embedder.Dimension())

    // Create filesystem
    fs := vfs.NewVirtualFS(store, nil, embedder)
    fs.Init(ctx)

    // Use it
    nodes, _ := fs.Ls("/")
    content, _ := fs.Cat(ctx, "/docs/readme.md")
    hits, _ := fs.Search(ctx, "deployment guide", "/", 10)
}
```

### Key Interfaces

```go
// VectorStore — implemented by SQLite, Zilliz REST, Milvus gRPC
type VectorStore interface {
    UpsertPathTree(ctx context.Context, tree PathTree) error
    GetPathTree(ctx context.Context) (PathTree, error)
    UpsertChunks(ctx context.Context, chunks []Chunk) error
    GetChunksByPage(ctx context.Context, pageSlug string) ([]Chunk, error)
    DeleteChunksByPage(ctx context.Context, pageSlug string) error
    SearchText(ctx context.Context, pattern string, filter PathFilter, limit int) ([]Chunk, error)
    SearchVector(ctx context.Context, queryVec []float32, filter PathFilter, topK int) ([]SearchHit, error)
    // ...
}

// EmbeddingProvider — implemented by OpenAI, Cohere, SiliconFlow
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
}
```

## Examples

| Example | Backend | Dependencies | Run |
|---------|---------|-------------|-----|
| [SQLite Demo](examples/sqlite_demo/) | SQLite | None | `go run examples/sqlite_demo/main.go` |
| [Milvus Cloud Demo](examples/milvus_demo/) | Milvus gRPC | API keys | `MILVUS_ENDPOINT=... go run examples/milvus_demo/main.go` |
| [Zilliz Cloud Demo](examples/zilliz_cloud_demo/) | Zilliz REST | API keys | `go run examples/zilliz_cloud_demo/main.go` |

## Development

```bash
# Build
make build

# Run all tests
make test

# Run unit tests only
make test-unit

# Lint
make vet

# Install to $GOPATH/bin
make install
```

### Project Structure

```
vkfs/
├── cmd/
│   ├── vkfs/              # CLI: ls, cat, grep, find, search, ingest
│   └── vkfs-admin/        # Admin CLI: init
├── pkg/
│   ├── vfs/               # Core: VirtualFS, PathTree, Chunker, interfaces
│   ├── vectorstore/       # Backends: SQLite, Zilliz REST, Milvus gRPC
│   └── embedding/         # Providers: OpenAI, Cohere, SiliconFlow
├── internal/
│   └── config/            # YAML config loading, env var interpolation
├── examples/              # Runnable demos for each backend
├── tests/
│   ├── unit/              # Unit tests
│   └── integration/       # Integration tests (requires cloud services)
└── Makefile
```

## Roadmap

- [ ] Qdrant adapter
- [ ] Hybrid search (vector + BM25 combined ranking)
- [ ] S3 external store for lazy pointers
- [ ] File watch and auto-ingest
- [ ] gRPC server mode (for remote AI agent access)
- [ ] Multi-tenant ACL (group-based file visibility)
- [ ] Collection auto-creation with schema migration

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Commit your changes (`git commit -m 'feat: add my feature'`)
4. Push to the branch (`git push origin feat/my-feature`)
5. Open a Pull Request

## License

[MIT](LICENSE)
