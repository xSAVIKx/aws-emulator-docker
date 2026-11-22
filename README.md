# AWS Emulator container image

[![Build and publish](https://github.com/xSAVIKx/aws-emulator-docker/actions/workflows/release.yml/badge.svg)](https://github.com/xSAVIKx/aws-emulator-docker/actions/workflows/release.yml)
[![Lint](https://github.com/xSAVIKx/aws-emulator-docker/actions/workflows/lint.yml/badge.svg)](https://github.com/xSAVIKx/aws-emulator-docker/actions/workflows/lint.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![GitHub Marketplace](https://img.shields.io/badge/Marketplace-Run%20AWS%20Emulator-orange?logo=github)](https://github.com/marketplace/actions/run-aws-emulator)

Website: <https://xsavikx.github.io/aws-emulator-docker/>

This project publishes a multi-architecture container image of the
[AWS Emulator](https://github.com/aws/aws-emulator), a local emulator of the AWS APIs.
You can run the image immediately. You do not build it first.

AWS does not publish a prebuilt image. The
[official guide](https://docs.aws.amazon.com/emulator/latest/userguide/getting-started-docker.html)
tells you to build one yourself. This project builds that image, tests it, signs it, and
publishes it for you. A scheduled workflow checks for a new upstream release every hour.
After a new upstream release, the workflow usually publishes a new image in about one to two
hours.

A blog post about `docker run`, Compose, GitHub Actions and known problems:
[Running AWS Emulator in Docker, on any arch, in CI](https://serhiichuk.dev/blog/aws-emulator-container-image/)

> This is an unofficial community project. It is not affiliated with or endorsed by
> Amazon Web Services.

## Quick start

```sh
mkdir -p ~/.aws/emulator
docker run --rm -p 127.0.0.1:9297:9297 --user "$(id -u):$(id -g)" \
  -e HOME="$HOME" -v "$HOME/.aws/emulator:$HOME/.aws/emulator" ghcr.io/xsavikx/aws-emulator
```

In a second terminal:

```sh
export AWS_ENDPOINT_URL=https://127.0.0.1:9297 AWS_CA_BUNDLE=~/.aws/emulator/9297/ca.pem \
  AWS_SHARED_CREDENTIALS_FILE=~/.aws/emulator/9297/emulator_credentials AWS_CONFIG_FILE=~/.aws/emulator/9297/config
aws sts get-caller-identity   # "Account": "000000000000"
```

With rootless Docker, use `--user 0:0`. In CI, use the
[Run AWS Emulator](https://github.com/marketplace/actions/run-aws-emulator) action. For
Windows, SDKs and Compose, see [Run it step by step](#run-it-step-by-step).

### Compared with the image from the AWS guide

The [AWS guide](https://docs.aws.amazon.com/emulator/latest/userguide/getting-started-docker.html)
tells you to build the image yourself. This project gives you a built image that is also smaller.
The sizes are for `linux/amd64`, uncompressed:

| Image | Size |
|---|---|
| Built with the Dockerfile from the AWS guide | 1.86 GB |
| This project, Amazon Linux (default) | 1.49 GB |
| This project, Debian | 1.48 GB |
| This project, distroless | 1.39 GB |

The guide's Dockerfile uses `ADD` for the RPM package. The `rm` in the next step does not
remove the 180 MB package from the image, because the package stays in the `ADD` layer.
This project mounts the package only for the install step. Measured on 2026-11-14 with
emulator 1.0.367 (guide) and 1.0.366 (this project).

The guide's image also does not check the package signature and runs no tests. This project
checks the AWS signature on each package, tests each image, and signs it. See
[Supply chain](#supply-chain).

## Images

GitHub Container Registry:

```sh
docker pull ghcr.io/xsavikx/aws-emulator
```

Docker Hub:

```sh
docker pull docker.io/xsavikx/aws-emulator
```

Quay.io:

```sh
docker pull quay.io/xsavikx/aws-emulator
```

For the Debian or distroless variant, add the `:debian` or `:distroless` tag. All tags and
digests are the same on the three registries.

Platforms: `linux/amd64` and `linux/arm64`. Docker Desktop on macOS (Intel and Apple
silicon) and on Windows (x64 and Arm, WSL 2 backend) runs these Linux images. Docker,
Podman, containerd and Kubernetes on Linux run them natively.

### Tags

| Tag | Meaning |
|---|---|
| `latest`, `lts` | The newest LTS release |
| `<version>-<yyyymmdd>.<run>` | One build. This tag never moves. |
| `<version>` | The exact upstream package version, for example `1.0.322.1` |
| `<emulator-version>` | The emulator version (as `aws-emulator --version` reports it), for example `1.0.322` |
| `1.0`, `1` | The newest release in that line |
| `sha-<commit>` | The build from one commit of this repository |

### Variants

| Variant | Base image | Tags |
|---|---|---|
| Amazon Linux (default) | `public.ecr.aws/amazonlinux/amazonlinux:2023-minimal` | `latest`, `<version>`, ... |
| Debian | `debian:trixie-slim` (Debian 13) | `debian`, `latest-debian`, `<version>-debian`, `1.0-debian`, ... |
| Distroless | `gcr.io/distroless/cc-debian13` | `distroless`, `latest-distroless`, `<version>-distroless`, ... |

Each tag of the default variant has a matching tag with the `-debian` suffix and a matching
tag with the `-distroless` suffix. The three variants install the same upstream emulator.
The Amazon Linux variant installs it from the `.rpm` package. The Debian and distroless
variants install it from the `.deb` package. The three variants use the same user (UID 1000),
port and health check route. Use the Debian variant if your tooling expects `apt` and Debian.

The distroless variant contains only glibc, the emulator and a small static health check
program in Go. It has no shell, no package manager and no `curl`, so security scanners
report very few findings. You cannot `docker exec` a shell into it. Its Docker `HEALTHCHECK`
runs the Go program. The other two variants run a shell script with `curl`. The entry point
of the distroless variant is `/opt/aws-emulator/bin/aws-emulator -b 0.0.0.0`. The other two
variants use `aws-emulator -b 0.0.0.0`.

The version tags move when the image is rebuilt (for example for a base image fix). For
reproducible builds, pin a build tag or a digest. Each build is listed in the
[releases](https://github.com/xSAVIKx/aws-emulator-docker/releases).

## Run it step by step

1. Create the working directory before the first start:

   ```sh
   mkdir -p ~/.aws/emulator
   ```

2. Start the emulator (macOS and Linux). With rootless Docker, use `--user 0:0` instead of
   `--user "$(id -u):$(id -g)"`. Otherwise the start fails with
   `Permission denied (os error 13)`.

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

4. Stop it with Ctrl+C or `docker stop`. The emulator removes its runtime files from
   `~/.aws/emulator/<port>/`. All state is in memory and goes away.

### Docker Compose

See [`compose.yaml`](compose.yaml).

```sh
mkdir -p ~/.aws/emulator
UID_GID="$(id -u):$(id -g)" docker compose up
# With rootless Docker:
UID_GID=0:0 docker compose up
```

## Call it from other containers

The emulator certificate names only `localhost`, `127.0.0.1` and `::1`
([aws/aws-emulator#23](https://github.com/aws/aws-emulator/issues/23)). A client in another
container that calls `https://aws:9297` therefore fails TLS validation. The image has an
optional TLS front end for this case. Set one of these variables to turn it on:

| Variable | Effect |
|---|---|
| `EMULATOR_TLS_NAMES` | More names for the certificate, separated by commas: service names, host names, IP addresses, or wildcards such as `*.example.test`. |
| `EMULATOR_TLS_CA_CERT`, `EMULATOR_TLS_CA_KEY` | Your own CA certificate and key (PEM files) to sign the certificate. |
| `EMULATOR_HTTP_PORT` | A second port that accepts plain HTTP, for a private network. |

With the front end, the emulator listens on `127.0.0.1` in the container. The front end
listens on the same port on the other addresses of the container. It serves a certificate
for these names:

- `localhost`, `*.localhost`, `000000000000.localhost` (S3 Control), `127.0.0.1` and `::1`,
- the container host name and its IP addresses,
- the names in `EMULATOR_TLS_NAMES`.

The front end passes each connection to the emulator without changes, so SigV4
signatures, presigned URLs and queue URLs work as before. A CA signs the certificate. The
front end makes the CA at the first start and keeps it in `~/.aws/emulator-ca/`. When you
put that folder on a volume, the CA stays the same after a restart. The folder is not in
`~/.aws/emulator/`, so clients that mount the run folder do not get the CA key.

> [!CAUTION]
> Do not add the CA to the trust store of an operating system or a browser. The CA can sign
> a certificate for any name. Give it only to the test clients, for example with
> `AWS_CA_BUNDLE`.

In `~/.aws/emulator/<port>/`, the front end writes:

- `tls-ca.pem`: the CA only. Give this file to clients in other containers.
- `ca.pem` and `truststore.p12`: the CA and the emulator certificate. Clients on the host
  can use these files as before.

The Docker health check reports healthy only after the front end is ready.
[`examples/compose/compose.yaml`](examples/compose/compose.yaml) is a complete example. CI
tests the front end at each build with [`tests/tls.sh`](tests/tls.sh).

```yaml
services:
  aws:
    image: ghcr.io/xsavikx/aws-emulator:latest
    environment:
      EMULATOR_TLS_NAMES: aws
    volumes:
      - emulator:/home/emulator/.aws/emulator
  tests:
    image: public.ecr.aws/aws-cli/aws-cli:latest
    depends_on:
      aws: { condition: service_healthy }
    environment:
      AWS_ENDPOINT_URL: https://aws:9297
      AWS_CA_BUNDLE: /emulator/9297/tls-ca.pem
      AWS_SHARED_CREDENTIALS_FILE: /emulator/9297/emulator_credentials
      AWS_REGION: us-east-1
    volumes:
      - emulator:/emulator:ro
    command: ["sts", "get-caller-identity"]
volumes:
  emulator:
```

Limits of the front end:

- Do not set `-b`, and do not use `-p 0`. The front end needs a fixed port.
- Do not use it with `--network host`. Use a container network.
- The front end listens on the addresses that the container has at the start. It does not
  listen on a network that you connect later with `docker network connect`.
- OpenSSL and curl do not accept `*.localhost` for `bucket.localhost`. Use path-style S3
  addresses.
- The emulator files belong to UID 1000. Run the client container as UID 1000 or as root,
  or give the emulator the same user as the client.

## Examples and GitHub Action

- [`examples/`](examples/) has integration tests in Python (pytest), TypeScript (Vitest),
  Java (JUnit 5), Go (`go test`), C# (xUnit) and Rust (`cargo test`). They cover STS, S3,
  DynamoDB, SQS, SNS to SQS and Secrets Manager. CI runs them on amd64 and arm64 every day
  and after each image release.
- [`testcontainers/`](testcontainers/) has Testcontainers modules for Python, TypeScript,
  Java, Go, .NET and Rust. A test starts its own emulator and gets the endpoint, the
  credentials and a TLS helper. No volume and no fixed port are necessary.
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
  `::1`. To call the emulator by a service name, set `EMULATOR_TLS_NAMES`. See
  [Call it from other containers](#call-it-from-other-containers).
- **Use `127.0.0.1`, not `localhost`.** With a host name, some SDKs use virtual-hosted S3
  addresses such as `bucket.localhost`. These addresses do not resolve. The AWS SDK for
  JavaScript v3 is an example of such an SDK.
- **No Windows containers.** The emulator runs only on Linux. On Windows, use Linux
  containers (Docker Desktop with WSL 2).

### Known emulator limits

These limits come from the upstream emulator, not from this image. The image runs the
unchanged upstream package. The issues are open in
[aws/aws-emulator](https://github.com/aws/aws-emulator/issues). This list states only what
each issue reports. Check the issue for the current status.

- **Cognito, TOTP-only MFA
  ([#542](https://github.com/aws/aws-emulator/issues/542)).** A user pool with
  `MfaConfiguration: "ON"` and only `SOFTWARE_TOKEN_MFA` fails to create in a CloudFormation
  stack. The message asks for an SMS configuration. The issue reports that version 1.0.497-1
  creates the pool, and that 1.0.498-1 and 1.0.500-1 do not.
- **Lambda, `provided.al2023`
  ([#539](https://github.com/aws/aws-emulator/issues/539)).** `CreateFunction` accepts the
  runtime. `Invoke` fails with `InvalidRuntimeException`. The message says that the emulator
  runs the Python and node runtimes.
- **ALB to ECS target
  ([#521](https://github.com/aws/aws-emulator/issues/521)).** The issue reports that
  `DescribeTargetHealth` shows the target as healthy, but each request to the ALB listener
  returns HTTP 502 with an empty body. It was reported on version 1.0.385.
- **Process exit after bulk Lambda updates
  ([#527](https://github.com/aws/aws-emulator/issues/527)).** The issue reports that the
  emulator container exited after about 103 back-to-back `UpdateFunctionConfiguration`
  calls with a new layer version. All state was lost. A run with a pause after each 10
  functions did not exit. The cause is not confirmed in the issue.
- **No official image
  ([#14](https://github.com/aws/aws-emulator/issues/14)).** The issue asks AWS to publish an
  official multi-arch container image. It is open. This project is an unofficial image.

### Pin a version

The `latest` tag moves to each new upstream release. If a release adds a regression, your
tests can start to fail with no change in your code. To stay on one emulator version, use a
version tag in place of `latest`:

```sh
docker pull ghcr.io/xsavikx/aws-emulator:1.0.322.1
```

- `<version>` (for example `1.0.322.1`) is the exact upstream package version. It moves
  when this project rebuilds the image.
- `<version>-<yyyymmdd>.<run>` is one build. It never moves.
- A digest (`ghcr.io/xsavikx/aws-emulator@sha256:...`) never changes.
- Add `-debian` or `-distroless` to the tag for those variants.

See [Tags](#tags) for the full list. The [releases](https://github.com/xSAVIKx/aws-emulator-docker/releases)
page lists each build and its digest. The version numbers in the upstream issues use a
different form than the image tags. Look for the tag in the releases before you use it. The
releases do not list each upstream version.

## Supply chain

Every published image has:

- Open Container Initiative labels and index annotations (`org.opencontainers.image.*`),
  including `version`, `revision`, `created`, `source`, `licenses`, `base.name` and
  `base.digest`.
- An SBOM and SLSA provenance (`mode=max`) attestation from BuildKit.
- A keyless [cosign](https://github.com/sigstore/cosign) signature.
- A GitHub artifact attestation (GHCR).

The build downloads a pinned upstream version (from `versions.json`). It checks the SHA-256
digest of each package. It checks the RPM or DEB package signature with the AWS package
signing key:

- Signer: `SSM Publishing <ssm-publishing@amazon.com>`
- Fingerprint: `98BC F336 A3BE 53B7 34E9 67C5 2E5B F49B D940 F624`
- Source: <https://downloads.aws.com/keys/gpg/current.asc>, stored in [`keys/`](keys/)

Each base image is pinned by digest.

Verify a signature (cosign v3 or later; v2 reports "no signatures found"):

```sh
cosign verify ghcr.io/xsavikx/aws-emulator:latest \
  --certificate-identity 'https://github.com/xSAVIKx/aws-emulator-docker/.github/workflows/release.yml@refs/heads/main' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Without a local cosign installation, run cosign as a container:

```sh
docker run --rm ghcr.io/sigstore/cosign/cosign:v3.1.4 verify ghcr.io/xsavikx/aws-emulator:latest \
  --certificate-identity 'https://github.com/xSAVIKx/aws-emulator-docker/.github/workflows/release.yml@refs/heads/main' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Verify the GitHub attestation:

```sh
gh attestation verify oci://ghcr.io/xsavikx/aws-emulator:latest --owner xSAVIKx --format json
```

### What a correct signature tells you

The AWS signature on the package tells you that the emulator binary comes from AWS. The
cosign signature and the attestations tell you that the image comes from the release
workflow of this repository, from a known commit. These checks do not tell you that the
emulator code is free of defects. This project does not review or change the upstream code.
Use the emulator for local and CI tests only. Do not give it real AWS credentials or
production data.

## Build it yourself

For a local test, build for the platform of your host and load the image into Docker
with `--load`:

```sh
docker buildx build --load -t aws-emulator:dev .
docker buildx build --load -f Dockerfile.debian -t aws-emulator:dev-debian .
docker buildx build --load -f Dockerfile.distroless -t aws-emulator:dev-distroless .
tests/smoke.sh aws-emulator:dev
```

A multi-platform build needs `--push` to a registry. Without `--load` or `--push`, the build
keeps the result only in the build cache, and Docker has no local image:

```sh
docker buildx build --platform linux/amd64,linux/arm64 --push -t <registry>/aws-emulator:dev .
```

Each Dockerfile downloads the pinned LTS package for each target architecture. The Amazon
Linux variant checks the signature of the `.rpm` package with `rpmkeys`. The Debian and
distroless variants check the detached `.sig` file of the `.deb` package with `gpg`.

To build another version, pass `--build-arg EMULATOR_VERSION=<version>`. Use a version from
[`versions.json`](https://downloads.aws.com/aws-emulator/packages/aws-emulator/LTS/versions.json),
or `latest`. To also check the SHA-256 digests, pass `--build-arg RPM_SHA256_AMD64=<sha>` and
`--build-arg RPM_SHA256_ARM64=<sha>`. For the Debian and distroless variants, pass
`DEB_SHA256_AMD64` and `DEB_SHA256_ARM64` instead.

## Licenses

- The files in this repository: [Apache License 2.0](LICENSE).
  Copyright 2026 Yurii Serhiichuk.
- The AWS Emulator: CC0 1.0 Universal, with code in its `vendor/` directory under the
  Apache License 2.0. See [aws/aws-emulator](https://github.com/aws/aws-emulator).
- The base images `public.ecr.aws/amazonlinux/amazonlinux:2023-minimal`, `debian:trixie-slim`
  and `gcr.io/distroless/cc-debian13`: see the licenses of their packages.

## Author

Created and maintained by **Yurii Serhiichuk** ([yurii@serhiichuk.dev](mailto:yurii@serhiichuk.dev),
[serhiichuk.dev](https://serhiichuk.dev), [@xSAVIKx](https://github.com/xSAVIKx)).
Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).
