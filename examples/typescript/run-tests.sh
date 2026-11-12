#!/usr/bin/env bash
# Install the pinned dependencies, check the types, and run the tests.
# The emulator must run, and its environment must be set. See README.md.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

npm ci --no-audit --no-fund
npm run typecheck
# Extra arguments go to Vitest, for example a test file name.
npm test -- "$@"
