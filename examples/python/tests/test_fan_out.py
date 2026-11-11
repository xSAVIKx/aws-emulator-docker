"""Fan-out scenario: an SNS topic delivers to an SQS queue."""

import json
import time


def receive_one(sqs, queue_url, timeout_seconds=30):
    """Long-poll the queue until one message arrives or the time ends.

    SNS delivers asynchronously, so the first receive can be empty.
    """
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        response = sqs.receive_message(
            QueueUrl=queue_url, MaxNumberOfMessages=1, WaitTimeSeconds=10
        )
        if response.get("Messages"):
            return response["Messages"][0]
    raise AssertionError(f"no message arrived in {queue_url} after {timeout_seconds} s")


def test_topic_delivers_to_queue(sns, sqs, unique_name):
    topic_arn = sns.create_topic(Name=unique_name)["TopicArn"]
    queue_url = sqs.create_queue(QueueName=unique_name)["QueueUrl"]
    try:
        queue_arn = sqs.get_queue_attributes(
            QueueUrl=queue_url, AttributeNames=["QueueArn"]
        )["Attributes"]["QueueArn"]

        # The queue policy lets this topic, and only this topic, send to the queue.
        policy = {
            "Version": "2012-10-17",
            "Statement": [
                {
                    "Effect": "Allow",
                    "Principal": {"Service": "sns.amazonaws.com"},
                    "Action": "sqs:SendMessage",
                    "Resource": queue_arn,
                    "Condition": {"ArnEquals": {"aws:SourceArn": topic_arn}},
                }
            ],
        }
        sqs.set_queue_attributes(QueueUrl=queue_url, Attributes={"Policy": json.dumps(policy)})

        # Raw delivery puts the published text in the SQS body as it is,
        # without the SNS JSON envelope.
        subscription_arn = sns.subscribe(
            TopicArn=topic_arn,
            Protocol="sqs",
            Endpoint=queue_arn,
            Attributes={"RawMessageDelivery": "true"},
            ReturnSubscriptionArn=True,
        )["SubscriptionArn"]

        sns.publish(TopicArn=topic_arn, Message="hello fan-out")

        message = receive_one(sqs, queue_url)
        assert message["Body"] == "hello fan-out"

        sqs.delete_message(QueueUrl=queue_url, ReceiptHandle=message["ReceiptHandle"])
        sns.unsubscribe(SubscriptionArn=subscription_arn)
    finally:
        sqs.delete_queue(QueueUrl=queue_url)
        sns.delete_topic(TopicArn=topic_arn)
