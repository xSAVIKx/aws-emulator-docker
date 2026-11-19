#!/usr/bin/env bash
# Test of the TLS front end (EMULATOR_TLS_NAMES and EMULATOR_HTTP_PORT).
# Usage: tests/tls.sh <image>
# A client container on the same Docker network calls the emulator by its
# service name over HTTPS and over plain HTTP. The host calls it through the
# published port. A restart must keep the same CA. A second container uses
# a CA from files (EMULATOR_TLS_CA_CERT and EMULATOR_TLS_CA_KEY).
set -euo pipefail

IMAGE="${1:?usage: $0 <image>}"
CURL_IMAGE="${CURL_IMAGE:-curlimages/curl:8.23.0@sha256:3fb7e13e5d179a845c97d78204f3f1064d1907037f26a3ebf91710571151fd93}"
PORT="${PORT:-19298}"
HTTP_PORT=4566
WORK="${WORK:-$(mktemp -d)}"
H="$WORK/home"
D="$H/.aws/emulator/$PORT"
NAME="aws-emulator-tls-$$"
NET="aws-emulator-tls-$$"
OWN="$NAME-own-ca"
OWN_PORT=19299
# With rootless Docker, container root maps to the host user.
if docker info -f '{{.SecurityOptions}}' 2>/dev/null | grep -q rootless; then
  USER_SPEC=0:0
else
  USER_SPEC="$(id -u):$(id -g)"
fi

cleanup() {
  docker rm -f "$NAME" "$OWN" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  docker logs "$NAME" 2>&1 | tail -50 >&2 || true
  exit 1
}

start() {
  docker run -d --name "$NAME" --network "$NET" --network-alias aws --memory=2g --memory-swap=2g \
    -p "127.0.0.1:$PORT:$PORT" \
    --user "$USER_SPEC" \
    -e HOME="$H" -e EMULATOR_TELEMETRY=off -e EMULATOR_HEALTHCHECK_PORT="$PORT" \
    -e EMULATOR_TLS_NAMES="aws,aws.test,*.svc.test" -e EMULATOR_HTTP_PORT="$HTTP_PORT" \
    -v "$H/.aws/emulator:$H/.aws/emulator" -v "$H/.aws/emulator-ca:$H/.aws/emulator-ca" \
    "$IMAGE" -p "$PORT" >/dev/null
  for _ in $(seq 90); do
    [ -s "$D/tls-ca.pem" ] && return 0
    docker inspect -f '{{.State.Running}}' "$NAME" 2>/dev/null | grep -q true || fail "the container stopped"
    sleep 1
  done
  fail "the TLS front end did not get ready"
}

# client runs curl in a container on the same network, with the run folder mounted.
client() {
  docker run --rm --network "$NET" --user "$USER_SPEC" -v "$D:/run-dir:ro" "$CURL_IMAGE" "$@"
}

docker network create "$NET" >/dev/null
mkdir -p "$H/.aws/emulator" "$H/.aws/emulator-ca"

echo "== start with EMULATOR_TLS_NAMES=aws,aws.test,*.svc.test and EMULATOR_HTTP_PORT=$HTTP_PORT"
start
for f in emulator_credentials config ca.pem truststore.p12 tls-ca.pem; do
  [ -s "$D/$f" ] || fail "missing $D/$f"
done
[ "$(grep -c 'BEGIN CERTIFICATE' "$D/ca.pem")" = 2 ] || fail "ca.pem does not have two certificates"
ca_before=$(cat "$H/.aws/emulator-ca/ca.pem")

key=$(awk -F' *= *' '$1=="aws_access_key_id"{print $2}' "$D/emulator_credentials")
secret=$(awk -F' *= *' '$1=="aws_secret_access_key"{print $2}' "$D/emulator_credentials")
sts=(--aws-sigv4 "aws:amz:us-east-1:sts" --user "$key:$secret" -d 'Action=GetCallerIdentity&Version=2011-06-15')

