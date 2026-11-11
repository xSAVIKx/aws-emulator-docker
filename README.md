# AWS Emulator container image

[![Build and publish](https://github.com/xSAVIKx/aws-emulator-docker/actions/workflows/release.yml/badge.svg)](https://github.com/xSAVIKx/aws-emulator-docker/actions/workflows/release.yml)
[![Lint](https://github.com/xSAVIKx/aws-emulator-docker/actions/workflows/lint.yml/badge.svg)](https://github.com/xSAVIKx/aws-emulator-docker/actions/workflows/lint.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![GitHub Marketplace](https://img.shields.io/badge/Marketplace-Run%20AWS%20Emulator-orange?logo=github)](https://github.com/marketplace/actions/run-aws-emulator)

Website: <https://xsavikx.github.io/aws-emulator-docker/>

A ready-to-run, multi-architecture container image of the
[AWS Emulator](https://github.com/aws/aws-emulator), a local emulator of the AWS APIs.

AWS does not publish a prebuilt image. The
[official guide](https://docs.aws.amazon.com/emulator/latest/userguide/getting-started-docker.html)
tells you to build one yourself. This project builds that image, tests it, signs it, and
publishes it for you. A scheduled workflow checks for a new upstream release every hour.

Write-up with `docker run`, Compose, GitHub Actions and the gotchas:
[Running AWS Emulator in Docker, on any arch, in CI](https://serhiichuk.dev/blog/aws-emulator-container-image/)

> This is an unofficial community project. It is not affiliated with or endorsed by
> Amazon Web Services.

## Images

| Registry | Image |
|---|---|
| GitHub Container Registry | `ghcr.io/xsavikx/aws-emulator` |
| Docker Hub | `docker.io/xsavikx/aws-emulator` |
| Quay.io | `quay.io/xsavikx/aws-emulator` |

Platforms: `linux/amd64` and `linux/arm64`. Docker Desktop on macOS (Intel and Apple
silicon) and on Windows (x64 and Arm, WSL 2 backend) runs these Linux images. Docker,
Podman, containerd and Kubernetes on Linux run them natively.

### Tags

| Tag | Meaning |
|---|---|
| `latest`, `lts` | The newest LTS release |
| `1.0.322.1-<yyyymmdd>.<run>` | One build. This tag never moves. |
| `1.0.322.1` | The exact upstream package version |
| `1.0.322` | The emulator version (as `aws-emulator --version` reports it) |
| `1.0`, `1` | The newest release in that line |
| `sha-<commit>` | The build from one commit of this repository |

### Variants

| Variant | Base image | Tags |
|---|---|---|
| Amazon Linux (default) | `amazonlinux:2023-minimal` | `latest`, `1.0.322.1`, ... |
| Debian | `debian:trixie-slim` (Debian 13) | `debian`, `latest-debian`, `1.0.322.1-debian`, `1.0-debian`, ... |

Each tag of the default variant has a Debian twin with the `-debian` suffix. Both variants
install the same upstream emulator: the Amazon Linux variant from the `.rpm` package, the
Debian variant from the `.deb` package. Both have the same entry point, user, port and
health check. Use the Debian variant if your tooling expects `apt` and Debian.

The version tags move when the image is rebuilt (for example for a base image fix). For
reproducible builds, pin a build tag or a digest. Each build is listed in the
[releases](https://github.com/xSAVIKx/aws-emulator-docker/releases).

## Quick start

1. Create the working directory before the first start:

   ```sh
   mkdir -p ~/.aws/emulator
   ```

2. Start the emulator (macOS and Linux):

   ```sh
   PORT=9297
   docker run --rm --memory=2g --memory-swap=2g -p 127.0.0.1:$PORT:$PORT \
     --user "$(id -u):$(id -g)" \
     -e HOME="$HOME" -v "$HOME/.aws/emulator:$HOME/.aws/emulator" \
     ghcr.io/xsavikx/aws-emulator -p $PORT
   ```

   Windows PowerShell:

   ```powershell
   New-Item -ItemType Directory -Force "$env:USERPROFILE\.aws\emulator"
   $port = 9297
   docker run --rm --memory=2g --memory-swap=2g -p "127.0.0.1:${port}:${port}" `
     -v "$env:USERPROFILE\.aws\emulator:/home/emulator/.aws/emulator" `
     ghcr.io/xsavikx/aws-emulator -p $port
   ```

3. Point the AWS CLI or an SDK at it:

   ```sh
   export AWS_ENDPOINT_URL=https://127.0.0.1:9297
   export AWS_SHARED_CREDENTIALS_FILE=~/.aws/emulator/9297/emulator_credentials
   export AWS_CONFIG_FILE=~/.aws/emulator/9297/config
   export AWS_CA_BUNDLE=~/.aws/emulator/9297/ca.pem
   export AWS_EC2_METADATA_DISABLED=true
   export AWS_DISABLE_HOST_PREFIX_INJECTION=true
   export AWS_REGION=us-east-1 AWS_DEFAULT_REGION=us-east-1
   unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_PROFILE

   aws sts get-caller-identity   # Account: 000000000000
   ```

   Node.js needs `NODE_EXTRA_CA_CERTS`, Rust and .NET need `SSL_CERT_FILE`, and Java needs
   the `truststore.p12` file. See the
   [SDK guide](https://docs.aws.amazon.com/emulator/latest/userguide/point-sdk-to-emulator.html).

4. Stop it with Ctrl+C or `docker stop`. The emulator removes its files from
   `~/.aws/emulator/<port>/`. All state is in memory and goes away.

### Docker Compose

See [`compose.yaml`](compose.yaml).

```sh
mkdir -p ~/.aws/emulator
UID_GID="$(id -u):$(id -g)" docker compose up
```

## Examples and GitHub Action

- [`examples/`](examples/) has integration tests in Python (pytest), TypeScript (Vitest),
  Java (JUnit 5), Go (`go test`), C# (xUnit) and Rust (`cargo test`). They cover STS, S3,
  DynamoDB, SQS, SNS to SQS and Secrets Manager. CI runs them on amd64 and arm64.
- The [Run AWS Emulator](https://github.com/marketplace/actions/run-aws-emulator) GitHub Action ([source](https://github.com/xSAVIKx/run-aws-emulator)) starts
  the emulator in your own workflow and sets the SDK environment:

  ```yaml
  - uses: xSAVIKx/run-aws-emulator@v1
  - run: aws sts get-caller-identity
  ```

## Important facts

- **Bind address.** The image starts `aws-emulator -b 0.0.0.0`, so Docker can forward the
  port. Always publish the port on `127.0.0.1`. A plain `-p 9297:9297` exposes the emulator
  to your whole network.
- **Same port on both sides.** The emulator writes its files to `~/.aws/emulator/<port>/`.
  If you change the port, change it in all three places: the host side of `-p`, the
  container side of `-p`, and the emulator `-p` argument. Do not use `-p 0`.
- **TLS.** The emulator serves HTTPS with a self-signed certificate that it creates at each
  start (`ca.pem`). Your client must trust that file.
- **Memory.** Give the container 2 GiB. If it goes over the limit, Docker stops it with exit
  code 137 and you lose all emulated resources.
- **Telemetry.** The upstream emulator sends anonymous usage telemetry by default. This image
  does not change that. To turn it off, add `-e EMULATOR_TELEMETRY=off`. A variable in your
  own shell does not reach the container.
- **Health check.** The image has a Docker `HEALTHCHECK` that calls `/__admin/health`. If you
  use a port other than 9297, also set `-e EMULATOR_HEALTHCHECK_PORT=<port>`.
- **User.** The image runs as UID 1000. On Linux with rootful Docker, add
  `--user "$(id -u):$(id -g)"` so the files in `~/.aws/emulator` belong to you. With rootless
  Docker, use `--user 0:0` instead: container root maps to your host user. Without it, or
  with your own UID, the emulator stops with `Permission denied (os error 13)`. With Podman,
  use `--userns=keep-id`.
- **Other containers.** The emulator certificate names only `localhost`, `127.0.0.1` and
  `::1`. A client in another container that calls `https://aws-emulator:9297` fails TLS
  validation. Run the client in the network namespace of the emulator instead
  (`docker run --network container:<emulator> ...` with `https://127.0.0.1:<port>`).
- **Use `127.0.0.1`, not `localhost`.** With a host name, some SDKs (for example the AWS SDK
  for JavaScript v3) use virtual-hosted S3 addresses such as `bucket.localhost`, which do not
  resolve.
- **No Windows containers.** The emulator runs only on Linux. On Windows, use Linux
  containers (Docker Desktop with WSL 2).

## Supply chain

Every published image has:

- Open Container Initiative labels and index annotations (`org.opencontainers.image.*`),
  including `version`, `revision`, `created`, `source`, `licenses`, `base.name` and
  `base.digest`.
- An SBOM and SLSA provenance (`mode=max`) attestation from BuildKit.
- A keyless [cosign](https://github.com/sigstore/cosign) signature.
- A GitHub artifact attestation (GHCR).

The build downloads a pinned upstream version (from `versions.json`), checks the SHA-256
digest of each RPM, and checks the RPM signature with the AWS package signing key
(`SSM Publishing <ssm-publishing@amazon.com>`, fingerprint
`98BC F336 A3BE 53B7 34E9 67C5 2E5B F49B D940 F624`, from
<https://downloads.aws.com/keys/gpg/current.asc>, stored in [`keys/`](keys/)). The base image
is pinned by digest.

Verify a signature (cosign v3 or later; v2 reports "no signatures found"):

```sh
cosign verify ghcr.io/xsavikx/aws-emulator:latest \
  --certificate-identity-regexp '^https://github.com/xSAVIKx/aws-emulator-docker/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Verify the GitHub attestation:

```sh
gh attestation verify oci://ghcr.io/xsavikx/aws-emulator:latest --owner xSAVIKx
```

## Build it yourself

```sh
docker buildx build --platform linux/amd64,linux/arm64 -t aws-emulator .
docker buildx build --platform linux/amd64,linux/arm64 -f Dockerfile.debian -t aws-emulator:debian .
```

The Dockerfile downloads the pinned LTS RPM for each target architecture and checks its
signature (the Debian variant checks the detached `.sig` file with `gpg`). To build another version, pass `--build-arg EMULATOR_VERSION=<version>` with a
version from
[`versions.json`](https://downloads.aws.com/aws-emulator/packages/aws-emulator/LTS/versions.json),
or `latest`. Pass `--build-arg RPM_SHA256_AMD64=<sha>` and `--build-arg RPM_SHA256_ARM64=<sha>` (`DEB_SHA256_*` for Debian)
to also check the SHA-256 digests. Run the tests with `tests/smoke.sh aws-emulator`.

## Licenses

- The files in this repository: [Apache License 2.0](LICENSE).
  Copyright 2026 Yurii Serhiichuk.
- The AWS Emulator: CC0 1.0 Universal, with code in its `vendor/` directory under the
  Apache License 2.0. See [aws/aws-emulator](https://github.com/aws/aws-emulator).
- Amazon Linux 2023 and Debian base images: see their package licenses.

## Author

Created and maintained by **Yurii Serhiichuk** ([yurii@serhiichuk.dev](mailto:yurii@serhiichuk.dev),
[serhiichuk.dev](https://serhiichuk.dev), [@xSAVIKx](https://github.com/xSAVIKx)).
Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).
