package example

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/require"
)

// TestQueue creates a queue, sends a message, receives and deletes it, and
// deletes the queue.
func TestQueue(t *testing.T) {
	ctx := t.Context()
	client := sqs.NewFromConfig(loadConfig(t))

	created, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(uniqueName(t))})
	require.NoError(t, err)
	// The emulator returns an AWS-style URL, for example
	// https://sqs.us-east-1.amazonaws.com/000000000000/<name>. The SDK still
	// sends the request to AWS_ENDPOINT_URL, so pass the URL back unchanged.
	queueURL := created.QueueUrl
	t.Cleanup(func() {
		_, _ = client.DeleteQueue(cleanupContext(), &sqs.DeleteQueueInput{QueueUrl: queueURL})
	})

	const body = "order-created"
	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{QueueUrl: queueURL, MessageBody: aws.String(body)})
	require.NoError(t, err)

	// Long polling: the call waits up to 5 seconds for a message.
	got, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            queueURL,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     5,
	})
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)
	require.Equal(t, body, aws.ToString(got.Messages[0].Body))

	// A received message stays in the queue until you delete it.
	_, err = client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      queueURL,
		ReceiptHandle: got.Messages[0].ReceiptHandle,
	})
	require.NoError(t, err)

	_, err = client.DeleteQueue(ctx, &sqs.DeleteQueueInput{QueueUrl: queueURL})
	require.NoError(t, err)
}
