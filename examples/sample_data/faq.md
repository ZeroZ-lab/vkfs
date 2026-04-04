# FAQ

## General

**Q: What is VKFS?**
A: VKFS (Virtual Knowledge File System) provides Unix-like filesystem commands
over vector databases. It lets AI agents navigate and search knowledge bases
using familiar commands like ls, cat, grep, and search.

**Q: Who is VKFS for?**
A: VKFS is designed for developers building AI agents and RAG (Retrieval
Augmented Generation) systems that need to navigate large document collections.

**Q: Is VKFS a real filesystem?**
A: No. VKFS is a virtual filesystem that maps vector database content to
filesystem-like paths. Files are stored as chunks with embeddings, not as
traditional files on disk.

## Backends

**Q: Which vector database should I use?**
A: Use SQLite for local development and testing. Use Zilliz Cloud Serverless
for production with minimal ops. Use Milvus Cloud Dedicated for
latency-sensitive production workloads.

**Q: Can I use VKFS offline?**
A: Yes, with the SQLite backend and a local embedding provider. You can also
pre-compute embeddings and use VKFS in read-only mode.

**Q: Does VKFS support Qdrant?**
A: Qdrant support is planned but not yet implemented. Contributions welcome.

## Embeddings

**Q: Which embedding model should I use?**
A: For Chinese content, use SiliconFlow BAAI/bge-m3 (1024 dim). For English
content, use OpenAI text-embedding-3-small (1536 dim). For maximum quality,
use OpenAI text-embedding-3-large (3072 dim).

**Q: Can I use a custom embedding provider?**
A: Yes. Implement the `EmbeddingProvider` interface with `Embed`,
`EmbedBatch`, and `Dimension` methods. Register it in the factory.

**Q: What happens if embedding dimensions don't match?**
A: VKFS auto-detects the dimension from the embedding provider and passes it
to the vector store. If you change models, you need to re-ingest your data.

## Performance

**Q: How fast is ls/find/stat?**
A: Zero network latency. The PathTree is loaded into memory at startup.
Operations are pure map lookups.

**Q: How fast is search?**
A: Depends on the backend. SQLite uses brute-force L2 distance (O(n)).
Zilliz uses indexed HNSW search (sub-100ms for millions of vectors).

**Q: How large of a dataset can VKFS handle?**
A: SQLite handles up to ~100K chunks comfortably. Zilliz Cloud handles
millions of chunks with indexed search.

## Troubleshooting

**Q: "PathTree not found" error**
A: Run `vkfs-admin init` to create the initial PathTree.

**Q: "unsupported vectorstore backend" error**
A: Check that `backend` in config.yaml is set to "sqlite", "zilliz", "milvus",
or "qdrant".

**Q: "failed to connect to Zilliz" error**
A: Zilliz Cloud Serverless does not support gRPC. Use the REST adapter by
setting `backend: zilliz` (not `milvus`). Only Dedicated plans expose gRPC.
