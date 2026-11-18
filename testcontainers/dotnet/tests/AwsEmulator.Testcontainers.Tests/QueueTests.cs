// SQS: create a queue, send a message, receive one message.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using Amazon.SQS.Model;

namespace AwsEmulator.Testcontainers.Tests;

public class QueueTests(EmulatorFixture aws)
{
    [Fact]
    public async Task Queue_SendAndReceive()
    {
        var ct = TestContext.Current.CancellationToken;

        var queueUrl = (await aws.Sqs.CreateQueueAsync(new CreateQueueRequest { QueueName = EmulatorFixture.UniqueName() }, ct)).QueueUrl;
        await aws.Sqs.SendMessageAsync(new SendMessageRequest { QueueUrl = queueUrl, MessageBody = "hello" }, ct);

        var received = await aws.Sqs.ReceiveMessageAsync(new ReceiveMessageRequest
        {
            QueueUrl = queueUrl,
            MaxNumberOfMessages = 1,
            WaitTimeSeconds = 5,
        }, ct);

        // In SDK v4, an empty response has a null list.
        var message = Assert.Single(received.Messages ?? []);
        Assert.Equal("hello", message.Body);
    }
}
