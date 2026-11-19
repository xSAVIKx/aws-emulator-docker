#!/usr/bin/env bash
# Check that all registries serve the same image for a tag.
#
# Usage: scripts/check-registry-parity.sh <tag> [<tag> ...]
#
# For each tag, the script:
#   1. Reads the manifest-list digest from each registry. It does not pull the image.
#   2. Prints a table of the digests.
#   3. Fails when a registry does not have the tag or the digests are different.
#   4. Verifies the keyless cosign signature on each registry (cosign v3 or later).
#
# Environment:
#   PARITY_IMAGES     Space-separated image repositories. The default is the three
#                     public registries.
#   COSIGN_IDENTITY   The certificate identity. The default is the release workflow.
#   COSIGN_ISSUER     The OIDC issuer. The default is GitHub Actions.
#
# Requirements: docker buildx, jq, cosign v3 or later.
set -euo pipefail

IMAGES=${PARITY_IMAGES:-"ghcr.io/xsavikx/aws-emulator docker.io/xsavikx/aws-emulator quay.io/xsavikx/aws-emulator"}
COSIGN_IDENTITY=${COSIGN_IDENTITY:-"https://github.com/xSAVIKx/aws-emulator-docker/.github/workflows/release.yml@refs/heads/main"}
COSIGN_ISSUER=${COSIGN_ISSUER:-"https://token.actions.githubusercontent.com"}

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 <tag> [<tag> ...]" >&2
  exit 2
fi

for tool in docker jq cosign; do
  command -v "$tool" >/dev/null || { echo "ERROR: $tool is not installed." >&2; exit 2; }
done

# Print the manifest-list digest of an image reference, or nothing on failure.
digest_of() {
  docker buildx imagetools inspect "$1" --format '{{json .Manifest}}' 2>/dev/null \
    | jq -r '.digest // empty' 2>/dev/null || true
}

failed=0
read -r -a repos <<<"$IMAGES"

for tag in "$@"; do
  echo "== Tag: $tag"
  printf '%-45s %-73s %s\n' "IMAGE" "DIGEST" "SIGNATURE"
  first=""
  for repo in "${repos[@]}"; do
    digest=$(digest_of "$repo:$tag")
    if [ -z "$digest" ]; then
      printf '%-45s %-73s %s\n' "$repo" "MISSING" "-"
      failed=1
      continue
    fi
    if err=$(cosign verify "$repo@$digest" \
        --certificate-identity "$COSIGN_IDENTITY" \
        --certificate-oidc-issuer "$COSIGN_ISSUER" 2>&1 >/dev/null); then
      sig=ok
    else
      sig=FAIL
      failed=1
      echo "ERROR: cosign verify failed for $repo@$digest:" >&2
      echo "$err" | tail -5 >&2
    fi
    printf '%-45s %-73s %s\n' "$repo" "$digest" "$sig"
    if [ -z "$first" ]; then
      first=$digest
    elif [ "$digest" != "$first" ]; then
      echo "ERROR: $repo:$tag has digest $digest. The first registry has $first." >&2
      failed=1
    fi
  done
  echo
done

if [ "$failed" -ne 0 ]; then
  echo "FAIL: The registries do not agree, or a signature is not valid." >&2
  exit 1
fi
echo "OK: All registries serve the same signed digest."
