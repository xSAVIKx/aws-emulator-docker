#!/usr/bin/env bash
# Run the tests of the Java Testcontainers module with the local Maven and JDK.
# AWS_EMULATOR_IMAGE selects the image (default: ghcr.io/xsavikx/aws-emulator:latest).
#
# Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
export AWS_EMULATOR_IMAGE="${AWS_EMULATOR_IMAGE:-ghcr.io/xsavikx/aws-emulator:latest}"
echo "image: $AWS_EMULATOR_IMAGE"
mvn -B -ntp test "$@"
