#!/usr/bin/env bash
# Run the Python example against a running AWS Emulator.
# The emulator environment (AWS_ENDPOINT_URL and others) must be set already.
# From the repository root: examples/with-emulator.sh examples/python/run-tests.sh
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# --frozen: use uv.lock exactly as committed; never re-resolve versions.
exec uv run --frozen pytest -v "$@"
