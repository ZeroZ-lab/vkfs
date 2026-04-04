# Configuration Reference

## Vector Store Backends

### SQLite

Local file-based storage. Best for development and single-machine deployments.

```yaml
vectorstore:
  backend: sqlite
  sqlite:
    path: "~/.vkfs/vkfs.db"  # optional, defaults to ~/.vkfs/vkfs.db
```

**Characteristics:**
- Zero external dependencies
- Vectors stored as BLOBs, L2 distance computed in Go
- WAL mode for concurrent read/write
- Suitable for datasets up to ~100K chunks

### Zilliz Cloud (Serverless)

Managed Milvus with REST API. Best for production serverless workloads.

```yaml
vectorstore:
  backend: zilliz
  zilliz:
    endpoint: "https://in03-xxx.serverless.aws-eu-central-1.cloud.zilliz.com"
    api_key: "${ZILLIZ_API_KEY}"
    collection: "vkfs_prod"
```

**Characteristics:**
- REST API v2 (gRPC not available on Serverless)
- Automatic scaling
- Dynamic fields for metadata

### Milvus Cloud (Dedicated)

Milvus with gRPC interface. Best for low-latency production workloads.

```yaml
vectorstore:
  backend: milvus
  milvus:
    endpoint: "in03-xxx.aws-us-east-1.cloud.zilliz.com:19530"
    api_key: "${MILVUS_API_KEY}"     # optional for self-hosted
    collection: "vkfs_prod"
```

**Characteristics:**
- gRPC protocol (port 19530)
- Lower latency than REST
- Full Milvus SDK feature set

## Embedding Providers

### SiliconFlow

Cost-effective embeddings with multilingual support (Chinese/English).

```yaml
embedding:
  provider: siliconflow
  siliconflow:
    api_key: "${SILICONFLOW_API_KEY}"
    model: "BAAI/bge-m3"  # 1024 dimensions, multilingual
```

### OpenAI

High quality English-focused embeddings.

```yaml
embedding:
  provider: openai
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"  # 1536 dimensions
    # model: "text-embedding-3-large"  # 3072 dimensions
    base_url: ""  # optional, for OpenAI-compatible APIs
```

### Cohere

English-focused embeddings with search-optimized models.

```yaml
embedding:
  provider: cohere
  cohere:
    api_key: "${COHERE_API_KEY}"
    model: "embed-english-v3.0"  # 1024 dimensions
```

## Model Dimensions

| Provider | Model | Dimensions |
|----------|-------|------------|
| SiliconFlow | BAAI/bge-m3 | 1024 |
| OpenAI | text-embedding-3-small | 1536 |
| OpenAI | text-embedding-3-large | 3072 |
| Cohere | embed-english-v3.0 | 1024 |
| Cohere | embed-multilingual-v3.0 | 1024 |

## Environment Variable Interpolation

Config values support `${VAR}` and `$VAR` syntax for environment variables:

```yaml
api_key: "${MY_API_KEY}"
endpoint: "https://${MILVUS_HOST}:${MILVUS_PORT}"
```

## Config File Discovery

VKFS looks for config in this order:
1. `VKFS_CONFIG` environment variable (exact path)
2. `~/.vkfs/config.yaml` (default location)
