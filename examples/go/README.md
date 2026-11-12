# Go example

This example tests Go code against the AWS Emulator. It uses the standard `testing`
package, [testify](https://github.com/stretchr/testify) `require`, and the
[AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2). The tests cover the six
scenarios in the [examples README](../README.md#scenarios).

## Prerequisites

- Docker.
- Go 1.25 or later. The `go` directive in `go.mod` sets the minimum version. The
  `toolchain` directive selects the newest Go 1.25 patch release when your Go is older.

## Run

From the repository root, run:

```sh
examples/with-emulator.sh examples/go/run-tests.sh
```

The script starts a fresh emulator, runs `go vet ./...` and `go test -v -count=1 ./...`,
and stops the emulator.

## Files

| File | What it contains |
|---|---|
| `main_test.go` | `TestMain` with the endpoint guard, and the shared helpers `loadConfig` and `uniqueName`. |
| `sts_test.go` | Identity: `GetCallerIdentity` returns account `000000000000`. |
| `s3_test.go` | Object storage: a bucket and an object. |
| `dynamodb_test.go` | Key-value table: a table with a partition key and a sort key. |
| `sqs_test.go` | Queue: send, receive and delete a message. |
| `sns_test.go` | Fan-out: an SNS topic sends to an SQS queue with raw delivery. |
| `secretsmanager_test.go` | Secrets: a JSON secret. |
| `run-tests.sh` | Runs `go vet` and `go test`. |
| `go.mod`, `go.sum` | The module and the pinned dependency versions. |

## Environment variables

The code contains no endpoint, keys or certificate paths. `config.LoadDefaultConfig`
reads these variables, which `with-emulator.sh` sets:

| Variable | Use |
|---|---|
| `AWS_ENDPOINT_URL` | The endpoint for all service clients. `TestMain` stops the run if the host is not `127.0.0.1` or `localhost`. |
| `AWS_CA_BUNDLE` | The emulator CA certificate for HTTPS. |
| `AWS_REGION` | The AWS Region, `us-east-1`. |
| `AWS_SHARED_CREDENTIALS_FILE`, `AWS_CONFIG_FILE` | The credentials and the configuration that the emulator writes. |

## Notes

- The S3 client uses path-style addresses (`o.UsePathStyle = true`), because the endpoint
  is an IP address.
- Each test makes names like `ex-go-3f9a1c2b`, so runs do not collide. Each test removes
  its resources with `t.Cleanup`, also when it fails.
- `CreateTable` returns the status `CREATING`. The test uses
  `dynamodb.NewTableExistsWaiter` to wait for `ACTIVE`.
- SQS returns queue URLs like `https://sqs.us-east-1.amazonaws.com/000000000000/<name>`.
  The test passes the URL back unchanged. The Go SDK does not use the host of the queue
  URL as the endpoint. It sends every SQS request to `AWS_ENDPOINT_URL`, so you need no
  extra setting. (The request log shows `Host: 127.0.0.1:<port>` for all SQS calls.)
- After a forced `DeleteSecret`, `GetSecretValue` fails. The error can be
  `ResourceNotFoundException` or `InvalidRequestException`. Real AWS can also return
  `InvalidRequestException` right after the delete, because the deletion is asynchronous.
  The test checks only that the read fails.
- The emulator does not send the `X-Amz-Crc32` header on DynamoDB responses. The test
  sets `DisableValidateResponseChecksum`. Without it, the SDK logs a
  `failed to close HTTP response body` warning for each call.
- The emulator does not send a checksum header on S3 `GetObject`. The test sets
  `DisableLogOutputChecksumValidationSkipped` to stop the SDK message about it.
