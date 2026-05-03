#!/usr/bin/env sh
set -eu

BASE_URL="${RINHA_DATASET_BASE_URL:-https://raw.githubusercontent.com/zanfranceschi/rinha-de-backend-2026/main/resources}"
DATA_DIR="${DATA_DIR:-data}"

mkdir -p "$DATA_DIR"

download() {
  file="$1"
  url="$BASE_URL/$file"
  dest="$DATA_DIR/$file"
  if [ "${FORCE_DOWNLOAD:-0}" != "1" ] && [ -s "$dest" ]; then
    echo "skip $dest (already exists)"
    return 0
  fi
  echo "download $url -> $dest"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$dest"
  else
    echo "curl or wget is required" >&2
    return 1
  fi
}

download references.json.gz
download mcc_risk.json
download normalization.json

echo "datasets available in $DATA_DIR"
