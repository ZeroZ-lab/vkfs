# VKFS Development Plan

Generated: 2026-04-06
Status: Draft

## Current State

| Component | Status |
|-----------|--------|
| SQLite adapter | ✅ Complete |
| Zilliz REST adapter | ✅ Complete |
| Milvus gRPC adapter | ⚠️ Partial (3 methods stub) |
| Hybrid search | ⚠️ Stub (falls back to vector) |
| ExternalStore (S3) | ❌ Interface only |
| Qdrant adapter | ❌ Not started |
| VectorStore tests | ❌ Zero coverage |
| CLI tests | ❌ Zero coverage |

## Task Breakdown

### Phase 1: Test Coverage (Foundation)

#### T1. SQLite adapter unit tests
- **Scope**: `pkg/vectorstore/sqlite_test.go`
- **Acceptance**:
  - All VectorStore methods tested with in-memory DB
  - UpsertChunks → GetChunksByPage round-trip
  - SearchText with LIKE filter
  - SearchVector with L2 distance
  - SearchHybrid returns results
  - DeleteChunksByPage removes chunks
  - UpsertLazyPointer → GetLazyPointer round-trip
  - UpsertPathTree → GetPathTree round-trip
- **Verify**: `go test ./pkg/vectorstore/... -run TestSQLite -v`

#### T2. Zilliz REST adapter unit tests
- **Scope**: `pkg/vectorstore/zilliz_rest_test.go`
- **Acceptance**:
  - Mock HTTP server for all endpoints
  - Test all VectorStore methods via mocked REST responses
  - Error handling for non-200 responses
- **Verify**: `go test ./pkg/vectorstore/... -run TestZillizREST -v`

#### T3. Chunker unit tests
- **Scope**: `pkg/vfs/chunker_test.go`
- **Acceptance**:
  - Markdown splitting preserves headers
  - Code blocks kept intact
  - Large files split at boundaries
  - Empty file edge case
- **Verify**: `go test ./pkg/vfs/... -run TestChunker -v`

### Phase 2: Feature Completion

#### T4. Complete Milvus gRPC adapter
- **Scope**: `pkg/vectorstore/zilliz.go`
- **Acceptance**:
  - `DeleteChunksByPage` implemented
  - `UpsertLazyPointer` + `GetLazyPointer` implemented
  - `SearchHybrid` returns combined results
- **Verify**: `go test ./pkg/vectorstore/... -run TestZilliz -v`
- **Depends on**: T1 (test patterns established)

#### T5. Hybrid search implementation
- **Scope**: `pkg/vectorstore/sqlite.go`, `pkg/vectorstore/zilliz.go`, `pkg/vectorstore/zilliz_rest.go`
- **Acceptance**:
  - RRF (Reciprocal Rank Fusion) score combination
  - α parameter for text vs vector weight
  - Unit tests with known inputs/outputs
- **Verify**: `go test ./pkg/vectorstore/... -run TestHybrid -v`
- **Depends on**: T1, T2

### Phase 3: Quality & Documentation

#### T6. CLI command tests
- **Scope**: `cmd/vkfs/main_test.go`
- **Acceptance**:
  - Test each subcommand with mock VectorStore
  - Verify output format (ls, stat, cat, grep, search)
  - Error cases (path not found, empty query)
- **Verify**: `go test ./cmd/vkfs/... -v`
- **Depends on**: T1

#### T7. Add internal/config to CI + update docs
- **Scope**: `.github/workflows/ci.yml`, `README.md`
- **Acceptance**:
  - CI includes `internal/...` in test run
  - README updated for Ollama `dimension` config requirement
  - CHANGELOG.md created with recent fixes
- **Verify**: CI pipeline green, docs review

## Dependency Graph

```
T1 (SQLite tests) ──┬── T4 (Milvus completion)
                    ├── T5 (Hybrid search)
                    ├── T6 (CLI tests)
T2 (Zilliz tests) ──┴── T5 (Hybrid search)
T3 (Chunker tests) ──── independent
T7 (CI + docs) ──────── independent
```

## Checkpoint Criteria

- **After Phase 1**: `go test ./...` passes with >60% coverage on `pkg/vectorstore/` and `pkg/vfs/`
- **After Phase 2**: All VectorStore interface methods implemented across all adapters
- **After Phase 3**: CI green, docs current, all tests passing
