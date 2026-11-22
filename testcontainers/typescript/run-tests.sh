#!/usr/bin/env bash
# Install the pinned dependencies, build the library, check the types, and
# run the tests. The tests start containers, so Docker must run.
#
#   AWS_EMULATOR_IMAGE  The image to test.
#                       Default: ghcr.io/xsavikx/aws-emulator:latest
#
# Extra arguments go to Vitest, for example a test file name.
#
# Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

export AWS_EMULATOR_IMAGE="${AWS_EMULATOR_IMAGE:-ghcr.io/xsavikx/aws-emulator:latest}"

# npm ci runs the "prepare" script, which builds dist/.
npm ci --no-audit --no-fund
npm run typecheck
npm test -- "$@"
