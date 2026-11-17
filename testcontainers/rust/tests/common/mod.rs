// Helpers that every test file shares.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

//! Each file in `tests/` is a separate test binary. Each binary includes this
//! module with `mod common;` and uses only some of the helpers.
#![allow(dead_code)]

use std::collections::hash_map::RandomState;
use std::hash::{BuildHasher, Hasher};

use aws_config::{BehaviorVersion, Region, SdkConfig};
use aws_sdk_sts::config::Credentials;
use aws_smithy_http_client::tls::{self, TlsContext, TrustStore};
use testcontainers_aws_emulator::{AwsEmulator, DEFAULT_IMAGE, StartedAwsEmulator};

/// Returns the module for the image in `AWS_EMULATOR_IMAGE`, or for the
/// default image when the variable is not set.
pub fn emulator() -> AwsEmulator {
    match std::env::var("AWS_EMULATOR_IMAGE") {
        Ok(image) if !image.trim().is_empty() => AwsEmulator::new(image.trim()),
        _ => AwsEmulator::new(DEFAULT_IMAGE),
    }
}

/// Makes an SDK configuration for the emulator.
///
/// The HTTPS client trusts only the CA of the emulator. It does not load the
/// trust roots of the operating system.
pub async fn sdk_config(emulator: &StartedAwsEmulator) -> SdkConfig {
    let trust_store = TrustStore::empty()
        .with_native_roots(false)
        .with_pem_certificate(emulator.ca_pem_bytes());
    let tls_context = TlsContext::builder()
        .with_trust_store(trust_store)
        .build()
        .expect("TLS context");
    let http_client = aws_smithy_http_client::Builder::new()
        .tls_provider(tls::Provider::Rustls(
            tls::rustls_provider::CryptoMode::AwsLc,
        ))
        .tls_context(tls_context)
        .build_https();

    aws_config::defaults(BehaviorVersion::latest())
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
        .await
}

/// Returns a unique lowercase name such as `tc-rs-3f9a1c0b`.
pub fn unique_name() -> String {
    let random = RandomState::new().build_hasher().finish() as u32;
    format!("tc-rs-{random:08x}")
}
