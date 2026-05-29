#!/bin/bash
set -e

echo "=== DHI Harness - Tool Installation ==="
echo ""

echo "[1/5] Installing core tools (ripgrep, jq, w3m)..."
brew install ripgrep jq w3m 2>/dev/null || true

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
echo "Run 'cargo run' to start DHI Harness."
