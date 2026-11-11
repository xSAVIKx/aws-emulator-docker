//! Queue: create a queue, send a message, receive and delete it, delete the queue.

mod common;

#[tokio::test]
async fn sqs_send_receive_delete() {
    let config = common::sdk_config().await;
    let sqs = aws_sdk_sqs::Client::new(&config);
    let name = common::unique_name();

    let created = sqs.create_queue().queue_name(&name).send().await.expect("CreateQueue");
    // The emulator returns an AWS-style URL, for example
    // https://sqs.us-east-1.amazonaws.com/000000000000/<name>. Pass it back
    // unchanged. The AWS SDK for Rust puts the QueueUrl only in the request
    // body. It sends the request to AWS_ENDPOINT_URL, not to the QueueUrl
    // host, so no extra setting is needed (unlike the JavaScript SDK).
    let queue_url = created.queue_url().expect("QueueUrl").to_string();
    println!("QueueUrl: {queue_url}");

    sqs.send_message()
        .queue_url(&queue_url)
        .message_body("hello from Rust")
        .send()
        .await
        .expect("SendMessage");

    let received = sqs
        .receive_message()
        .queue_url(&queue_url)
        .max_number_of_messages(1)
        // Long polling: wait up to 5 seconds for a message.
        .wait_time_seconds(5)
        .send()
        .await
        .expect("ReceiveMessage");
    let messages = received.messages();
    assert_eq!(messages.len(), 1);
    assert_eq!(messages[0].body(), Some("hello from Rust"));

    // A received message comes back after its visibility timeout unless you delete it.
    sqs.delete_message()
        .queue_url(&queue_url)
        .receipt_handle(messages[0].receipt_handle().expect("ReceiptHandle"))
        .send()
        .await
        .expect("DeleteMessage");

    sqs.delete_queue().queue_url(&queue_url).send().await.expect("DeleteQueue");
}
