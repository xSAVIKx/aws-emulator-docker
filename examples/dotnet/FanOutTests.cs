// Scenario "Fan-out" (SNS + SQS): create a topic and a queue, allow the topic in the
// queue policy, subscribe with raw delivery, publish, receive the message.

using System.Text.Json;
using Amazon.SimpleNotificationService.Model;
using Amazon.SQS;
using Amazon.SQS.Model;

namespace AwsEmulator.Examples;

public class FanOutTests(EmulatorFixture aws)
{
    [Fact]
    public async Task Topic_DeliversToQueue()
    {
        var ct = TestContext.Current.CancellationToken;
        var name = EmulatorFixture.UniqueName();

        var topicArn = (await aws.Sns.CreateTopicAsync(new CreateTopicRequest { Name = name }, ct)).TopicArn;
        string? queueUrl = null;
        try
        {
            queueUrl = (await aws.Sqs.CreateQueueAsync(new CreateQueueRequest { QueueName = name }, ct)).QueueUrl;
            var queueArn = (await aws.Sqs.GetQueueAttributesAsync(new GetQueueAttributesRequest
            {
                QueueUrl = queueUrl,
                AttributeNames = [QueueAttributeName.QueueArn],
            }, ct)).QueueARN;

            // The queue policy lets this one topic (and no other) send messages to the queue.
            var policy = JsonSerializer.Serialize(new
            {
                Version = "2012-10-17",
                Statement = new[]
                {
                    new
                    {
                        Effect = "Allow",
                        Principal = new { Service = "sns.amazonaws.com" },
                        Action = "sqs:SendMessage",
                        Resource = queueArn,
                        Condition = new { ArnEquals = new Dictionary<string, string> { ["aws:SourceArn"] = topicArn } },
                    },
                },
            });
            await aws.Sqs.SetQueueAttributesAsync(new SetQueueAttributesRequest
            {
                QueueUrl = queueUrl,
                Attributes = new Dictionary<string, string> { [QueueAttributeName.Policy] = policy },
            }, ct);

            // Raw delivery puts the message text in the SQS body, without the SNS JSON envelope.
            await aws.Sns.SubscribeAsync(new SubscribeRequest
            {
                TopicArn = topicArn,
                Protocol = "sqs",
                Endpoint = queueArn,
                Attributes = new Dictionary<string, string> { ["RawMessageDelivery"] = "true" },
            }, ct);

            await aws.Sns.PublishAsync(new PublishRequest { TopicArn = topicArn, Message = "fan-out" }, ct);

            // SNS delivers the message asynchronously. Use long polling and retry for up to
            // 10 s in total. Each long poll waits at most 2 s and not past the deadline.
            Message? message = null;
            var deadline = DateTime.UtcNow.AddSeconds(10);
            while (message is null && DateTime.UtcNow < deadline)
            {
                var remaining = (int)(deadline - DateTime.UtcNow).TotalSeconds;
                var received = await aws.Sqs.ReceiveMessageAsync(new ReceiveMessageRequest
                {
                    QueueUrl = queueUrl,
                    MaxNumberOfMessages = 1,
                    WaitTimeSeconds = Math.Clamp(remaining, 1, 2),
                }, ct);
                // In SDK v4, an empty response has a null list, not an empty list.
                message = received.Messages?.FirstOrDefault();
            }

            Assert.NotNull(message);
            Assert.Equal("fan-out", message.Body);
        }
        finally
        {
            // Deleting the topic also deletes its subscriptions.
            await EmulatorFixture.CleanUpAsync("DeleteTopic", () =>
                aws.Sns.DeleteTopicAsync(new DeleteTopicRequest { TopicArn = topicArn }, CancellationToken.None));
            if (queueUrl is not null)
            {
                await EmulatorFixture.CleanUpAsync("DeleteQueue", () =>
                    aws.Sqs.DeleteQueueAsync(new DeleteQueueRequest { QueueUrl = queueUrl }, CancellationToken.None));
            }
        }
    }
}
