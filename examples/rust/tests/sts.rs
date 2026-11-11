//! Identity: STS `GetCallerIdentity` returns the emulator account.

mod common;

#[tokio::test]
async fn sts_get_caller_identity_returns_emulator_account() {
    let config = common::sdk_config().await;
    let sts = aws_sdk_sts::Client::new(&config);

    let identity = sts.get_caller_identity().send().await.expect("GetCallerIdentity");

    assert_eq!(identity.account(), Some("000000000000"));
}