echo "== host: https://127.0.0.1 and https://localhost with ca.pem"
for url in "https://127.0.0.1:$PORT" "https://localhost:$PORT"; do
  body=$(curl -sf --cacert "$D/ca.pem" "$url/__admin/health") || fail "$url health"
  [ "$body" = ok ] || fail "$url health body: $body"
done
resp=$(curl -sf --cacert "$D/ca.pem" "${sts[@]}" "https://127.0.0.1:$PORT/") || fail "host STS call"
echo "$resp" | grep -q 000000000000 || fail "host STS response: $resp"
echo "host: OK"

echo "== host: https://000000000000.localhost (S3 Control style host)"
code=$(curl -s -o /dev/null -w '%{http_code}' --cacert "$D/ca.pem" \
  --resolve "000000000000.localhost:$PORT:127.0.0.1" "https://000000000000.localhost:$PORT/__admin/health") || fail "TLS to 000000000000.localhost"
[ "$code" = 200 ] || fail "000000000000.localhost: HTTP $code"
echo "S3 Control host: OK"

echo "== container: https://aws:$PORT with tls-ca.pem"
body=$(client -sf --cacert /run-dir/tls-ca.pem "https://aws:$PORT/__admin/health") || fail "https://aws health"
[ "$body" = ok ] || fail "https://aws health body: $body"
resp=$(client -sf --cacert /run-dir/tls-ca.pem "${sts[@]}" "https://aws:$PORT/") || fail "https://aws STS call"
echo "$resp" | grep -q 000000000000 || fail "https://aws STS response: $resp"
body=$(client -sf --cacert /run-dir/ca.pem --resolve "aws.test:$PORT:$(docker inspect -f "{{(index .NetworkSettings.Networks \"$NET\").IPAddress}}" "$NAME")" \
  "https://aws.test:$PORT/__admin/health") || fail "https://aws.test health"
echo "service name: OK"

echo "== container: an unknown name fails verification"
if client -sf --cacert /run-dir/tls-ca.pem --resolve "other:$PORT:$(docker inspect -f "{{(index .NetworkSettings.Networks \"$NET\").IPAddress}}" "$NAME")" \
  "https://other:$PORT/__admin/health" >/dev/null 2>&1; then
  fail "the certificate accepted the name other"
fi
echo "unknown name: rejected"

ip=$(docker inspect -f "{{(index .NetworkSettings.Networks \"$NET\").IPAddress}}" "$NAME")

echo "== container: a wildcard name (s3.svc.test for *.svc.test)"
body=$(client -sf --cacert /run-dir/tls-ca.pem --resolve "s3.svc.test:$PORT:$ip" \
  "https://s3.svc.test:$PORT/__admin/health") || fail "https://s3.svc.test health"
[ "$body" = ok ] || fail "https://s3.svc.test health body: $body"
if client -sf --cacert /run-dir/tls-ca.pem --resolve "a.b.svc.test:$PORT:$ip" \
  "https://a.b.svc.test:$PORT/__admin/health" >/dev/null 2>&1; then
  fail "the wildcard accepted two labels (a.b.svc.test)"
fi
echo "wildcard: OK"

echo "== container: the container IP address $ip"
body=$(client -sf --cacert /run-dir/tls-ca.pem "https://$ip:$PORT/__admin/health") || fail "https://$ip health"
[ "$body" = ok ] || fail "https://$ip health body: $body"
echo "IP address: OK"

echo "== container: plain HTTP on http://aws:$HTTP_PORT"
resp=$(client -sf "${sts[@]}" "http://aws:$HTTP_PORT/") || fail "plain HTTP STS call"
echo "$resp" | grep -q 000000000000 || fail "plain HTTP STS response: $resp"
echo "plain HTTP: OK"

echo "== Docker HEALTHCHECK"
for _ in $(seq 30); do
  status=$(docker inspect -f '{{.State.Health.Status}}' "$NAME")
  [ "$status" = "healthy" ] && break
  sleep 2
done
[ "$status" = "healthy" ] || fail "container health is $status"
echo "container: $status"

echo "== restart keeps the CA"
docker stop -t 20 "$NAME" >/dev/null
[ ! -e "$D/tls-ca.pem" ] || fail "tls-ca.pem left after stop"
code=$(docker inspect -f '{{.State.ExitCode}}' "$NAME")
docker rm "$NAME" >/dev/null
echo "exit code after stop: $code"
start
[ "$(cat "$H/.aws/emulator-ca/ca.pem")" = "$ca_before" ] || fail "the CA changed after a restart"
body=$(client -sf --cacert /run-dir/tls-ca.pem "https://aws:$PORT/__admin/health") || fail "https://aws health after restart"
echo "restart: OK"

echo "== the CA key is not in the run folder"
[ ! -e "$H/.aws/emulator/tls" ] || fail "a CA folder is in ~/.aws/emulator"
echo "CA key: OK"

echo "== an invalid setting stops the container with a message"
out=$(docker run --rm -e EMULATOR_TLS_NAMES='bad*name' "$IMAGE" 2>&1) && fail "a bad name was accepted"
echo "$out" | grep -q 'is not a host name' || fail "unexpected message: $out"
out=$(docker run --rm -e EMULATOR_TLS_CA_CERT=/x.pem "$IMAGE" 2>&1) && fail "a CA certificate without a key was accepted"
echo "$out" | grep -q 'set both EMULATOR_TLS_CA_CERT and EMULATOR_TLS_CA_KEY' || fail "unexpected message: $out"
echo "invalid setting: OK"

echo "== a CA from files (EMULATOR_TLS_CA_CERT and EMULATOR_TLS_CA_KEY)"
OWN_DIR="$WORK/own-ca"
OWN_H="$WORK/own-home"
OWN_D="$OWN_H/.aws/emulator/$OWN_PORT"
mkdir -p "$OWN_DIR" "$OWN_H/.aws/emulator"
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 2 \
  -subj "/CN=tls.sh test CA" \
  -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -keyout "$OWN_DIR/ca-key.pem" -out "$OWN_DIR/ca.pem" 2>/dev/null
chmod 0644 "$OWN_DIR/ca-key.pem"
docker run -d --name "$OWN" --network "$NET" --network-alias aws-own --memory=2g --memory-swap=2g \
  --user "$USER_SPEC" \
  -e HOME="$OWN_H" -e EMULATOR_TELEMETRY=off -e EMULATOR_HEALTHCHECK_PORT="$OWN_PORT" \
  -e EMULATOR_TLS_NAMES=aws-own -e EMULATOR_TLS_CA_CERT=/ca/ca.pem -e EMULATOR_TLS_CA_KEY=/ca/ca-key.pem \
  -v "$OWN_DIR:/ca:ro" -v "$OWN_H/.aws/emulator:$OWN_H/.aws/emulator" \
  "$IMAGE" -p "$OWN_PORT" >/dev/null
for _ in $(seq 90); do
  [ -s "$OWN_D/tls-ca.pem" ] && break
  docker inspect -f '{{.State.Running}}' "$OWN" 2>/dev/null | grep -q true || fail "the own-CA container stopped"
  sleep 1
done
[ -s "$OWN_D/tls-ca.pem" ] || fail "the own-CA front end did not get ready"
cmp -s "$OWN_D/tls-ca.pem" "$OWN_DIR/ca.pem" || fail "tls-ca.pem is not the CA from EMULATOR_TLS_CA_CERT"
body=$(docker run --rm --network "$NET" --user "$USER_SPEC" -v "$OWN_DIR:/ca:ro" "$CURL_IMAGE" \
  -sf --cacert /ca/ca.pem "https://aws-own:$OWN_PORT/__admin/health") || fail "https://aws-own health with the own CA"
[ "$body" = ok ] || fail "https://aws-own health body: $body"
if docker run --rm --network "$NET" --user "$USER_SPEC" -v "$D:/run-dir:ro" "$CURL_IMAGE" \
  -sf --cacert /run-dir/tls-ca.pem "https://aws-own:$OWN_PORT/__admin/health" >/dev/null 2>&1; then
  fail "the generated CA of the first container also verified the own-CA container"
fi
echo "own CA: OK"

echo "PASS"
