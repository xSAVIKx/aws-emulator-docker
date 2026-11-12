//! Key-value table: create a table, wait until it is ACTIVE, put two items,
//! get one, query both, delete the table.

mod common;

use std::cell::Cell;
use std::time::Duration;

// The `Waiters` trait adds the `wait_until_*` methods to the client.
use aws_sdk_dynamodb::client::Waiters;
use aws_sdk_dynamodb::types::{
    AttributeDefinition, AttributeValue, BillingMode, KeySchemaElement, KeyType,
    ScalarAttributeType, TableStatus,
};

#[tokio::test]
async fn dynamodb_table_put_get_query() {
    let config = common::sdk_config().await;
    let ddb = aws_sdk_dynamodb::Client::new(&config);
    let table = common::unique_name();
    // The body sets this flag. The cleanup deletes the table if it exists.
    let table_created = Cell::new(false);

    let test = async {
        // A table with a partition key "pk" and a sort key "sk", both strings.
        let created = ddb
            .create_table()
            .table_name(&table)
            .billing_mode(BillingMode::PayPerRequest)
            .attribute_definitions(attribute("pk"))
            .attribute_definitions(attribute("sk"))
            .key_schema(key("pk", KeyType::Hash))
            .key_schema(key("sk", KeyType::Range))
            .send()
            .await
            .expect("CreateTable");
        table_created.set(true);
        // Like AWS, the emulator returns CREATING first. Accept ACTIVE too,
        // because a table can become ACTIVE fast.
        let status = created.table_description().and_then(|t| t.table_status());
        assert!(
            matches!(status, Some(TableStatus::Creating | TableStatus::Active)),
            "unexpected CreateTable status: {status:?}"
        );

        // The SDK waiter calls DescribeTable until the status is ACTIVE.
        ddb.wait_until_table_exists()
            .table_name(&table)
            .wait(Duration::from_secs(60))
            .await
            .expect("table becomes ACTIVE");
        let described = ddb.describe_table().table_name(&table).send().await.expect("DescribeTable");
        assert_eq!(
            described.table().and_then(|t| t.table_status()),
            Some(&TableStatus::Active)
        );

        for (sk, title) in [("book#1", "Dune"), ("book#2", "Solaris")] {
            ddb.put_item()
                .table_name(&table)
                .item("pk", AttributeValue::S("author#1".into()))
                .item("sk", AttributeValue::S(sk.into()))
                .item("title", AttributeValue::S(title.into()))
                .send()
                .await
                .expect("PutItem");
        }

        let got = ddb
            .get_item()
            .table_name(&table)
            .key("pk", AttributeValue::S("author#1".into()))
            .key("sk", AttributeValue::S("book#2".into()))
            .send()
            .await
            .expect("GetItem");
        let item = got.item().expect("the item exists");
        assert_eq!(item.get("title"), Some(&AttributeValue::S("Solaris".into())));

        let queried = ddb
            .query()
            .table_name(&table)
            .key_condition_expression("pk = :pk")
            .expression_attribute_values(":pk", AttributeValue::S("author#1".into()))
            .send()
            .await
            .expect("Query");
        let titles: Vec<&str> = queried
            .items()
            .iter()
            .filter_map(|i| i.get("title")?.as_s().ok())
            .map(String::as_str)
            .collect();
        // Query returns the items in sort-key order.
        assert_eq!(titles, ["Dune", "Solaris"]);
    };

    // Clean up also when the test fails.
    let cleanup = async {
        let mut failures = Vec::new();
        if table_created.get() {
            let deleted = ddb.delete_table().table_name(&table).send().await;
            common::note(&mut failures, "DeleteTable", deleted);
        }
        failures
    };

    common::run_with_cleanup(test, cleanup).await;
}

fn attribute(name: &str) -> AttributeDefinition {
    AttributeDefinition::builder()
        .attribute_name(name)
        .attribute_type(ScalarAttributeType::S)
        .build()
        .expect("valid attribute definition")
}

fn key(name: &str, key_type: KeyType) -> KeySchemaElement {
    KeySchemaElement::builder()
        .attribute_name(name)
        .key_type(key_type)
        .build()
        .expect("valid key schema element")
}
