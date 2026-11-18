// Identity: STS GetCallerIdentity returns the emulator account.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

mod common;

use testcontainers_aws_emulator::{ACCOUNT_ID, REGION};

#[tokio::test]
async fn sts_get_caller_identity_returns_emulator_account() {
    let emulator = common::emulator().run().await.expect("start the emulator");
    assert!(
        emulator.endpoint().starts_with("https://"),
        "{}",
        emulator.endpoint()
    );
    assert_eq!(emulator.region(), REGION);
    assert!(
        std::fs::read_to_string(emulator.ca_file())
            .unwrap()
            .contains("BEGIN CERTIFICATE")
    );
    let env = emulator.environment();
    assert_eq!(env["AWS_ENDPOINT_URL"], emulator.endpoint());
    assert_eq!(
        env["AWS_CA_BUNDLE"],
        emulator.ca_file().display().to_string()
    );
    assert_eq!(env["AWS_EC2_METADATA_DISABLED"], "true");
    #[cfg(feature = "rustls")]
    assert_eq!(emulator.rustls_root_store().expect("root store").len(), 1);

    let config = common::sdk_config(&emulator).await;
    let sts = aws_sdk_sts::Client::new(&config);
    let identity = sts
        .get_caller_identity()
        .send()
        .await
        .expect("GetCallerIdentity");

    assert_eq!(identity.account(), Some(ACCOUNT_ID));
}
