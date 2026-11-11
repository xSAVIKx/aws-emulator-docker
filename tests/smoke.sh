#!/usr/bin/env bash
# Smoke test for the aws-emulator image.
# Usage: tests/smoke.sh <image>
# It follows the documented Docker flow: a bind mount of ~/.aws/emulator,
# the host user, and a port that is the same on the host and in the container.
set -euo pipefail

IMAGE="${1:?usage: $0 <image>}"
PORT="${PORT:-19297}"
WORK="$(mktemp -d)"
H="$WORK/home"
D="$H/.aws/emulator/$PORT"
NAME="aws-emulator-smoke-$$"

cleanup() {
  docker logs "$NAME" >"$WORK/container.log" 2>&1 || true
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker rm -f "$NAME-default" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  docker logs "$NAME" 2>&1 | tail -50 >&2 || true
  exit 1
}

echo "== version"
docker run --rm "$IMAGE" --version

echo "== start with bind mount on port $PORT"
mkdir -p "$H/.aws/emulator"
docker run -d --name "$NAME" --memory=2g --memory-swap=2g \
  -p "127.0.0.1:$PORT:$PORT" \
  --user "$(id -u):$(id -g)" \
  -e HOME="$H" -e EMULATOR_TELEMETRY=off -e EMULATOR_HEALTHCHECK_PORT="$PORT" \
  -v "$H/.aws/emulator:$H/.aws/emulator" \
  "$IMAGE" -p "$PORT" >/dev/null

for _ in $(seq 60); do
  if curl -sf --cacert "$D/ca.pem" "https://127.0.0.1:$PORT/__admin/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
body=$(curl -sf --cacert "$D/ca.pem" "https://127.0.0.1:$PORT/__admin/health") || fail "health route did not answer"
[ "$body" = "ok" ] || fail "unexpected health body: $body"
echo "health: $body"

for f in emulator_credentials config ca.pem truststore.p12; do
  [ -s "$D/$f" ] || fail "missing $D/$f"
done
echo "files: OK"

echo "== STS GetCallerIdentity (SigV4 with curl)"
key=$(awk -F' *= *' '$1=="aws_access_key_id"{print $2}' "$D/emulator_credentials")
secret=$(awk -F' *= *' '$1=="aws_secret_access_key"{print $2}' "$D/emulator_credentials")
resp=$(curl -sf --cacert "$D/ca.pem" --aws-sigv4 "aws:amz:us-east-1:sts" --user "$key:$secret" \
  -d 'Action=GetCallerIdentity&Version=2011-06-15' "https://127.0.0.1:$PORT/") || fail "STS call failed"
echo "$resp" | grep -q '000000000000' || fail "unexpected STS response: $resp"
echo "sts: account 000000000000"

if command -v aws >/dev/null 2>&1; then
  echo "== AWS CLI"
  export AWS_ENDPOINT_URL="https://127.0.0.1:$PORT" \
    AWS_SHARED_CREDENTIALS_FILE="$D/emulator_credentials" AWS_CONFIG_FILE="$D/config" \
    AWS_CA_BUNDLE="$D/ca.pem" AWS_EC2_METADATA_DISABLED=true \
    AWS_DISABLE_HOST_PREFIX_INJECTION=true AWS_REGION=us-east-1 AWS_DEFAULT_REGION=us-east-1
  unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_PROFILE
  aws sts get-caller-identity
  aws s3 mb s3://smoke-test-bucket
  echo hello | aws s3 cp - s3://smoke-test-bucket/hello.txt
  [ "$(aws s3 cp s3://smoke-test-bucket/hello.txt -)" = "hello" ] || fail "S3 round trip"
  echo "s3: OK"
fi

echo "== Docker HEALTHCHECK"
for _ in $(seq 30); do
  status=$(docker inspect -f '{{.State.Health.Status}}' "$NAME")
  [ "$status" = "healthy" ] && break
  sleep 2
done
[ "$status" = "healthy" ] || fail "container health is $status"
echo "container: $status"

echo "== clean stop removes runtime files"
docker stop -t 20 "$NAME" >/dev/null
[ ! -e "$D/emulator_credentials" ] || fail "credentials file left after stop"

echo "== default run (no arguments)"
docker run -d --name "$NAME-default" -e EMULATOR_TELEMETRY=off "$IMAGE" >/dev/null
for _ in $(seq 30); do
  status=$(docker inspect -f '{{.State.Health.Status}}' "$NAME-default")
  [ "$status" = "healthy" ] && break
  sleep 2
done
[ "$status" = "healthy" ] || fail "default container health is $status"
echo "default: $status"

echo "PASS"
