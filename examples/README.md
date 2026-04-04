# VKFS Examples

## claude_agent_demo

**AI Agent Demo** — Claude CLI uses VKFS commands to answer questions about a knowledge base.

```bash
# Default question: "How do I deploy VKFS to production?"
bash examples/claude_agent_demo/run.sh

# Custom question
bash examples/claude_agent_demo/run.sh "What embedding models are supported?"
bash examples/claude_agent_demo/run.sh "How to configure for Chinese content?"
```

What happens:
1. Sets up isolated SQLite database + config
2. Ingests sample data into `/docs`
3. Launches Claude CLI with VKFS as available tools
4. Claude uses `ls`, `cat`, `grep`, `search` to research and answer

Prerequisites: Claude CLI installed, `SILICONFLOW_API_KEY` set.

## sqlite_demo

End-to-end demo using local SQLite backend. Zero external dependencies.

```bash
go run examples/sqlite_demo/main.go
```

## milvus_demo

Demo using Milvus Cloud (gRPC) + SiliconFlow embeddings.

```bash
export MILVUS_ENDPOINT="in03-xxxxx.aws-us-east-1.cloud.zilliz.com:19530"
export MILVUS_API_KEY="your-api-key"
export SILICONFLOW_API_KEY="your-siliconflow-key"
go run examples/milvus_demo/main.go
```

## zilliz_cloud_demo

Demo using Zilliz Cloud Serverless + SiliconFlow embeddings. Requires API keys.

```bash
export ZILLIZ_API_KEY="your-zilliz-key"
export SILICONFLOW_API_KEY="your-siliconflow-key"
go run ./cmd/vkfs-admin init
go run examples/zilliz_cloud_demo/main.go
```

## Config examples

See `config_example.yaml` for a complete config file with all backends.
