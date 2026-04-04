#!/bin/bash
# VKFS + Claude Agent Demo
#
# Demonstrates Claude CLI using VKFS as a Unix-like tool.
# Claude calls `vkfs ls`, `vkfs cat`, `vkfs grep`, `vkfs search` to answer questions.
#
# Usage:
#   bash examples/claude_agent_demo/run.sh "How do I deploy VKFS?"
#   bash examples/claude_agent_demo/run.sh              # default question
#
# Prerequisites: Claude CLI + SILICONFLOW_API_KEY

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEMO_DIR="$PROJECT_DIR/.demo_agent"

QUESTION="${1:-How do I deploy VKFS to production?}"

echo "=== VKFS + Claude Agent Demo ==="
echo ""

# --- Setup ---
echo "[1/3] Building..."
mkdir -p "$DEMO_DIR"
cd "$PROJECT_DIR"

# Build binaries
go build -o "$DEMO_DIR/vkfs_bin" ./cmd/vkfs 2>/dev/null
go build -o "$DEMO_DIR/vkfs-admin" ./cmd/vkfs-admin 2>/dev/null

# Config
cat > "$DEMO_DIR/config.yaml" <<YAML
vectorstore:
  backend: sqlite
  sqlite:
    path: "$DEMO_DIR/vkfs.db"

embedding:
  provider: siliconflow
  siliconflow:
    api_key: "${SILICONFLOW_API_KEY}"
    model: "BAAI/bge-m3"
YAML

# Remove old DB
rm -f "$DEMO_DIR/vkfs.db"

# Init + Ingest
VKFS_CONFIG="$DEMO_DIR/config.yaml" "$DEMO_DIR/vkfs-admin" init 2>&1 | sed 's/^/  /'
VKFS_CONFIG="$DEMO_DIR/config.yaml" "$DEMO_DIR/vkfs" ingest "$PROJECT_DIR/examples/sample_data" /docs 2>&1 | sed 's/^/  /'
echo ""

echo "[2/3] Knowledge base ready:"
VKFS_CONFIG="$DEMO_DIR/config.yaml" "$DEMO_DIR/vkfs" ls /docs 2>&1 | sed 's/^/  /'
echo ""

# --- Create wrapper that Claude can call directly ---
cat > "$DEMO_DIR/vkfs" <<'WRAPPER'
#!/bin/bash
# VKFS wrapper - runs vkfs with demo config
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export VKFS_CONFIG="$SCRIPT_DIR/config.yaml"
exec "$SCRIPT_DIR/vkfs_bin" "$@"
WRAPPER

# Keep the compiled binary as vkfs_bin and expose a thin wrapper as vkfs
chmod +x "$DEMO_DIR/vkfs_bin" "$DEMO_DIR/vkfs"

echo "[3/3] Launching Claude..."
echo "  Question: $QUESTION"
echo "  ──────────────────────────────────────────────"
echo ""

cd "$DEMO_DIR"

claude --print \
  --append-system-prompt "$(cat "$SCRIPT_DIR/system_prompt.md")" \
  "$QUESTION" 2>/dev/null || {
    echo ""
    echo "  Claude CLI not available. Install: https://docs.anthropic.com/en/docs/claude-cli"
    echo ""
    echo "  Try manually:"
    echo "    ./vkfs ls /docs"
    echo "    ./vkfs cat /docs/faq.md"
    echo "    ./vkfs search 'deployment' /docs"
}

echo ""
echo "  ──────────────────────────────────────────────"
echo ""
echo "Cleanup: rm -rf $DEMO_DIR"
echo ""
echo "More questions:"
echo "  bash examples/claude_agent_demo/run.sh \"What embedding models are supported?\""
echo "  bash examples/claude_agent_demo/run.sh \"如何配置中文环境？\""
