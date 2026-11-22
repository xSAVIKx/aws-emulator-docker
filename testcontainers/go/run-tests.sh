#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Run the tests of the Go Testcontainers module for the AWS Emulator.
# AWS_EMULATOR_IMAGE selects the image (default: ghcr.io/xsavikx/aws-emulator:latest).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

export AWS_EMULATOR_IMAGE="${AWS_EMULATOR_IMAGE:-ghcr.io/xsavikx/aws-emulator:latest}"
echo "image: $AWS_EMULATOR_IMAGE"

gofmt_out=$("$(go env GOROOT)/bin/gofmt" -l .)
if [ -n "$gofmt_out" ]; then
  echo "gofmt: these files need formatting:" >&2
  echo "$gofmt_out" >&2
  exit 1
fi
go vet ./...
go test -v -count=1 -timeout 15m ./...
