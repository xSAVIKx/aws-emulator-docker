#!/usr/bin/env bash
# Run a command with a fresh AWS Emulator, then stop the emulator.
#
# Usage: examples/with-emulator.sh <command> [args...]
# Example: EMULATOR_PORT=19701 examples/with-emulator.sh examples/python/run-tests.sh
#
# See .github/actions/aws-emulator/start-emulator.sh for the settings.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
start="$here/../.github/actions/aws-emulator/start-emulator.sh"

eval "$("$start")"
trap '"$start" --stop' EXIT
"$@"
