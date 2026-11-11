#!/bin/sh
# Health check for the AWS Emulator container.
# The emulator serves HTTPS with a self-signed certificate that it creates at
# start, so this check skips certificate verification. It only calls the
# local health route.
# Set EMULATOR_HEALTHCHECK_PORT when you start the emulator with "-p <port>".
set -eu
port="${EMULATOR_HEALTHCHECK_PORT:-9297}"
exec curl -fsSk --max-time 4 "https://127.0.0.1:${port}/__admin/health" -o /dev/null
