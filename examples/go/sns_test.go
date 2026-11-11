package example

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/require"
)

// TestFanOut creates a topic and a queue, allows the topic in the queue
// policy, subscribes the queue with raw delivery, publishes, and receives the
// message from the queue.
func TestFanOut(t *testing.T) {
	ctx := t.Context()
	cfg := loadConfig(t)
	snsClient := sns.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)
	name := uniqueName(t)

	topic, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{Name: aws.String(name)})
	require.NoError(t, err)
	topicArn := topic.TopicArn
	t.Cleanup(func() {
		_, _ = snsClient.DeleteTopic(cleanupContext(), &sns.DeleteTopicInput{TopicArn: topicArn})
	})

	queue, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name)})
	require.NoError(t, err)
	queueURL := queue.QueueUrl
	t.Cleanup(func() {
		_, _ = sqsClient.DeleteQueue(cleanupContext(), &sqs.DeleteQueueInput{QueueUrl: queueURL})
	})

	attrs, err := sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       queueURL,
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
	})
	require.NoError(t, err)
	queueArn := attrs.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]
	require.NotEmpty(t, queueArn)

	// The queue policy lets SNS send messages to the queue, but only for
	// messages from this topic.
	policy, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect":    "Allow",
			"Principal": map[string]string{"Service": "sns.amazonaws.com"},
			"Action":    "sqs:SendMessage",
			"Resource":  queueArn,
			"Condition": map[string]any{
				"ArnEquals": map[string]string{"aws:SourceArn": aws.ToString(topicArn)},
			},
		}},
	})
	require.NoError(t, err)
	_, err = sqsClient.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		QueueUrl:   queueURL,
		Attributes: map[string]string{string(sqstypes.QueueAttributeNamePolicy): string(policy)},
	})
	require.NoError(t, err)

	// With raw delivery, the queue gets the message body as it is, without the
	// SNS JSON envelope.
	sub, err := snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn:   topicArn,
		Protocol:   aws.String("sqs"),
		Endpoint:   aws.String(queueArn),
		Attributes: map[string]string{"RawMessageDelivery": "true"},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = snsClient.Unsubscribe(cleanupContext(), &sns.UnsubscribeInput{SubscriptionArn: sub.SubscriptionArn})
	})

	const body = `{"event":"order-shipped"}`
	_, err = snsClient.Publish(ctx, &sns.PublishInput{TopicArn: topicArn, Message: aws.String(body)})
	require.NoError(t, err)

	// SNS delivers to the queue asynchronously. Poll for up to about 10 seconds.
	var got []sqstypes.Message
	deadline := time.Now().Add(10 * time.Second)
	for len(got) == 0 && time.Now().Before(deadline) {
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            queueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     2,
		})
		require.NoError(t, err)
		got = out.Messages
	}
	require.Len(t, got, 1, "no message arrived in the queue")
	require.Equal(t, body, aws.ToString(got[0].Body))
}
