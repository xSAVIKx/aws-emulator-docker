// Tests with the AWS SDK for JavaScript v3 against one container.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from "node:fs";

import {
  CreateBucketCommand,
  DeleteBucketCommand,
  DeleteObjectCommand,
  GetObjectCommand,
  PutObjectCommand,
  S3Client,
} from "@aws-sdk/client-s3";
import {
  CreateQueueCommand,
  DeleteQueueCommand,
  ReceiveMessageCommand,
  SendMessageCommand,
  SQSClient,
} from "@aws-sdk/client-sqs";
import { GetCallerIdentityCommand, STSClient } from "@aws-sdk/client-sts";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { AwsEmulatorContainer, type StartedAwsEmulatorContainer } from "../src/index.js";
import { clientConfig, IMAGE, uniqueName } from "./helpers.js";

describe("AwsEmulatorContainer", () => {
  let container: StartedAwsEmulatorContainer;

  beforeAll(async () => {
    container = await new AwsEmulatorContainer(IMAGE).start();
  });

  afterAll(async () => {
    await container?.stop();
  });

  it("gives the endpoint, Region, credentials and CA", () => {
    expect(container.getEndpoint()).toMatch(/^https:\/\/.+:\d+$/);
    expect(container.getRegion()).toBe("us-east-1");
    expect(container.getAccessKeyId()).not.toBe("");
    expect(container.getSecretAccessKey()).not.toBe("");
    expect(container.getCaCertificate()).toContain("-----BEGIN CERTIFICATE-----");

    const env = container.getEnvironment();
    expect(env).toMatchObject({
      AWS_ENDPOINT_URL: container.getEndpoint(),
      AWS_ACCESS_KEY_ID: container.getAccessKeyId(),
      AWS_SECRET_ACCESS_KEY: container.getSecretAccessKey(),
      AWS_REGION: "us-east-1",
      AWS_DEFAULT_REGION: "us-east-1",
      AWS_EC2_METADATA_DISABLED: "true",
    });
    expect(readFileSync(env.AWS_CA_BUNDLE!, "utf8")).toBe(container.getCaCertificate());
  });

  it("STS GetCallerIdentity returns the emulator account", async () => {
    const sts = new STSClient(clientConfig(container));
    try {
      const identity = await sts.send(new GetCallerIdentityCommand({}));
      expect(identity.Account).toBe("000000000000");
    } finally {
      sts.destroy();
    }
  });

  it("S3 creates a bucket, puts and gets an object (path-style)", async () => {
    // Without forcePathStyle the bucket goes into the host name, which
    // does not resolve to the emulator.
    const s3 = new S3Client({ ...clientConfig(container), forcePathStyle: true });
    const bucket = uniqueName();
    const key = "greetings/hello.txt";
    try {
      await s3.send(new CreateBucketCommand({ Bucket: bucket }));
      await s3.send(new PutObjectCommand({ Bucket: bucket, Key: key, Body: "Hello", ContentType: "text/plain" }));
      const object = await s3.send(new GetObjectCommand({ Bucket: bucket, Key: key }));
      expect(await object.Body?.transformToString()).toBe("Hello");
    } finally {
      await s3.send(new DeleteObjectCommand({ Bucket: bucket, Key: key })).catch(() => {});
      await s3.send(new DeleteBucketCommand({ Bucket: bucket })).catch(() => {});
      s3.destroy();
    }
  });

  it("SQS creates a queue, sends and receives one message", async () => {
    // The emulator returns AWS-style queue URLs. useQueueUrlAsEndpoint:
    // false keeps every request on the emulator endpoint.
    const sqs = new SQSClient({ ...clientConfig(container), useQueueUrlAsEndpoint: false });
    let queueUrl: string | undefined;
    try {
      queueUrl = (await sqs.send(new CreateQueueCommand({ QueueName: uniqueName() }))).QueueUrl;
      expect(queueUrl).toBeDefined();
      await sqs.send(new SendMessageCommand({ QueueUrl: queueUrl, MessageBody: "order-42" }));
      const received = await sqs.send(
        new ReceiveMessageCommand({ QueueUrl: queueUrl, MaxNumberOfMessages: 1, WaitTimeSeconds: 5 }),
      );
      expect(received.Messages).toHaveLength(1);
      expect(received.Messages![0]!.Body).toBe("order-42");
    } finally {
      if (queueUrl) await sqs.send(new DeleteQueueCommand({ QueueUrl: queueUrl })).catch(() => {});
      sqs.destroy();
    }
  });

  it("the HTTPS agent trusts only the emulator CA", async () => {
    // A request with the default trust store must fail: the CA is not in it.
    const sts = new STSClient(container.getClientConfig());
    try {
      await expect(sts.send(new GetCallerIdentityCommand({}))).rejects.toThrow();
    } finally {
      sts.destroy();
    }
  });
});
