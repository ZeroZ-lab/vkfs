# VKFS - Virtual Knowledge File System

Unix-like filesystem interface to vector databases (Zilliz/Qdrant) for AI agents.

## Quick Start

### 1. Install

```bash
make build
# Binaries: bin/vkfs, bin/vkfs-admin
```

### 2. Configure

Create `~/.vkfs/config.yaml`:

```yaml
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: "https://in03-xxx.api.gcp-us-west1.zillizcloud.com"
    api_key: "${ZILLIZ_API_KEY}"
    collection: "vkfs_chunks"

embedding:
  provider: openai
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"
```

Set environment variables:
```bash
export ZILLIZ_ENDPOINT="https://..."
export ZILLIZ_API_KEY="your-key"
export OPENAI_API_KEY="your-key"
```

### 3. Initialize

```bash
bin/vkfs-admin init
```

### 4. Use

```bash
bin/vkfs ls /
bin/vkfs cat /path/file.md
bin/vkfs grep "keyword" /path
bin/vkfs search "semantic query" /path
bin/vkfs find / -name "*.md"
bin/vkfs stat /path
```

## Architecture

- **Vector DB as single source of truth**: File tree, chunks, metadata all in Zilliz/Qdrant
- **In-memory PathTree**: Loaded at startup, zero-latency ls/find/stat
- **Two-stage grep**: BM25 coarse filter (vector DB) → regex fine filter (in-memory)
- **Semantic search**: Query embedding → vector search → top-K results
- **Lazy pointers**: Large files (>1MB) stored in S3, referenced in vector DB

## Commands

| Command | Description | Network Calls |
|---------|-------------|---------------|
| `vkfs ls /path` | List directory | 0 (in-memory) |
| `vkfs stat /path` | File info | 0 (in-memory) |
| `vkfs find / -name "*.md"` | Find files | 0 (in-memory) |
| `vkfs cat /path/file.md` | Display file | 1 (chunks) or S3 |
| `vkfs grep "term" /path` | Text search | 1 (BM25) |
| `vkfs search "query" /path` | Semantic search | 2 (embed + vector) |

## Development

```bash
# Build
make build

# Run tests
make test

# Unit tests only
make test-unit

# Integration tests (requires Zilliz + OpenAI)
make test-integration
```

## Week 1 Status (Completed)

- ✅ Core data model (VirtualNode, PathTree, Chunk, LazyPointer, SearchHit)
- ✅ VectorStore interface
- ✅ ZillizAdapter (UpsertPathTree, GetPathTree, GetChunksByPage)
- ✅ Config loading (YAML + env vars)
- ✅ VirtualFS core (in-memory PathTree, Ls, Stat, Find)
- ✅ CLI commands (ls, stat, find)
- ✅ vkfs-admin init

## Week 2 Status (Completed)

- ✅ Cat command (lazy pointer + chunk assembly + integrity validation)
- ✅ Grep command (BM25 + regex two-stage filtering)
- ✅ Search command (embedding + vector search)
- ✅ EmbeddingProvider interface (OpenAI + Cohere)
- ✅ ZillizAdapter SearchText and SearchVector

## Week 3 Roadmap

- [ ] QdrantAdapter (second vector DB backend)
- [ ] S3Store (lazy pointer external storage)
- [ ] UpsertChunks (ingest functionality)
- [ ] Integration tests with real Zilliz + OpenAI
- [ ] Shared test suite for both adapters

## Design Decisions

Following Mintlify ChromaFS architecture:
- Vector database is the only persistent layer
- File tree loaded into memory at startup (one network call)
- ls/find/stat are zero-latency in-memory operations
- cat assembles chunks from vector DB or fetches from S3
- grep uses BM25 coarse filter (top-50) then regex fine filter

## License

MIT
