# TypeScript (Node.js) example

This example runs integration tests against the AWS Emulator. It uses
[Vitest](https://vitest.dev/) and the
[AWS SDK for JavaScript v3](https://github.com/aws/aws-sdk-js-v3). The tests cover
the six scenarios in the [examples README](../README.md#scenarios): STS, S3,
DynamoDB, SQS, SNS to SQS fan-out, and Secrets Manager.

## Prerequisites

- Docker.
- Node.js 24 (the version in `.nvmrc`) and npm.

## Run the tests

From the repository root, run:

```sh
examples/with-emulator.sh examples/typescript/run-tests.sh
```

The script starts a fresh emulator, runs the tests, and stops the emulator.
`run-tests.sh` runs `npm ci`, then `tsc --noEmit`, then `vitest run` (the `npm test`
script). Extra arguments go to Vitest, for example
`examples/typescript/run-tests.sh test/queue.test.ts`.

## Files

| File | Purpose |
|---|---|
| `run-tests.sh` | Installs the pinned packages, checks the types, runs the tests. |
| `package.json`, `package-lock.json` | Exact package versions. |
| `tsconfig.json` | Strict TypeScript settings for ESM. |
| `vitest.config.ts` | Test files, the endpoint guard, and a 30 s test timeout. |
| `test/endpoint-guard.ts` | Stops the run unless the endpoint is local. |
| `test/aws.ts` | Unique names and the S3 and SQS client settings. |
| `test/*.test.ts` | One test file for each scenario. |

## Environment

The code contains no endpoint, keys, or certificate paths. The SDK reads these
variables, which the emulator start script sets:

| Variable | Use |
|---|---|
| `AWS_ENDPOINT_URL` | The emulator endpoint. The SDK sends all requests to it. |
| `AWS_SHARED_CREDENTIALS_FILE`, `AWS_CONFIG_FILE` | The credentials and the configuration that the emulator writes. |
| `AWS_REGION` | The Region, `us-east-1`. |
| `NODE_EXTRA_CA_CERTS` | Node trusts the emulator CA. Node reads this variable only at process start. |

The endpoint guard stops the run if the host in `AWS_ENDPOINT_URL` is not
`127.0.0.1` or `localhost`. This prevents a run against a real AWS account.

## Two client settings

**S3: `forcePathStyle: true`.** Without this setting, the SDK puts the bucket name in
the host name, for example `my-bucket.127.0.0.1` or `my-bucket.localhost`. That host
does not resolve to the emulator, so the requests fail.

**SQS: `useQueueUrlAsEndpoint: false`.** The emulator returns AWS-style queue URLs,
for example `https://sqs.us-east-1.amazonaws.com/000000000000/<name>`. By default the
SDK sends a request to the host in the queue URL. The SDK does not treat
`AWS_ENDPOINT_URL` as an explicit endpoint for this check. Without the setting, the
requests go to real AWS and fail with `InvalidClientTokenId`. With the setting, all
requests go to the emulator.
