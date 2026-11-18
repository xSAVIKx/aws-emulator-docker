# testcontainers-aws-emulator (Rust)

<!-- Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev> -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

This crate is a [Testcontainers for Rust](https://docs.rs/testcontainers) module for the
AWS Emulator image (`ghcr.io/xsavikx/aws-emulator`). The struct `AwsEmulator` implements
the `testcontainers::Image` trait. The API is async and uses Tokio.

The container serves HTTPS on port 9297. A CA in the container signs the server
certificate. After the start, the module copies the credentials file and the CA file out
of the container. The module uses the Docker archive API for this. It does not run a
command in the container, so the module also works with the distroless image.

The crate does not depend on the AWS SDK.

## Install

The crate is not published on crates.io. Add it from Git:

```toml
[dev-dependencies]
testcontainers-aws-emulator = { git = "https://github.com/xSAVIKx/aws-emulator-docker" }
tokio = { version = "1", features = ["macros", "rt-multi-thread"] }
```

Cargo finds the crate in the `testcontainers/rust` folder of the repository. To use a local
copy, set `path = "<repository>/testcontainers/rust"` instead of `git`.

The minimum Rust version of the library is 1.88. The AWS SDK for Rust needs Rust 1.94.1 or later. `rust-toolchain.toml` pins Rust 1.100.0 for the tests of this module.

## Usage with the AWS SDK for Rust

Add the SDK crates and `aws-smithy-http-client` with a rustls feature:

```toml
[dev-dependencies]
aws-config = { version = "1", default-features = false, features = ["behavior-version-latest", "rt-tokio"] }
aws-sdk-sts = { version = "1", default-features = false, features = ["rt-tokio"] }
aws-smithy-http-client = { version = "1", features = ["rustls-aws-lc"] }
```

The example starts the container and makes an SDK configuration. The HTTPS client trusts
only the CA of the emulator.

```rust
use aws_config::{BehaviorVersion, Region};
use aws_sdk_sts::config::Credentials;
use aws_smithy_http_client::tls::{self, TlsContext, TrustStore};
use testcontainers_aws_emulator::AwsEmulator;

#[tokio::test]
async fn get_caller_identity() {
    let emulator = AwsEmulator::default().run().await.unwrap();

    let trust_store = TrustStore::empty()
        .with_native_roots(false)
        .with_pem_certificate(emulator.ca_pem_bytes());
    let tls_context = TlsContext::builder().with_trust_store(trust_store).build().unwrap();
    let http_client = aws_smithy_http_client::Builder::new()
        .tls_provider(tls::Provider::Rustls(tls::rustls_provider::CryptoMode::AwsLc))
        .tls_context(tls_context)
        .build_https();

    let config = aws_config::defaults(BehaviorVersion::latest())
        .endpoint_url(emulator.endpoint())
        .region(Region::new(emulator.region()))
        .credentials_provider(Credentials::new(
            emulator.access_key_id(),
            emulator.secret_access_key(),
            None,
            None,
            "aws-emulator",
        ))
        .http_client(http_client)
        .load()
        .await;

    let sts = aws_sdk_sts::Client::new(&config);
    let identity = sts.get_caller_identity().send().await.unwrap();
    assert_eq!(identity.account(), Some("000000000000"));
}
```

For S3, set `force_path_style(true)` on the S3 configuration. The endpoint host is
`localhost` or an IP address, so virtual-hosted addresses do not work.

The container stops when the `StartedAwsEmulator` value is dropped.

## API

| Item | Description |
|---|---|
| `AwsEmulator::default()` | The module for `ghcr.io/xsavikx/aws-emulator:latest`. |
| `AwsEmulator::new(image)` | The module for another image, for example `aws-emulator:tls-al`. |
| `.with_tls_name(name)`, `.with_tls_names(names)` | Adds host names or IP addresses to the server certificate. |
| `.with_network_alias(network, alias)` | Puts the container on a Docker network. Other containers on the network reach it as `alias`. See "Limits". |
| `.with_telemetry(true)` | Turns the telemetry of the emulator on. The default is off. |
| `.request()` | Returns a `ContainerRequest` with the startup timeout (120 s) and the network. Use it to add more `ImageExt` settings. |
| `.run().await` | Starts the container and returns a `StartedAwsEmulator`. |
| `StartedAwsEmulator::from_container(container).await` | Reads the endpoint, the credentials and the CA from a container that you started with `AsyncRunner::start`. |
| `.endpoint()` | The endpoint URL, `https://<host>:<mapped port>`. |
| `.region()` | `us-east-1`. |
| `.access_key_id()`, `.secret_access_key()` | The credentials. The emulator requires SigV4 with these credentials. |
| `.ca_pem()`, `.ca_pem_bytes()` | The CA certificate (PEM). |
| `.ca_file()` | The path of a temporary file with the CA. The file exists until the value is dropped. |
| `.environment()` | `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `AWS_DEFAULT_REGION`, `AWS_CA_BUNDLE` and `AWS_EC2_METADATA_DISABLED=true`, for a child process. |
| `.rustls_root_store()` | A `rustls::RootCertStore` with only the CA. It needs the `rustls` feature. |
| `.container()` | The `ContainerAsync<AwsEmulator>`. |

The constants `DEFAULT_IMAGE`, `PORT`, `REGION`, `ACCOUNT_ID` and `STARTUP_TIMEOUT` are
public.

The module sets these values on the container:

- The command `-p 9297`.
- `EMULATOR_TLS_NAMES`: the Testcontainers host, the network alias and the extra names.
  When the list is empty, the value is `localhost`.
- `EMULATOR_TELEMETRY=off`, unless telemetry is on.
- The wait condition: the log line `TLS front end ready` on stderr.

The AWS SDK for Rust does not read `AWS_CA_BUNDLE`. Use the trust store in the example, or
set `SSL_CERT_FILE` to a file with the CA for `rustls-native-certs`.

## The host of the endpoint

The module uses the host that testcontainers-rs gives (`ContainerAsync::get_host`). When the
environment variable `TESTCONTAINERS_HOST_OVERRIDE` is set, the module uses its value
instead. The module also adds this host to the certificate names before the start. It finds
the host from `TESTCONTAINERS_HOST_OVERRIDE` or from a TCP `DOCKER_HOST`. For a local
socket, it uses `localhost`.

Set `TESTCONTAINERS_HOST_OVERRIDE` when the tests run in a container with the host network.
In that case, testcontainers-rs finds `/.dockerenv` and gives the bridge gateway as the host,
but the mapped ports are on `127.0.0.1`.

## Run the tests

Run this command from the module folder:

```sh
AWS_EMULATOR_IMAGE=aws-emulator:tls-al ./run-tests.sh
```

`AWS_EMULATOR_IMAGE` selects the image for the tests. The default is
`ghcr.io/xsavikx/aws-emulator:latest`. The library does not read this variable.

When `cargo` is on `PATH`, the script runs `cargo test --locked --all-features`. When
`cargo` is not on `PATH`, the script runs the same command in the `rust:1.100` container.
The script mounts the Docker socket and sets `TESTCONTAINERS_HOST_OVERRIDE=127.0.0.1`. With
rootless Docker, the script runs the container as root, because root in the container maps
to the host user. `CARGO_CONTAINER_DIR` sets the folder for the Cargo cache and the build
output (the default is `target/container`).

The tests are:

| File | Test |
|---|---|
| `tests/sts.rs` | STS `GetCallerIdentity` returns the account `000000000000`. The test also checks the CA file and the environment map. |
| `tests/s3.rs` | S3: create a bucket, put an object, get the object (path-style addresses). |
| `tests/sqs.rs` | SQS: create a queue, send a message, receive the message. |
| `tests/network.rs` | A `curlimages/curl` container on the same network calls `https://<alias>:9297/__admin/health` with the CA and gets `ok`. |
| `src/lib.rs` | Unit tests for the image reference, the credentials file and the environment variables. |

Each test file starts its own container.

## Limits

- testcontainers-rs 0.28 has no API for network aliases. On a user-defined Docker network,
  the DNS name of a container is its container name. `with_network_alias(network, alias)`
  thus sets the container name to `alias`. A container name is unique on the Docker host.
  Use a unique alias, for example `aws-<random>`, not `aws`. The test uses such a name.
- The module cannot add a network alias after the start. The TLS front end listens only on
  the addresses that the container has at the start.
- `Image::env_vars` cannot see the settings of the `ContainerRequest`. If you set a network
  or a container name with `ImageExt`, add the name to the certificate with
  `with_tls_name`.
- testcontainers-rs reads the Docker host also from `~/.testcontainers.properties`. The
  module does not read that file to find the certificate names. The certificate always
  contains `localhost`, `127.0.0.1` and `::1`.
- The `Image` trait has no startup timeout. `run()` and `request()` set 120 s. If you call
  `AsyncRunner::start` on `AwsEmulator` directly, the timeout of testcontainers-rs is 60 s.
  Add `.with_startup_timeout(STARTUP_TIMEOUT)`.
- For a digest reference (`name@sha256:<hex>`), `Image::name` returns `name@sha256` and
  `Image::tag` returns `<hex>`. Testcontainers joins them to the correct reference.

## License

Apache-2.0. See the repository [LICENSE](../../LICENSE).
