# Rust example

This example tests the AWS Emulator with the AWS SDK for Rust. It uses `cargo test` and
Tokio (`#[tokio::test]`). It runs the six scenarios in [`../README.md`](../README.md#scenarios).

## Prerequisites

- Docker.
- Rust stable with Cargo. The example uses edition 2024, so it needs Rust 1.85 or later.
  Install Rust with [rustup](https://rustup.rs/).
- A C compiler (`cc`). The default TLS library of the SDK (`aws-lc-rs`) compiles C code.

## Run

From the repository root, run:

```sh
examples/with-emulator.sh examples/rust/run-tests.sh
```

The first build downloads and compiles the dependencies. This takes some minutes.

`run-tests.sh` runs `cargo test --locked -- --nocapture`. The tests run in parallel.
Each test makes resources with a unique name (`ex-rs-<random>`) and deletes them at the end.

## Files

| File | Content |
|---|---|
| `Cargo.toml`, `Cargo.lock` | The pinned dependencies: `aws-config` and one SDK crate for each service. |
| `run-tests.sh` | The test command. |
| `tests/common/mod.rs` | Shared helpers: the endpoint guard, the SDK configuration, unique names. |
| `tests/sts.rs` | Identity: `GetCallerIdentity`. |
| `tests/s3.rs` | Object storage, with path-style addresses (`force_path_style(true)`). |
| `tests/dynamodb.rs` | Key-value table, with the `wait_until_table_exists` waiter. |
| `tests/sqs.rs` | Queue. |
| `tests/sns_sqs.rs` | Fan-out from an SNS topic to an SQS queue. |
| `tests/secretsmanager.rs` | Secrets. |

Each file in `tests/` compiles to its own test binary.

## How the tests connect

The code contains no endpoint, keys or certificate paths. `aws_config::load_from_env()`
reads `AWS_ENDPOINT_URL`, `AWS_REGION` and the credentials and config files that the start
script sets. The `behavior-version-latest` feature of `aws-config` selects the latest SDK
behavior.

The endpoint guard in `tests/common/mod.rs` stops each test unless `AWS_ENDPOINT_URL` points
to `127.0.0.1` or `localhost`.

The emulator returns an AWS-style SQS `QueueUrl`, for example
`https://sqs.us-east-1.amazonaws.com/000000000000/<name>`. The tests pass it back unchanged.
The AWS SDK for Rust sends SQS requests to `AWS_ENDPOINT_URL`, not to the `QueueUrl` host.
Thus no extra setting is necessary.

## TLS

The emulator uses HTTPS with a self-signed CA. The tests trust that CA with no extra code:

1. The default HTTPS client of the SDK uses rustls. It loads its trust roots with the
   `rustls-native-certs` crate.
2. `rustls-native-certs` reads `SSL_CERT_FILE` when that variable is set. It then uses the
   certificates in that file instead of the system store.
3. The start script sets `SSL_CERT_FILE` to a combined bundle: the system CAs plus the
   emulator CA. Thus the client trusts the emulator and the real AWS endpoints.

The AWS SDK for Rust does not read `AWS_CA_BUNDLE`. If you run the tests without the start
script, set `SSL_CERT_FILE` to a bundle that contains the emulator CA.

`Cargo.toml` turns off the default features of the SDK crates and turns on only
`default-https-client` and `rt-tokio`. The default features also build a legacy HTTP client
that the tests do not use.
