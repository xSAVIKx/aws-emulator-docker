# C# (.NET) example

This example tests the AWS Emulator with xUnit v3 and the AWS SDK for .NET v4.
It has one test for each scenario in [`examples/README.md`](../README.md).

## Prerequisites

- Docker.
- The .NET SDK in [`global.json`](global.json) (.NET 10, or a later feature band of it).

## Run

From the repository root:

```sh
examples/with-emulator.sh examples/dotnet/run-tests.sh
```

The script starts a fresh emulator, runs `dotnet test`, and stops the emulator.

## Files

| File | What it contains |
|---|---|
| `global.json` | The .NET SDK version. |
| `Directory.Packages.props` | All package versions (central package management). |
| `AwsEmulator.Examples.csproj` | The test project. |
| `EmulatorFixture.cs` | The endpoint guard, the AWS clients and the unique names. All tests share it. |
| `IdentityTests.cs` | STS: `GetCallerIdentity`. |
| `ObjectStorageTests.cs` | S3: bucket and object. |
| `KeyValueTableTests.cs` | DynamoDB: table, items, query. |
| `QueueTests.cs` | SQS: send, receive, delete. |
| `FanOutTests.cs` | SNS to SQS with raw delivery. |
| `SecretsTests.cs` | Secrets Manager: JSON secret. |
| `run-tests.sh` | Runs `dotnet test`. |

## How the clients connect

The code sets no endpoint, keys or certificate paths. The AWS SDK for .NET v4 reads
`AWS_ENDPOINT_URL`, so the code does not set `ServiceURL`. The SDK reads the credentials
from `AWS_SHARED_CREDENTIALS_FILE` and `AWS_CONFIG_FILE`, and the region from `AWS_REGION`.
The S3 client uses `ForcePathStyle = true`, because the endpoint is an IP address.

`EmulatorFixture` stops all tests if `AWS_ENDPOINT_URL` does not point to `127.0.0.1` or
`localhost`. This guard prevents a run against a real AWS account.

Each test uses a unique name such as `ex-net-3f9a1c2b`, and deletes its resources in a
`finally` block.

## TLS

The emulator uses HTTPS with a self-signed CA. On Linux, .NET uses OpenSSL, and OpenSSL
reads `SSL_CERT_FILE`. The start script sets `SSL_CERT_FILE` to a combined bundle: the
system CAs and the emulator CA. Thus `dotnet restore` can still download packages from
NuGet, and the tests trust the emulator. Without `SSL_CERT_FILE`, the requests fail with
`UntrustedRoot`.

## Notes

- The project uses the `xunit.v3.mtp-off` package. With this package, `dotnet test` uses
  VSTest, and the `--logger` option works. The default `xunit.v3` 4.x package uses
  Microsoft.Testing.Platform, which the .NET 10 `dotnet test` rejects in VSTest mode.
- `CreateTable` returns `CREATING`. The test polls `DescribeTable` until the table is `ACTIVE`.
- The emulator returns SQS queue URLs with the host `sqs.us-east-1.amazonaws.com`. The test
  passes the URL back unchanged. The AWS SDK for .NET v4 sends all SQS requests to
  `AWS_ENDPOINT_URL`, not to the host in the queue URL. No extra setting is necessary.
  The receive step proves this: it returns the message that the test sent.
- In SDK v4, an empty list in a response is `null`, not an empty list.
- Right after `DeleteSecret` with `ForceDeleteWithoutRecovery`, the emulator answers
  `GetSecretValue` with `InvalidRequestException`, not `ResourceNotFoundException`. The
  test checks only that the read fails (`AmazonSecretsManagerException`).
