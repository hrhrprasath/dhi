#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
TEMPLATE="$DIR/template.html"
CONTENT_MD="$DIR/content.md"
OUTPUT="$DIR/../index.html"

mkdir -p "$DIR/.."

if ! command -v pandoc &>/dev/null; then
  echo "Error: pandoc not found. Install it with: brew install pandoc" >&2
  exit 1
fi

pandoc "$CONTENT_MD" \
  --section-divs \
  -f markdown+pipe_tables+fenced_divs+raw_html \
  -t html \
  --wrap=preserve > /tmp/dhi-content.html

awk '
  NR==FNR { content = content $0 "\n"; next }
  /CONTENT_MARKER/ { printf "%s", content; next }
  { print }
' /tmp/dhi-content.html "$TEMPLATE" > "$OUTPUT"

echo "→ Built $OUTPUT"
