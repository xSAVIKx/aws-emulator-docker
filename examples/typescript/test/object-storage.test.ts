// Scenario: Object storage (S3).
import {
  CreateBucketCommand,
  DeleteBucketCommand,
  DeleteObjectCommand,
  GetObjectCommand,
  ListObjectsV2Command,
  PutObjectCommand,
} from "@aws-sdk/client-s3";
import { afterAll, describe, expect, it } from "vitest";

import { s3Client, uniqueName } from "./aws.js";

describe("object storage (S3)", () => {
  // forcePathStyle is set in s3Client(). See aws.ts for the reason.
  const s3 = s3Client();
  const bucket = uniqueName(); // Lowercase, 14 characters: a valid bucket name.
  const key = "greetings/hello.txt";
  const body = "Hello from the AWS Emulator!";

  afterAll(async () => {
    // Remove what the test left if an assertion failed before the cleanup.
    // Errors are ignored here: the resources can be gone already.
    await s3.send(new DeleteObjectCommand({ Bucket: bucket, Key: key })).catch(() => {});
    await s3.send(new DeleteBucketCommand({ Bucket: bucket })).catch(() => {});
    s3.destroy();
  });

  it("creates a bucket, puts, lists and gets an object, and deletes both", async () => {
    await s3.send(new CreateBucketCommand({ Bucket: bucket }));

    await s3.send(
      new PutObjectCommand({ Bucket: bucket, Key: key, Body: body, ContentType: "text/plain" }),
    );

    const listing = await s3.send(new ListObjectsV2Command({ Bucket: bucket }));
    expect(listing.Contents?.map((o) => o.Key)).toEqual([key]);

    const object = await s3.send(new GetObjectCommand({ Bucket: bucket, Key: key }));
    // Body is a stream. transformToString() reads all of it.
    expect(await object.Body?.transformToString()).toBe(body);

    await s3.send(new DeleteObjectCommand({ Bucket: bucket, Key: key }));
    await s3.send(new DeleteBucketCommand({ Bucket: bucket }));

    const after = await s3.send(new ListObjectsV2Command({ Bucket: bucket })).catch((e: unknown) => e);
    // The bucket is gone, so the list call fails with NoSuchBucket.
    expect(after).toBeInstanceOf(Error);
    expect((after as Error).name).toBe("NoSuchBucket");
  });
});
