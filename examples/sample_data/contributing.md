# Contributing to VKFS

Thank you for your interest in contributing to VKFS!

## Development Setup

```bash
# Clone the repository
git clone https://github.com/ZeroZ-lab/vkfs.git
cd vkfs

# Build
make build

# Run tests
make test

# Run unit tests only
make test-unit
```

## Code Structure

```
pkg/vfs/          Core filesystem logic and interfaces
pkg/vectorstore/  Vector store adapters (SQLite, Zilliz, Milvus)
pkg/embedding/    Embedding providers (OpenAI, Cohere, SiliconFlow)
internal/config/  Configuration loading
cmd/vkfs/         Main CLI
cmd/vkfs-admin/   Admin CLI
```

## Adding a New Vector Store Backend

1. Create a new file in `pkg/vectorstore/`
2. Implement the `VectorStore` interface (7 methods in vfs, 10 in store.go)
3. Add the backend to `internal/config/config.go` (struct + validation)
4. Add the case to `pkg/vectorstore/factory.go`
5. Add an example in `examples/`
6. Add tests

## Adding a New Embedding Provider

1. Create a new file in `pkg/embedding/`
2. Implement the `EmbeddingProvider` interface (Embed, EmbedBatch, Dimension)
3. Add the provider to `internal/config/config.go`
4. Add the case to `pkg/embedding/factory.go`
5. Add the dimension mapping to the provider's `Dimension()` method

## Commit Messages

Follow Conventional Commits:

```
feat: add Qdrant adapter
fix: resolve chunk ordering in cat command
docs: update configuration reference
refactor: extract vector encoding helpers
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Ensure `make test` and `make vet` pass
5. Submit a pull request with a clear description

## Code Style

- Follow standard Go conventions (gofmt, golint)
- Use table-driven tests where appropriate
- Keep interfaces minimal and composable
- Document public APIs with Go doc comments
