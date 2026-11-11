// Shared helpers for the tests.
//
// The clients get no endpoint, credentials or region in code. The SDK reads
// them from the environment that the emulator start script sets:
//   AWS_ENDPOINT_URL              -> the emulator endpoint (https://127.0.0.1:<port>)
//   AWS_SHARED_CREDENTIALS_FILE   -> the credentials that the emulator writes
//   AWS_CONFIG_FILE, AWS_REGION   -> the Region
//   NODE_EXTRA_CA_CERTS           -> Node trusts the emulator CA at process start
import { randomBytes } from "node:crypto";

import { S3Client } from "@aws-sdk/client-s3";
import { SQSClient } from "@aws-sdk/client-sqs";

/**
 * Return a resource name that is unique for this run, for example
 * "ex-ts-3f9a1c2b". The name uses only lowercase letters, digits and
 * hyphens, so it is valid for S3 buckets, DynamoDB tables, SQS queues,
 * SNS topics and secrets.
 */
export function uniqueName(prefix = "ex-ts"): string {
  return `${prefix}-${randomBytes(4).toString("hex")}`;
}

/**
 * Make an S3 client that uses path-style addresses
 * (https://127.0.0.1:<port>/<bucket>/<key>).
 *
 * Without forcePathStyle the SDK puts the bucket name in the host name
 * (<bucket>.127.0.0.1 or <bucket>.localhost). That host does not resolve to
 * the emulator, so the requests fail.
 */
export function s3Client(): S3Client {
  return new S3Client({ forcePathStyle: true });
}

/**
 * Make an SQS client that sends every request to AWS_ENDPOINT_URL.
 *
 * The emulator returns AWS-style queue URLs, for example
 * https://sqs.us-east-1.amazonaws.com/000000000000/<name>. By default the
 * JavaScript SDK sends a request to the host of the QueueUrl when that host
 * differs from the endpoint that it resolved. The SDK does not count
 * AWS_ENDPOINT_URL as an explicit endpoint here, so without this option the
 * requests go to real AWS and fail with InvalidClientTokenId.
 * useQueueUrlAsEndpoint: false keeps all requests on the emulator.
 */
export function sqsClient(): SQSClient {
  return new SQSClient({ useQueueUrlAsEndpoint: false });
}
