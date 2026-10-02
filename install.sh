#!/bin/bash
set -e

# DHI Harness — dependency installer for macOS and Linux.
# Installs: ripgrep, jq, curl, ddgr, s-search, agent-browser, Ollama + default model.
# Usage: ./install.sh [--no-model]

PULL_MODEL=true
if [ "${1:-}" = "--no-model" ]; then
  PULL_MODEL=false
fi

OS="$(uname -s)"
echo "=== DHI Harness - Tool Installation ($OS) ==="
echo ""

have() { command -v "$1" &>/dev/null; }
have_brew() { command -v brew &>/dev/null; }

check_go() {
  if ! have go; then
    echo "Error: Go toolchain not found. Install Go 1.26+ then re-run." >&2
    echo "  macOS: brew install go" >&2
    echo "  Linux: https://go.dev/dl/ (or via package manager)" >&2
    exit 1
  fi
  GO_VER="$(go version | awk '{print $3}' | sed 's/^go//')"
  GO_MAJOR="$(echo "$GO_VER" | cut -d. -f1)"
  GO_MINOR="$(echo "$GO_VER" | cut -d. -f2)"
  if [ "$GO_MAJOR" -lt 1 ] || { [ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 26 ]; }; then
    echo "Error: Go $GO_VER found, but Go 1.26+ is required." >&2
    exit 1
  fi
  echo "  ok: go $GO_VER"
}
check_go

install_macos() {
  if ! have_brew; then
    echo "Error: Homebrew not found. Install from https://brew.sh then re-run." >&2
    exit 1
  fi
  brew install ripgrep jq curl ddgr 2>/dev/null || true
  brew install agent-browser 2>/dev/null || brew install --cask agent-browser 2>/dev/null || true
  brew install s-search 2>/dev/null || true
  if ! have ollama; then
    brew install ollama
  fi
}

install_linux_pkg() {
  # $1 = Debian/Ubuntu name, $2 = Fedora name, $3 = Arch name
  if have apt-get; then
    sudo apt-get update -qq && sudo apt-get install -y "$1" 2>/dev/null || true
  elif have dnf; then
    sudo dnf install -y "$2" 2>/dev/null || true
  elif have pacman; then
    sudo pacman -Sy --noconfirm "$3" 2>/dev/null || true
  elif have apk; then
    sudo apk add "$1" 2>/dev/null || true
  elif have_brew; then
    brew install "$1" 2>/dev/null || true
  else
    echo "Warning: no supported package manager found for $1; install it manually." >&2
  fi
}

install_linux() {
  install_linux_pkg ripgrep ripgrep ripgrep
  install_linux_pkg jq jq jq
  install_linux_pkg curl curl curl
  install_linux_pkg ddgr ddgr ddgr
  install_linux_pkg xdg-utils xdg-utils xdg-utils

  if have_brew; then
    brew install agent-browser 2>/dev/null || true
    brew install s-search 2>/dev/null || true
  elif have npm; then
    npm i -g agent-browser 2>/dev/null || true
  else
    echo "Warning: install Homebrew (Linuxbrew) or npm to get agent-browser + s-search." >&2
  fi

  if ! have ollama; then
    curl -fsSL https://ollama.com/install.sh | sh
  fi
}

case "$OS" in
  Darwin) install_macos ;;
  Linux) install_linux ;;
  *) echo "Unsupported OS: $OS (macOS and Linux only)." >&2; exit 1 ;;
esac

echo ""
echo "[1/3] Core tools check..."
for bin in rg jq curl ddgr ollama; do
  if have "$bin"; then
    echo "  ok: $bin"
  else
    echo "  missing: $bin (see README Troubleshooting)" >&2
  fi
done
if have agent-browser; then
  echo "  ok: agent-browser"
  agent-browser install 2>/dev/null || true
else
  echo "  missing: agent-browser (optional, needed for web_search/fetch_url)" >&2
fi
if have s; then
  echo "  ok: s-search"
else
  echo "  missing: s-search (optional, needed for site_search pipeline)" >&2
fi

echo ""
echo "[2/3] Ollama model..."
if [ "$PULL_MODEL" = true ]; then
  if ! pgrep -x ollama >/dev/null 2>&1; then
    (ollama serve >/dev/null 2>&1 &) || true
    sleep 3
  fi
  ollama pull qwen2.5:1.5b
else
  echo "  skipped (--no-model)"
fi

echo ""
echo "[3/3] Building dhi..."
go build -o build/dhi ./cmd/dhi
echo "  binary: ./build/dhi"
echo "  runtime dir ~/.dhi/ is auto-created on first run (config.yaml, pipelines.yaml, scripts/)."

echo ""
echo "=== Installation Complete ==="
echo "Run './build/dhi' or 'go run ./cmd/dhi' to start DHI Harness."
echo "Optional API keys for non-Ollama providers: export GROQ_API_KEY=..., OPENROUTER_API_KEY=...."
