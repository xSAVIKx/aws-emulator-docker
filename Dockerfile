# syntax=docker/dockerfile:1.10
#
# Container image for the AWS Emulator (https://github.com/aws/aws-emulator).
# Based on the official guide:
# https://docs.aws.amazon.com/emulator/latest/userguide/getting-started-docker.html
#
# Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
# SPDX-License-Identifier: Apache-2.0

ARG BASE_IMAGE=public.ecr.aws/amazonlinux/amazonlinux:2023-minimal@sha256:61cf86a8168c2e0b8fc6b6badaac64968044c947718508d88fac06e61a596e6d

# ---------------------------------------------------------------------------
# Stage 1: download the RPM for the target architecture.
# The RPM stays out of the final image layers.
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM ${BASE_IMAGE} AS fetch
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ARG TARGETARCH
ARG EMULATOR_CHANNEL=LTS
ARG EMULATOR_DOWNLOAD_BASE=https://downloads.aws.com/aws-emulator/packages/aws-emulator
# Optional SHA-256 digests. CI sets them. When set, the build fails on a mismatch.
ARG RPM_SHA256_AMD64=""
ARG RPM_SHA256_ARM64=""
RUN set -eu; \
    case "${TARGETARCH}" in \
      amd64) rpm_arch=x86_64;  want="${RPM_SHA256_AMD64}" ;; \
      arm64) rpm_arch=aarch64; want="${RPM_SHA256_ARM64}" ;; \
      *) echo "Unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    url="${EMULATOR_DOWNLOAD_BASE}/${EMULATOR_CHANNEL}/latest/aws-emulator-${rpm_arch}.rpm"; \
    echo "Downloading ${url}"; \
    curl -fsSL --retry 5 --retry-all-errors -o /aws-emulator.rpm "${url}"; \
    if [ -n "${want}" ]; then \
      echo "${want}  /aws-emulator.rpm" | sha256sum -c -; \
    fi

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
