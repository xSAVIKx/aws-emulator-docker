//! Object storage: create a bucket, put an object, list and get it, delete both.

mod common;

use std::cell::Cell;

use aws_sdk_s3::primitives::ByteStream;

#[tokio::test]
async fn s3_bucket_and_object_round_trip() {
    let config = common::sdk_config().await;
    // The endpoint is an IP address, so S3 must use path-style addresses
    // (https://127.0.0.1:9297/bucket/key), not virtual-hosted addresses.
    let s3_config = aws_sdk_s3::config::Builder::from(&config)
        .force_path_style(true)
        .build();
    let s3 = aws_sdk_s3::Client::from_conf(s3_config);

    let bucket = common::unique_name();
    let key = "greetings/hello.txt";
    let body = "Hello from the AWS SDK for Rust";
    // The body sets this flag. The cleanup deletes only what exists.
    let bucket_created = Cell::new(false);

    let test = async {
        s3.create_bucket().bucket(&bucket).send().await.expect("CreateBucket");
        bucket_created.set(true);

        s3.put_object()
            .bucket(&bucket)
            .key(key)
            .content_type("text/plain")
            .body(ByteStream::from_static(body.as_bytes()))
            .send()
            .await
            .expect("PutObject");

        let listing = s3.list_objects_v2().bucket(&bucket).send().await.expect("ListObjectsV2");
        let keys: Vec<&str> = listing.contents().iter().filter_map(|o| o.key()).collect();
        assert_eq!(keys, [key]);

        let object = s3.get_object().bucket(&bucket).key(key).send().await.expect("GetObject");
        // The body is a stream. `collect` reads all of it into memory.
        let bytes = object.body.collect().await.expect("read object body").into_bytes();
        assert_eq!(bytes.as_ref(), body.as_bytes());
    };

    // Clean up also when the test fails. A bucket must be empty before you
    // can delete it. DeleteObject passes also when the object does not exist.
    let cleanup = async {
        let mut failures = Vec::new();
        if bucket_created.get() {
            let deleted = s3.delete_object().bucket(&bucket).key(key).send().await;
            common::note(&mut failures, "DeleteObject", deleted);
            let deleted = s3.delete_bucket().bucket(&bucket).send().await;
            common::note(&mut failures, "DeleteBucket", deleted);
        }
        failures
    };

    common::run_with_cleanup(test, cleanup).await;
}
