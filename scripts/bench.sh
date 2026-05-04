#!/usr/bin/env sh
set -eu

if ! command -v k6 >/dev/null 2>&1; then
  echo "k6 is required to run the official challenge tests." >&2
  echo "Install k6 or run this script in an environment that provides k6." >&2
  exit 1
fi

if [ ! -f specs/test/test.js ]; then
  echo "Challenge specs submodule is missing. Run: git submodule update --init --recursive" >&2
  exit 1
fi

cd specs
k6 run test/smoke.js
k6 run test/test.js
