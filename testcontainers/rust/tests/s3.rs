// Object storage: create a bucket, put an object, get it back.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

mod common;

use aws_sdk_s3::primitives::ByteStream;

#[tokio::test]
async fn s3_put_and_get_object() {
    let emulator = common::emulator().run().await.expect("start the emulator");
    let config = common::sdk_config(&emulator).await;
    // The endpoint host is an IP address or localhost, so S3 must use
    // path-style addresses (https://host:port/bucket/key).
    let s3_config = aws_sdk_s3::config::Builder::from(&config)
        .force_path_style(true)
        .build();
    let s3 = aws_sdk_s3::Client::from_conf(s3_config);

    let bucket = common::unique_name();
    let key = "greetings/hello.txt";
    let body = "Hello from testcontainers-rs";

    s3.create_bucket()
        .bucket(&bucket)
        .send()
        .await
        .expect("CreateBucket");
    s3.put_object()
        .bucket(&bucket)
        .key(key)
        .body(ByteStream::from_static(body.as_bytes()))
        .send()
        .await
        .expect("PutObject");
    let object = s3
        .get_object()
        .bucket(&bucket)
        .key(key)
        .send()
        .await
        .expect("GetObject");
    let bytes = object
        .body
        .collect()
        .await
        .expect("read the body")
        .into_bytes();

    assert_eq!(bytes.as_ref(), body.as_bytes());
}
