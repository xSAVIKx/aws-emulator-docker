#!/usr/bin/env bash
# Run the tests of the .NET Testcontainers module.
#
# The tests start the image in AWS_EMULATOR_IMAGE (default
# ghcr.io/xsavikx/aws-emulator:latest). Extra arguments go to "dotnet test".
#
# When "dotnet" is on the PATH, the script uses it. Otherwise it runs the
# official .NET SDK container (DOTNET_SDK_IMAGE) with the Docker socket.
#
# Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

export AWS_EMULATOR_IMAGE="${AWS_EMULATOR_IMAGE:-ghcr.io/xsavikx/aws-emulator:latest}"
export DOTNET_CLI_TELEMETRY_OPTOUT="${DOTNET_CLI_TELEMETRY_OPTOUT:-1}"
export DOTNET_NOLOGO="${DOTNET_NOLOGO:-1}"
SDK_IMAGE="${DOTNET_SDK_IMAGE:-mcr.microsoft.com/dotnet/sdk:11.0}"

# The Docker socket on this host (the path that the Docker daemon also sees).
case "${DOCKER_HOST:-}" in
  unix://*) SOCKET="${DOCKER_HOST#unix://}" ;;
  "") SOCKET=/var/run/docker.sock ;;
  *) SOCKET="" ;;
esac

# With rootless Docker, the default socket /var/run/docker.sock does not exist.
# Testcontainers mounts the socket into the Ryuk container, so it must know the path.
ROOTLESS=false
if docker info -f '{{.SecurityOptions}}' 2>/dev/null | grep -q rootless; then
  ROOTLESS=true
  if [ -n "$SOCKET" ]; then
    export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="${TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE:-$SOCKET}"
  fi
fi

run_tests() {
  # --locked-mode: restore the exact versions in packages.lock.json, or fail.
  dotnet restore --locked-mode
  dotnet test --no-restore --logger "console;verbosity=normal" "$@"
}

if command -v dotnet >/dev/null 2>&1; then
  run_tests "$@"
  exit 0
fi

if [ -z "$SOCKET" ]; then
  echo "dotnet is not installed, and DOCKER_HOST is not a unix socket: $DOCKER_HOST" >&2
  exit 1
fi

# The SDK container. It uses the host network, so the mapped ports are on
# 127.0.0.1. Testcontainers sees /.dockerenv and would use the gateway of the
# Docker network as the host, so TESTCONTAINERS_HOST_OVERRIDE sets 127.0.0.1.
# With rootless Docker, root in the container is the host user, so the files in
# bin/ and obj/ get the correct owner. Otherwise the container uses the host user.
if [ "$ROOTLESS" = true ]; then
  USER_ARGS=(--user 0:0)
else
  USER_ARGS=(--user "$(id -u):$(id -g)" --group-add "$(stat -c %g "$SOCKET")")
fi
ENV_ARGS=()
for name in AWS_EMULATOR_IMAGE DOTNET_CLI_TELEMETRY_OPTOUT DOTNET_NOLOGO \
  TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE TESTCONTAINERS_RYUK_DISABLED; do
  if [ -n "${!name:-}" ]; then
    ENV_ARGS+=(-e "$name=${!name}")
  fi
done

exec docker run --rm --network host \
  "${USER_ARGS[@]}" \
  -v "$PWD:$PWD" -w "$PWD" \
  -v "$SOCKET:/var/run/docker.sock" \
  -e DOCKER_HOST=unix:///var/run/docker.sock \
  -e TESTCONTAINERS_HOST_OVERRIDE="${TESTCONTAINERS_HOST_OVERRIDE:-127.0.0.1}" \
  -e HOME="$PWD/.home" \
  -e NUGET_PACKAGES="$PWD/.home/nuget-packages" \
  "${ENV_ARGS[@]}" \
  "$SDK_IMAGE" \
  bash -c "$(declare -f run_tests); run_tests \"\$@\"" run-tests "$@"
