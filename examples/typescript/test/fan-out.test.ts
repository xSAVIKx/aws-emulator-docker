// Scenario: Fan-out (SNS + SQS).
//
// SNS publishes to a topic. SQS receives a copy through a subscription.
import {
  CreateTopicCommand,
  DeleteTopicCommand,
  PublishCommand,
  SNSClient,
  SubscribeCommand,
} from "@aws-sdk/client-sns";
import {
  CreateQueueCommand,
  DeleteMessageCommand,
  DeleteQueueCommand,
  GetQueueAttributesCommand,
  type Message,
  ReceiveMessageCommand,
  SetQueueAttributesCommand,
  type SQSClient,
} from "@aws-sdk/client-sqs";
import { afterAll, describe, expect, it } from "vitest";

import { sqsClient, uniqueName } from "./aws.js";

describe("fan-out (SNS + SQS)", () => {
  const sns = new SNSClient({});
  // useQueueUrlAsEndpoint is set in sqsClient(). See aws.ts for the reason.
  const sqs = sqsClient();
  let topicArn: string | undefined;
  let queueUrl: string | undefined;

  afterAll(async () => {
    // Deleting the topic also removes its subscriptions.
    if (topicArn) await sns.send(new DeleteTopicCommand({ TopicArn: topicArn })).catch(() => {});
    if (queueUrl) await sqs.send(new DeleteQueueCommand({ QueueUrl: queueUrl })).catch(() => {});
    sns.destroy();
    sqs.destroy();
  });

  it("delivers a published message to a subscribed queue", async () => {
    const name = uniqueName();

    topicArn = (await sns.send(new CreateTopicCommand({ Name: name }))).TopicArn;
    queueUrl = (await sqs.send(new CreateQueueCommand({ QueueName: name }))).QueueUrl;
    expect(topicArn).toBeDefined();
    expect(queueUrl).toBeDefined();

    const attributes = await sqs.send(
      new GetQueueAttributesCommand({ QueueUrl: queueUrl, AttributeNames: ["QueueArn"] }),
    );
    const queueArn = attributes.Attributes?.QueueArn;
    expect(queueArn).toBeDefined();

    // The queue policy must allow the topic to send messages to the queue.
    // Without it, SNS cannot deliver, as on AWS.
    const policy = {
      Version: "2012-10-17",
      Statement: [
        {
          Effect: "Allow",
          Principal: { Service: "sns.amazonaws.com" },
          Action: "sqs:SendMessage",
          Resource: queueArn,
          Condition: { ArnEquals: { "aws:SourceArn": topicArn } },
        },
      ],
    };
    await sqs.send(
      new SetQueueAttributesCommand({ QueueUrl: queueUrl, Attributes: { Policy: JSON.stringify(policy) } }),
    );

    // RawMessageDelivery=true puts the message text in the SQS body as it is.
    // Without it, the body is an SNS JSON envelope.
    await sns.send(
      new SubscribeCommand({
        TopicArn: topicArn,
        Protocol: "sqs",
        Endpoint: queueArn,
        Attributes: { RawMessageDelivery: "true" },
        ReturnSubscriptionArn: true,
      }),
    );

    const text = JSON.stringify({ event: "order-created", id: 42 });
    await sns.send(new PublishCommand({ TopicArn: topicArn, Message: text }));

    // Delivery is asynchronous. Poll for up to 10 s in total.
    const message = await receiveOne(sqs, queueUrl!, 10_000);
    expect(message.Body).toBe(text);
    await sqs.send(new DeleteMessageCommand({ QueueUrl: queueUrl, ReceiptHandle: message.ReceiptHandle }));
  });
});

/** Receive one message with long polling. Retry until the deadline. */
async function receiveOne(sqs: SQSClient, queueUrl: string, timeoutMs: number): Promise<Message> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const waitSeconds = Math.max(1, Math.min(5, Math.floor((deadline - Date.now()) / 1000)));
    const { Messages } = await sqs.send(
      new ReceiveMessageCommand({ QueueUrl: queueUrl, MaxNumberOfMessages: 1, WaitTimeSeconds: waitSeconds }),
    );
    const first = Messages?.[0];
    if (first) return first;
  }
  throw new Error(`No message arrived in ${queueUrl} within ${timeoutMs} ms`);
}
