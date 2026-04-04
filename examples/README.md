# VKFS Examples

## sqlite_demo

End-to-end demo using local SQLite backend. Zero external dependencies.

```bash
go run examples/sqlite_demo/main.go
```

What it does:
1. Creates a temporary SQLite database
2. Initializes an empty PathTree
3. Ingests `examples/sample_data/*.md` into `/docs`
4. Runs ls, stat, cat, find, grep, search
5. Verifies data persists across DB reopen

## milvus_demo

Demo using Milvus Cloud (gRPC) + SiliconFlow embeddings. For Zilliz Cloud Dedicated or other cloud Milvus with gRPC support.

```bash
# Set env vars
export MILVUS_ENDPOINT="in03-xxxxx.aws-us-east-1.cloud.zilliz.com:19530"
export MILVUS_API_KEY="your-api-key"
export SILICONFLOW_API_KEY="your-siliconflow-key"

# Run demo
go run examples/milvus_demo/main.go

# Optional: override collection name
MILVUS_COLLECTION="my_collection" go run examples/milvus_demo/main.go
```

Note: Zilliz Cloud **Serverless** instances do not expose gRPC (port 19530). Use the `zilliz_cloud_demo` for Serverless, or use a **Dedicated** plan for gRPC access.

## zilliz_cloud_demo

Demo using Zilliz Cloud Serverless + SiliconFlow embeddings. Requires API keys.

```bash
# Set env vars (or use ${VAR} in config.yaml)
export ZILLIZ_API_KEY="your-zilliz-key"
export SILICONFLOW_API_KEY="your-siliconflow-key"

# Init first (creates collection + PathTree)
go run ./cmd/vkfs-admin init

# Run demo
go run examples/zilliz_cloud_demo/main.go

# Or use the CLI directly
go run ./cmd/vkfs ingest examples/sample_data /docs
go run ./cmd/vkfs ls /docs
go run ./cmd/vkfs cat /docs/readme.md
go run ./cmd/vkfs search "architecture" /docs
```

## Config examples

See `config_example.yaml` for a complete config file with all backends.
