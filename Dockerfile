# syntax=docker/dockerfile:1.10@sha256:865e5dd094beca432e8c0a1d5e1c465db5f998dca4e439981029b3b81fb39ed5
#
# Container image for the AWS Emulator (https://github.com/aws/aws-emulator).
# Based on the official guide:
# https://docs.aws.amazon.com/emulator/latest/userguide/getting-started-docker.html
#
# Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
# SPDX-License-Identifier: Apache-2.0

ARG BASE_IMAGE=public.ecr.aws/amazonlinux/amazonlinux:2023-minimal@sha256:61cf86a8168c2e0b8fc6b6badaac64968044c947718508d88fac06e61a596e6d

# ---------------------------------------------------------------------------
# Stage 1: download the RPM for the target architecture and verify it.
# The RPM stays out of the final image layers.
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM ${BASE_IMAGE} AS fetch
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ARG TARGETARCH
# Upstream package version from versions.json, for example 1.0.322.1.
# Set it to "latest" to use the moving LTS/latest URL.
ARG EMULATOR_VERSION=1.0.322.1
ARG EMULATOR_CHANNEL=LTS
ARG EMULATOR_DOWNLOAD_BASE=https://downloads.aws.com/aws-emulator/packages/aws-emulator
# AWS package signing key "SSM Publishing <ssm-publishing@amazon.com>".
# Source: https://downloads.aws.com/keys/gpg/current.asc
ARG RPM_SIGNER_FINGERPRINT=98BCF336A3BE53B734E967C52E5BF49BD940F624
# Optional SHA-256 digests. CI sets them. When set, the build fails on a mismatch.
ARG RPM_SHA256_AMD64=""
ARG RPM_SHA256_ARM64=""
COPY keys/aws-emulator-signing-key.asc /keys/aws-emulator-signing-key.asc
RUN set -eu; \
    case "${TARGETARCH}" in \
      amd64) rpm_arch=x86_64;  want="${RPM_SHA256_AMD64}" ;; \
      arm64) rpm_arch=aarch64; want="${RPM_SHA256_ARM64}" ;; \
      *) echo "Unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    base="${EMULATOR_DOWNLOAD_BASE}/${EMULATOR_CHANNEL}"; \
    if [ "${EMULATOR_VERSION}" = "latest" ]; then \
      url="${base}/latest/aws-emulator-${rpm_arch}.rpm"; \
    else \
      url="${base}/aws-emulator-${EMULATOR_VERSION%.*}-${EMULATOR_VERSION##*.}-${rpm_arch}.rpm"; \
    fi; \
    echo "Downloading ${url}"; \
    curl -fsSL --retry 5 --retry-all-errors -o /aws-emulator.rpm "${url}"; \
    if [ -n "${want}" ]; then \
      echo "${want}  /aws-emulator.rpm" | sha256sum -c -; \
    fi; \
    # Trust only the AWS key: remove the keys that the base image ships.
    rpm -e --allmatches gpg-pubkey >/dev/null 2>&1 || true; \
    rpmkeys --import /keys/aws-emulator-signing-key.asc; \
    test "$(rpm -q gpg-pubkey | wc -l)" -eq 1; \
    rpm -q --qf '%{VERSION}-%{RELEASE}\n' gpg-pubkey; \
    fp_short="$(echo "${RPM_SIGNER_FINGERPRINT}" | tail -c 9 | tr 'A-F' 'a-f')"; \
    rpm -q gpg-pubkey --qf '%{VERSION}\n' | grep -qx "${fp_short}"; \
    rpmkeys --checksig /aws-emulator.rpm | tee /dev/stderr | grep -Eq ': (digests )?signatures OK$'

# ---------------------------------------------------------------------------
# Stage 2: runtime image.
# ---------------------------------------------------------------------------
FROM ${BASE_IMAGE}

ARG EMULATOR_UID=1000
ARG EMULATOR_GID=1000
ARG BASE_IMAGE

# Static Open Container Initiative labels. CI adds or overrides the dynamic
# labels (version, created, revision, base.digest).
LABEL org.opencontainers.image.title="AWS Emulator" \
      org.opencontainers.image.description="Multi-arch container image of the AWS Emulator, a local emulator of the AWS APIs. Unofficial community build." \
      org.opencontainers.image.authors="Yurii Serhiichuk <yurii@serhiichuk.dev>" \
      org.opencontainers.image.vendor="Yurii Serhiichuk" \
      org.opencontainers.image.url="https://github.com/xSAVIKx/aws-emulator-docker" \
      org.opencontainers.image.source="https://github.com/xSAVIKx/aws-emulator-docker" \
      org.opencontainers.image.documentation="https://github.com/xSAVIKx/aws-emulator-docker#readme" \
      org.opencontainers.image.licenses="Apache-2.0 AND CC0-1.0" \
      org.opencontainers.image.base.name="${BASE_IMAGE}"

RUN --mount=type=bind,from=fetch,source=/aws-emulator.rpm,target=/mnt/pkg/aws-emulator.rpm \
    set -eu; \
    rpm -i /mnt/pkg/aws-emulator.rpm; \
    mkdir -p /home/emulator/.aws/emulator; \
    chown -R "${EMULATOR_UID}:${EMULATOR_GID}" /home/emulator; \
    rm -rf /var/cache/dnf /var/log/dnf*; \
    aws-emulator --version

COPY --chmod=0755 healthcheck.sh /usr/local/bin/aws-emulator-healthcheck

USER ${EMULATOR_UID}:${EMULATOR_GID}
ENV HOME=/home/emulator \
    EMULATOR_HEALTHCHECK_PORT=9297
WORKDIR /home/emulator

EXPOSE 9297
STOPSIGNAL SIGTERM

HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/usr/local/bin/aws-emulator-healthcheck"]

ENTRYPOINT ["aws-emulator", "-b", "0.0.0.0"]
