#!/usr/bin/env bash
# Run the Go example tests against a running AWS Emulator.
# Start the emulator first, or run: examples/with-emulator.sh examples/go/run-tests.sh
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
go vet ./...
go test -v -count=1 ./...
