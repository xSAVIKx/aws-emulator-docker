// Scenario: Queue (SQS).
import {
  CreateQueueCommand,
  DeleteMessageCommand,
  DeleteQueueCommand,
  ReceiveMessageCommand,
  SendMessageCommand,
} from "@aws-sdk/client-sqs";
import { afterAll, describe, expect, it } from "vitest";

import { sqsClient, uniqueName } from "./aws.js";

describe("queue (SQS)", () => {
  // useQueueUrlAsEndpoint is set in sqsClient(). See aws.ts for the reason.
  const sqs = sqsClient();
  let queueUrl: string | undefined;

  afterAll(async () => {
    if (queueUrl) {
      await sqs.send(new DeleteQueueCommand({ QueueUrl: queueUrl })).catch(() => {});
    }
    sqs.destroy();
  });

  it("creates a queue, sends, receives and deletes a message, and deletes the queue", async () => {
    const created = await sqs.send(new CreateQueueCommand({ QueueName: uniqueName() }));
    // The emulator returns an AWS-style URL, for example
    // https://sqs.us-east-1.amazonaws.com/000000000000/<name>.
    // Pass it back unchanged. The client sends every request to AWS_ENDPOINT_URL.
    queueUrl = created.QueueUrl;
    expect(queueUrl).toBeDefined();

    await sqs.send(new SendMessageCommand({ QueueUrl: queueUrl, MessageBody: "order-42" }));

    // Long polling: wait up to 5 s for a message.
    const received = await sqs.send(
      new ReceiveMessageCommand({ QueueUrl: queueUrl, MaxNumberOfMessages: 1, WaitTimeSeconds: 5 }),
    );
    expect(received.Messages).toHaveLength(1);
    const message = received.Messages![0]!;
    expect(message.Body).toBe("order-42");

    // A received message stays in the queue until you delete it.
    await sqs.send(new DeleteMessageCommand({ QueueUrl: queueUrl, ReceiptHandle: message.ReceiptHandle }));

    await sqs.send(new DeleteQueueCommand({ QueueUrl: queueUrl }));
    queueUrl = undefined;
  });
});
