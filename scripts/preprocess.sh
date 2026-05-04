#!/usr/bin/env sh
set -eu
ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
REFERENCES_PATH="${REFERENCES_PATH:-data/references.json.gz}"
INDEX_PATH="${INDEX_PATH:-data/index.heindall.ivf8192.bin}"

cd "$ROOT_DIR/apps/api"
go run ./cmd/preprocess -references "$ROOT_DIR/$REFERENCES_PATH" -out "$ROOT_DIR/$INDEX_PATH" -clusters 8192 -nprobe 8 -ambiguous-nprobe 32 -repair=true
