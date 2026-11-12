#!/usr/bin/env bash
# Run the .NET example tests against a running AWS Emulator.
# Start the emulator first, or use: examples/with-emulator.sh examples/dotnet/run-tests.sh
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
export DOTNET_CLI_TELEMETRY_OPTOUT="${DOTNET_CLI_TELEMETRY_OPTOUT:-1}"
export DOTNET_NOLOGO="${DOTNET_NOLOGO:-1}"
# --locked-mode: restore the exact versions in packages.lock.json, or fail.
dotnet restore --locked-mode
dotnet test --no-restore --logger "console;verbosity=normal" "$@"
