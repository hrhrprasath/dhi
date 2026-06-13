#!/bin/bash
set -e

echo "=== DHI Harness - Tool Installation ==="
echo ""

echo "[1/5] Installing core tools (ripgrep, jq, ddgr, agent-browser)..."
brew install ripgrep jq ddgr 2>/dev/null || true
brew install agent-browser 2>/dev/null || true
agent-browser install 2>/dev/null || true

echo "[2/5] Installing s-search..."
brew install s-search 2>/dev/null || true

echo "[3/5] Installing Ollama..."
if ! command -v ollama &>/dev/null; then
    brew install ollama
fi

echo "[4/5] Starting Ollama and pulling models..."
ollama serve &>/dev/null &
sleep 3
ollama pull qwen2.5:1.5b

echo "[5/5] Setting up project..."
chmod +x scripts/*
mkdir -p .data/queue .data/intent .data/conversation

echo ""
echo "=== Installation Complete ==="
echo "Run 'go run ./cmd/dhi' to start DHI Harness."
