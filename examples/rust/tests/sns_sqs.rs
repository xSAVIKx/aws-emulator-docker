//! Fan-out: create a topic and a queue, allow the topic in the queue policy,
//! subscribe with raw delivery, publish, receive the message.

mod common;

use std::time::{Duration, Instant};

use aws_sdk_sqs::types::QueueAttributeName;
use serde_json::json;

#[tokio::test]
async fn sns_publish_reaches_sqs_subscriber() {
    let config = common::sdk_config().await;
    let sns = aws_sdk_sns::Client::new(&config);
    let sqs = aws_sdk_sqs::Client::new(&config);
    let name = common::unique_name();

    let topic_arn = sns
        .create_topic()
        .name(&name)
        .send()
        .await
        .expect("CreateTopic")
        .topic_arn()
        .expect("TopicArn")
        .to_string();

    let queue_url = sqs
        .create_queue()
        .queue_name(&name)
        .send()
        .await
        .expect("CreateQueue")
        .queue_url()
        .expect("QueueUrl")
        .to_string();
    let queue_arn = sqs
        .get_queue_attributes()
        .queue_url(&queue_url)
        .attribute_names(QueueAttributeName::QueueArn)
        .send()
        .await
        .expect("GetQueueAttributes")
        .attributes()
        .and_then(|a| a.get(&QueueAttributeName::QueueArn))
        .expect("QueueArn")
        .clone();

    // SNS may send to the queue only when the queue policy allows it.
    // The condition limits the permission to this one topic.
    let policy = json!({
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Principal": { "Service": "sns.amazonaws.com" },
            "Action": "sqs:SendMessage",
            "Resource": queue_arn,
            "Condition": { "ArnEquals": { "aws:SourceArn": topic_arn } }
        }]
    });
    sqs.set_queue_attributes()
        .queue_url(&queue_url)
        .attributes(QueueAttributeName::Policy, policy.to_string())
        .send()
        .await
        .expect("SetQueueAttributes");

    // With raw delivery, the queue gets the message body as it is,
    // not wrapped in an SNS JSON envelope.
    let subscription_arn = sns
        .subscribe()
        .topic_arn(&topic_arn)
        .protocol("sqs")
        .endpoint(&queue_arn)
        .attributes("RawMessageDelivery", "true")
        .return_subscription_arn(true)
        .send()
        .await
        .expect("Subscribe")
        .subscription_arn()
        .expect("SubscriptionArn")
        .to_string();

    sns.publish()
        .topic_arn(&topic_arn)
        .message("fan-out from Rust")
        .send()
        .await
        .expect("Publish");

    // Delivery is asynchronous. Poll for up to about 10 seconds.
    let deadline = Instant::now() + Duration::from_secs(10);
    let body = loop {
        let received = sqs
            .receive_message()
            .queue_url(&queue_url)
            .max_number_of_messages(1)
            .wait_time_seconds(2)
            .send()
            .await
            .expect("ReceiveMessage");
        if let Some(message) = received.messages().first() {
            break message.body().unwrap_or_default().to_string();
        }
        assert!(Instant::now() < deadline, "no message arrived in the queue within 10 seconds");
    };
    assert_eq!(body, "fan-out from Rust");

    sns.unsubscribe()
        .subscription_arn(&subscription_arn)
        .send()
        .await
        .expect("Unsubscribe");
    sns.delete_topic().topic_arn(&topic_arn).send().await.expect("DeleteTopic");
    sqs.delete_queue().queue_url(&queue_url).send().await.expect("DeleteQueue");
}
