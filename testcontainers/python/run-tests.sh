#!/usr/bin/env bash
# Run the tests of the Python Testcontainers module.
# AWS_EMULATOR_IMAGE selects the image (default ghcr.io/xsavikx/aws-emulator:latest).
# Usage: testcontainers/python/run-tests.sh [pytest arguments]
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

export AWS_EMULATOR_IMAGE="${AWS_EMULATOR_IMAGE:-ghcr.io/xsavikx/aws-emulator:latest}"

# --locked: use uv.lock as committed. Stop if uv.lock does not match pyproject.toml.
exec uv run --locked pytest -v "$@"
