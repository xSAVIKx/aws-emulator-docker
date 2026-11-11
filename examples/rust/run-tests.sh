#!/usr/bin/env bash
# Run the Rust example tests against a running AWS Emulator.
# The emulator environment must be set; see examples/README.md.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
cargo test --locked -- --nocapture
