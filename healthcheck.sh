#!/bin/sh
# Health check for the AWS Emulator container.
# The emulator serves HTTPS with a self-signed certificate that it creates at
# start, so this check skips certificate verification. It only calls the
# local health route.
# Set EMULATOR_HEALTHCHECK_PORT when you start the emulator with "-p <port>".
# With the TLS front end, the check also waits until the front end is ready.
set -eu
port="${EMULATOR_HEALTHCHECK_PORT:-9297}"
if [ -n "${EMULATOR_TLS_NAMES:-}${EMULATOR_TLS_CA_CERT:-}${EMULATOR_HTTP_PORT:-}" ] &&
  [ ! -s "${HOME}/.aws/emulator/${port}/tls-ca.pem" ]; then
  echo "unhealthy: the TLS front end is not ready" >&2
  exit 1
fi
exec curl -fsSk --max-time 4 "https://127.0.0.1:${port}/__admin/health" -o /dev/null
