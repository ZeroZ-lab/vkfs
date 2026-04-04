# VKFS Project

Welcome to the VKFS project. This is a virtual knowledge file system
that provides Unix-like filesystem commands over vector databases.

## Features

- Unix-like commands: ls, cat, grep, find
- Semantic search powered by vector embeddings
- Pluggable vector store backends (Zilliz, SQLite)
- Pluggable embedding providers (OpenAI, Cohere, SiliconFlow)

## Getting Started

Install VKFS and run `vkfs-admin init` to set up your knowledge base.
Then use `vkfs ingest <dir> <path>` to import files.
