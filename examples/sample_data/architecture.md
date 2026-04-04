# Architecture

VKFS uses a layered architecture:

1. **VFS Layer** - PathTree in memory for zero-latency ls/find/stat
2. **Vector Store** - Stores chunks with embeddings for semantic search
3. **Embedding Provider** - Converts text to vectors

## Data Flow

User command → VFS → VectorStore → Embedding Provider

Files are split into chunks, embedded, and stored in the vector database.
The PathTree is also persisted in the vector DB as a special document.

## Storage Backends

- **Zilliz Cloud** - Production-grade, cloud-hosted
- **SQLite** - Local development, single-file database
- **Qdrant** - Coming soon
