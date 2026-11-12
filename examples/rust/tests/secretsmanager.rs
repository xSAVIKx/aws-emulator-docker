//! Secrets: create a JSON secret, read it back, delete it.

mod common;

use std::cell::Cell;

use serde_json::{Value, json};

#[tokio::test]
async fn secretsmanager_json_secret_round_trip() {
    let config = common::sdk_config().await;
    let secrets = aws_sdk_secretsmanager::Client::new(&config);
    let name = common::unique_name();
    let value = json!({ "username": "app", "password": "s3cr3t" });
    // The secret exists from CreateSecret until the test deletes it.
    // The cleanup deletes the secret only while it exists.
    let secret_exists = Cell::new(false);

    let test = async {
        secrets
            .create_secret()
            .name(&name)
            .secret_string(value.to_string())
            .send()
            .await
            .expect("CreateSecret");
        secret_exists.set(true);

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
        secret_exists.set(false);

        // The deletion is asynchronous. Right after the delete, real AWS can
        // also return InvalidRequestException instead of
        // ResourceNotFoundException. Assert only that the read fails.
        let after = secrets.get_secret_value().secret_id(&name).send().await;
        assert!(after.is_err(), "GetSecretValue must fail after DeleteSecret");
    };

    // Clean up also when the test fails before the delete.
    let cleanup = async {
        let mut failures = Vec::new();
        if secret_exists.get() {
            let deleted = secrets
                .delete_secret()
                .secret_id(&name)
                .force_delete_without_recovery(true)
                .send()
                .await;
            common::note(&mut failures, "DeleteSecret", deleted);
        }
        failures
    };

    common::run_with_cleanup(test, cleanup).await;
}
