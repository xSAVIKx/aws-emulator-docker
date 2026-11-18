# Testcontainers modules for the AWS Emulator

These modules start the AWS Emulator image from a test with
[Testcontainers](https://testcontainers.com/). Each module starts a fresh emulator, reads its
credentials and CA certificate, and gives you the endpoint, the credentials, and a TLS
helper that trusts only the emulator CA. You do not need a volume, a fixed port, or a shell
script.

| Language | Directory | Package | Testcontainers library |
|---|---|---|---|
| Python | [`python/`](python/) | `testcontainers-aws-emulator` | testcontainers-python |
| TypeScript (Node.js) | [`typescript/`](typescript/) | `testcontainers-aws-emulator` | Testcontainers for Node.js |
| Java | [`java/`](java/) | `io.github.xsavikx:testcontainers-aws-emulator` | Testcontainers for Java |
| Go | [`go/`](go/) | `github.com/xSAVIKx/aws-emulator-docker/testcontainers/go` | Testcontainers for Go |
| C# (.NET) | [`dotnet/`](dotnet/) | `AwsEmulator.Testcontainers` | Testcontainers for .NET |
| Rust | [`rust/`](rust/) | `testcontainers-aws-emulator` | testcontainers-rs |

The packages are not on the package registries yet. Each module README tells you how to
install it from this repository. The Go module works with `go get` at a commit.

## How the modules work

1. The module starts the image with `-p 9297` and a random host port for 9297.
2. The module sets `EMULATOR_TLS_NAMES` to the Testcontainers host and the network aliases.
   The TLS front end of the image then serves a certificate for these names. See
   [Call it from other containers](../README.md#call-it-from-other-containers).
3. The module waits for the log line `TLS front end ready`.
4. The module copies `emulator_credentials` and `tls-ca.pem` from the container. It uses the
   Docker archive API, so the distroless variant also works.
5. The module sets `EMULATOR_TELEMETRY=off`, unless you turn telemetry on.

The modules need an image with the TLS front end. The first image release after
[pull request #9](https://github.com/xSAVIKx/aws-emulator-docker/pull/9) has it.

## Tests

Each module has tests with the official AWS SDK of its language: STS, S3 and SQS, and a call
by a network alias from a second container. The
[Testcontainers workflow](../.github/workflows/testcontainers.yml) runs them:

- for each pull request and push, against an image that CI builds from the same commit,
- every week, against the published `latest` image.

To run the tests of one module, run `./run-tests.sh` in its directory. Set
`AWS_EMULATOR_IMAGE` to test another image.
