# Deployment Guide

## Docker Deployment

### Build Image

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN make build

FROM alpine:3.19
COPY --from=builder /app/bin/vkfs /usr/local/bin/
COPY --from=builder /app/bin/vkfs-admin /usr/local/bin/
ENTRYPOINT ["vkfs"]
```

### Run

```bash
docker build -t vkfs .
docker run -v ~/.vkfs:/root/.vkfs vkfs ls /
```

## Kubernetes Deployment

For production workloads, deploy VKFS as a sidecar or API server alongside
your AI agent service.

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vkfs-config
data:
  config.yaml: |
    vectorstore:
      backend: zilliz
      zilliz:
        endpoint: "https://in03-xxx.serverless.aws-eu-central-1.cloud.zilliz.com"
        collection: "vkfs_prod"
    embedding:
      provider: openai
      openai:
        model: "text-embedding-3-small"
```

### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vkfs-secrets
type: Opaque
stringData:
  ZILLIZ_API_KEY: "your-zilliz-key"
  OPENAI_API_KEY: "your-openai-key"
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vkfs
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vkfs
  template:
    metadata:
      labels:
        app: vkfs
    spec:
      containers:
      - name: vkfs
        image: vkfs:latest
        command: ["vkfs"]
        args: ["search", "deployment guide", "/docs"]
        envFrom:
        - secretRef:
            name: vkfs-secrets
        volumeMounts:
        - name: config
          mountPath: /root/.vkfs
      volumes:
      - name: config
        configMap:
          name: vkfs-config
```

## Performance Tuning

### SQLite

- Increase WAL checkpoint interval for write-heavy workloads
- Use SSD storage for the database file
- Set `PRAGMA synchronous=NORMAL` for better write throughput

### Zilliz Cloud

- Use Dedicated plan for latency-sensitive workloads (< 50ms p99)
- Set appropriate `limit` on search queries (default top-K = 10)
- Use path prefix filters to narrow search scope

### Embedding

- Use batch embedding (`EmbedBatch`) for ingest operations
- SiliconFlow BAAI/bge-m3 is cost-effective for Chinese + English content
- OpenAI text-embedding-3-large for maximum retrieval quality

## Monitoring

Key metrics to monitor:
- PathTree load time (should be < 1s)
- Embedding latency (provider-dependent, typically 50-200ms)
- Vector search latency (SQLite: O(n), Zilliz: < 100ms indexed)
- Chunk retrieval latency for `cat` operations
