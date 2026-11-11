// Scenario "Queue" (SQS): create a queue, send a message, receive and delete it,
// delete the queue.

using Amazon.SQS.Model;

namespace AwsEmulator.Examples;

public class QueueTests(EmulatorFixture aws)
{
    [Fact]
    public async Task Queue_SendReceiveDelete()
    {
        var ct = TestContext.Current.CancellationToken;
        var name = EmulatorFixture.UniqueName();

        // The emulator returns a queue URL with an AWS host name, for example
        // https://sqs.us-east-1.amazonaws.com/000000000000/<name>. We pass the URL back
        // unchanged. The SDK sends it as a request parameter and sends every request to
        // AWS_ENDPOINT_URL, so the host name in the URL does not matter. (Some other SDKs
        // send SQS requests to the QueueUrl host. The AWS SDK for .NET v4 does not: the
        // receive below returns the message that we sent, which only the emulator can do.)
        var queueUrl = (await aws.Sqs.CreateQueueAsync(new CreateQueueRequest { QueueName = name }, ct)).QueueUrl;
        try
        {
            await aws.Sqs.SendMessageAsync(new SendMessageRequest
            {
                QueueUrl = queueUrl,
                MessageBody = "hello",
            }, ct);

            var received = await aws.Sqs.ReceiveMessageAsync(new ReceiveMessageRequest
            {
                QueueUrl = queueUrl,
                MaxNumberOfMessages = 1,
                WaitTimeSeconds = 5,
            }, ct);
            // In SDK v4, an empty response has a null list, so use an empty list instead.
            var message = Assert.Single(received.Messages ?? []);
            Assert.Equal("hello", message.Body);

            // Delete the message, or SQS delivers it again after the visibility timeout.
            await aws.Sqs.DeleteMessageAsync(new DeleteMessageRequest
            {
                QueueUrl = queueUrl,
                ReceiptHandle = message.ReceiptHandle,
            }, ct);
        }
        finally
        {
            await aws.Sqs.DeleteQueueAsync(new DeleteQueueRequest { QueueUrl = queueUrl }, CancellationToken.None);
        }
    }
}
