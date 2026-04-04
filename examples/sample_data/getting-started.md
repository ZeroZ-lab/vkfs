# Getting Started Guide

## Installation

VKFS can be installed from source or downloaded as a pre-built binary.

### From Source

```bash
git clone https://github.com/ZeroZ-lab/vkfs.git
cd vkfs
make build
```

### Using Go Install

```bash
go install github.com/ZeroZ-lab/vkfs/cmd/vkfs@latest
go install github.com/ZeroZ-lab/vkfs/cmd/vkfs-admin@latest
```

## Configuration

Create a configuration file at `~/.vkfs/config.yaml`. VKFS supports multiple
vector store backends and embedding providers.

### Local Development (SQLite)

The simplest setup uses SQLite with no external services:

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

### Production (Zilliz Cloud)

For production workloads with automatic scaling:

```yaml
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: "https://in03-xxx.serverless.aws-eu-central-1.cloud.zilliz.com"
    api_key: "${ZILLIZ_API_KEY}"
    collection: "vkfs_prod"
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `SILICONFLOW_API_KEY` | If using SiliconFlow | SiliconFlow API key |
| `OPENAI_API_KEY` | If using OpenAI | OpenAI API key |
| `COHERE_API_KEY` | If using Cohere | Cohere API key |
| `ZILLIZ_API_KEY` | If using Zilliz | Zilliz Cloud API key |
| `VKFS_CONFIG` | No | Custom config file path |

## First Steps

1. Initialize the knowledge base:

```bash
vkfs-admin init
```

2. Ingest your first documents:

```bash
vkfs ingest ./my-docs /docs
```

3. Explore the virtual filesystem:

```bash
vkfs ls /docs
vkfs cat /docs/getting-started.md
vkfs search "how to install" /docs
```

## Next Steps

- Read the [Architecture](architecture.md) document to understand the design
- Check the [API Reference](api_reference.md) for all available commands
- Try the [examples](../README.md) for different backends
