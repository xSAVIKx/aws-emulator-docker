// Queue: create a queue, send a message, receive it.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

mod common;

#[tokio::test]
async fn sqs_send_and_receive_message() {
    let emulator = common::emulator().run().await.expect("start the emulator");
    let config = common::sdk_config(&emulator).await;
    let sqs = aws_sdk_sqs::Client::new(&config);

    let created = sqs
        .create_queue()
        .queue_name(common::unique_name())
        .send()
        .await
        .expect("CreateQueue");
    let queue_url = created.queue_url().expect("QueueUrl");

    sqs.send_message()
        .queue_url(queue_url)
        .message_body("hello from testcontainers-rs")
        .send()
        .await
        .expect("SendMessage");
    let received = sqs
        .receive_message()
        .queue_url(queue_url)
        .max_number_of_messages(1)
        .wait_time_seconds(5)
        .send()
        .await
        .expect("ReceiveMessage");

    let messages = received.messages();
    assert_eq!(messages.len(), 1);
    assert_eq!(messages[0].body(), Some("hello from testcontainers-rs"));
}
