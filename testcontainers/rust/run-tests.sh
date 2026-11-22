#!/usr/bin/env bash
# Run the tests of the Rust Testcontainers module.
#
# Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
# SPDX-License-Identifier: Apache-2.0
#
# Usage: run-tests.sh [extra cargo test arguments]
#
# AWS_EMULATOR_IMAGE selects the image (default ghcr.io/xsavikx/aws-emulator:latest).
# When cargo is on PATH, the script uses it. Otherwise it runs cargo in the
# official Rust container (RUST_IMAGE) with the Docker socket mounted.
set -euo pipefail

module_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$module_dir"
export AWS_EMULATOR_IMAGE="${AWS_EMULATOR_IMAGE:-ghcr.io/xsavikx/aws-emulator:latest}"

if command -v cargo >/dev/null 2>&1; then
  exec cargo test --locked --all-features "$@"
fi

RUST_IMAGE="${RUST_IMAGE:-rust:1.100}"
# The container keeps CARGO_HOME and the build output in this folder.
work_dir="${CARGO_CONTAINER_DIR:-$module_dir/target/container}"
mkdir -p "$work_dir"
work_dir="$(cd "$work_dir" && pwd)"

# The Docker socket of the host. Testcontainers in the container uses it.
docker_host="${DOCKER_HOST:-unix:///var/run/docker.sock}"
case "$docker_host" in
  unix://*) socket="${docker_host#unix://}" ;;
  *) echo "run-tests.sh: the container mode needs a unix DOCKER_HOST, not $docker_host" >&2; exit 2 ;;
esac

# With rootless Docker, root in the container maps to the host user.
if docker info -f '{{.SecurityOptions}}' 2>/dev/null | grep -q rootless; then
  user_spec=0:0
else
  user_spec="$(id -u):$(id -g)"
fi

# The test container uses the host network, so the mapped ports are on
# 127.0.0.1. testcontainers-rs sees /.dockerenv and would use the bridge
# gateway as the host; TESTCONTAINERS_HOST_OVERRIDE tells the module to use
# 127.0.0.1 instead.
repo_dir="$(cd "$module_dir/../.." && pwd)"
docker run --rm --network host \
  --user "$user_spec" \
  -v "$repo_dir:$repo_dir" \
  -v "$work_dir:$work_dir" \
  -v "$socket:/var/run/docker.sock" \
  -w "$module_dir" \
  -e DOCKER_HOST=unix:///var/run/docker.sock \
  -e AWS_EMULATOR_IMAGE \
  -e TESTCONTAINERS_HOST_OVERRIDE="${TESTCONTAINERS_HOST_OVERRIDE:-127.0.0.1}" \
  -e CARGO_HOME="$work_dir/cargo" \
  -e CARGO_TARGET_DIR="$work_dir/target" \
  -e HOME="$work_dir" \
  "$RUST_IMAGE" cargo test --locked --all-features "$@"
