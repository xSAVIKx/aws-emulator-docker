#!/usr/bin/env bash
# Run the .NET example tests against a running AWS Emulator.
# Start the emulator first, or use: examples/with-emulator.sh examples/dotnet/run-tests.sh
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
export DOTNET_CLI_TELEMETRY_OPTOUT="${DOTNET_CLI_TELEMETRY_OPTOUT:-1}"
export DOTNET_NOLOGO="${DOTNET_NOLOGO:-1}"
dotnet test --logger "console;verbosity=normal"
