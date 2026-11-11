//! Helpers that every test file shares.
//!
//! Each file in `tests/` is a separate test binary. Each binary includes this
//! module with `mod common;` and uses only some of the helpers, so the
//! compiler does not warn about the helpers that one binary does not use.
#![allow(dead_code)]

use std::collections::hash_map::RandomState;
use std::hash::{BuildHasher, Hasher};

use aws_config::SdkConfig;

/// Loads the shared AWS configuration from the environment.
///
/// `aws_config::load_from_env` reads the same variables as the other AWS SDKs:
/// `AWS_ENDPOINT_URL`, `AWS_REGION`, `AWS_SHARED_CREDENTIALS_FILE` and
/// `AWS_CONFIG_FILE`. The code contains no endpoint and no keys.
///
/// TLS: the default HTTPS client of the SDK uses rustls and loads its trust
/// roots with `rustls-native-certs`. That crate reads `SSL_CERT_FILE` when it
/// is set. The start script sets `SSL_CERT_FILE` to the system CAs plus the
/// emulator CA, so the client trusts the emulator without extra code.
/// The Rust SDK does not read `AWS_CA_BUNDLE`.
///
/// Each test calls this function and gets its own clients. A client keeps a
/// connection pool, and a pool must not outlive the Tokio runtime of a test.
pub async fn sdk_config() -> SdkConfig {
    guard_endpoint();
    aws_config::load_from_env().await
}

/// Stops the test unless `AWS_ENDPOINT_URL` points to this computer.
///
/// This guard prevents a run against a real AWS account.
pub fn guard_endpoint() {
    let endpoint = std::env::var("AWS_ENDPOINT_URL").unwrap_or_default();
    let host = host_of(&endpoint);
    if host != "127.0.0.1" && host != "localhost" {
        panic!(
            "AWS_ENDPOINT_URL must point to the AWS Emulator on 127.0.0.1 or localhost, \
             but it is {endpoint:?}. Start the tests with examples/with-emulator.sh, \
             or run eval \"$(.github/actions/aws-emulator/start-emulator.sh)\" first."
        );
    }
}

/// Returns the host part of a URL such as `https://127.0.0.1:9297/`.
fn host_of(url: &str) -> &str {
    let rest = url.split_once("://").map_or(url, |(_, rest)| rest);
    let authority = rest.split(['/', '?', '#']).next().unwrap_or("");
    let host_port = authority.rsplit_once('@').map_or(authority, |(_, h)| h);
    host_port.split(':').next().unwrap_or("")
}

/// Returns a unique lowercase name such as `ex-rs-3f9a1c0b7d2e4a65`.
///
/// Tests run in parallel, and many runs can share one emulator, so each
/// resource gets its own name. The name is valid for S3 buckets, DynamoDB
/// tables, SQS queues, SNS topics and secrets.
pub fn unique_name() -> String {
    // `RandomState` gets a random seed from the operating system. A hash of
    // nothing with that seed is a random 64-bit number, without an extra crate.
    let random = RandomState::new().build_hasher().finish();
    format!("ex-rs-{random:016x}")
}

