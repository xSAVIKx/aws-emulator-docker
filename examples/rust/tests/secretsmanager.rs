//! Secrets: create a JSON secret, read it back, delete it.

mod common;

use serde_json::{Value, json};

#[tokio::test]
async fn secretsmanager_json_secret_round_trip() {
    let config = common::sdk_config().await;
    let secrets = aws_sdk_secretsmanager::Client::new(&config);
    let name = common::unique_name();
    let value = json!({ "username": "app", "password": "s3cr3t" });

    secrets
        .create_secret()
        .name(&name)
        .secret_string(value.to_string())
        .send()
        .await
        .expect("CreateSecret");

    let read = secrets.get_secret_value().secret_id(&name).send().await.expect("GetSecretValue");
    let parsed: Value =
        serde_json::from_str(read.secret_string().expect("SecretString")).expect("valid JSON");
    assert_eq!(parsed, value);

    // Without force_delete_without_recovery, AWS keeps the secret for a
    // recovery window of 7 to 30 days.
    secrets
        .delete_secret()
        .secret_id(&name)
        .force_delete_without_recovery(true)
        .send()
        .await
        .expect("DeleteSecret");

    // The secret is gone. Assert only that the read fails: right after a
    // forced delete, the emulator returns InvalidRequestException, not
    // ResourceNotFoundException.
    let after = secrets.get_secret_value().secret_id(&name).send().await;
    assert!(after.is_err(), "GetSecretValue must fail after DeleteSecret");
}
