#!/usr/bin/env bash
# Run the Java example tests. Start the emulator first, or use:
#   examples/with-emulator.sh examples/java/run-tests.sh
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
mvn -B -ntp test
