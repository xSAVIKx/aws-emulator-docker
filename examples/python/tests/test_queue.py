"""Queue scenario: SQS send, receive and delete."""


def test_send_receive_delete_message(sqs, unique_name):
    # The emulator returns a URL like
    # https://sqs.us-east-1.amazonaws.com/000000000000/<name>. Pass it back
    # unchanged: Boto3 sends the request to AWS_ENDPOINT_URL, not to that host.
    queue_url = sqs.create_queue(QueueName=unique_name)["QueueUrl"]
    try:
        sqs.send_message(QueueUrl=queue_url, MessageBody="hello queue")

        response = sqs.receive_message(
            QueueUrl=queue_url, MaxNumberOfMessages=1, WaitTimeSeconds=5
        )
        messages = response.get("Messages", [])
        assert len(messages) == 1
        assert messages[0]["Body"] == "hello queue"

        # A received message stays in the queue until you delete it
        # with its receipt handle.
        sqs.delete_message(QueueUrl=queue_url, ReceiptHandle=messages[0]["ReceiptHandle"])
    finally:
        sqs.delete_queue(QueueUrl=queue_url)
