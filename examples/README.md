# Examples

Integration tests in six languages that run against the AWS Emulator image. Each example
uses the most common testing framework of its language and the official AWS SDK. All
examples test the same scenarios, so you can compare them side by side. CI runs every
example on `linux/amd64` and `linux/arm64`
([examples workflow](../.github/workflows/examples.yml)).

| Language | Directory | Test framework | AWS SDK |
|---|---|---|---|
| Python | [`python/`](python/) | pytest | Boto3 |
| TypeScript (Node.js) | [`typescript/`](typescript/) | Vitest | AWS SDK for JavaScript v3 |
| Java | [`java/`](java/) | JUnit 5 + Maven | AWS SDK for Java 2.x |
| Go | [`go/`](go/) | `go test` | AWS SDK for Go v2 |
| C# (.NET) | [`dotnet/`](dotnet/) | xUnit | AWS SDK for .NET |
| Rust | [`rust/`](rust/) | `cargo test` + Tokio | AWS SDK for Rust |

## Scenarios

Every example has one test for each scenario:

| Scenario | Services | What the test does |
|---|---|---|
| Identity | STS | `GetCallerIdentity` returns account `000000000000`. |
| Object storage | S3 | Create a bucket, put an object, list and get it, delete both. |
| Key-value table | DynamoDB | Create a table, wait until it is `ACTIVE`, put two items, get one, query both, delete the table. |
| Queue | SQS | Create a queue, send a message, receive and delete it, delete the queue. |
| Fan-out | SNS + SQS | Create a topic and a queue, allow the topic in the queue policy, subscribe with raw delivery, publish, receive the message. |
| Secrets | Secrets Manager | Create a JSON secret, read it back, delete it. |

## Run an example on your computer

1. Install Docker and the toolchain of the language.
2. From the repository root, run the example with a fresh emulator:

   ```sh
   examples/with-emulator.sh examples/python/run-tests.sh
   ```

`with-emulator.sh` starts the image, waits until it is healthy, sets the environment
below, runs the command, and stops the emulator. Set `EMULATOR_PORT` to use another
port, and `EMULATOR_IMAGE` to test another image.

To keep one emulator for many runs:

```sh
eval "$(.github/actions/aws-emulator/start-emulator.sh)"
examples/python/run-tests.sh
.github/actions/aws-emulator/start-emulator.sh --stop
```

## How the examples connect

The examples contain no endpoint, keys or certificate paths. The SDKs read the standard
environment variables, which the start script sets:

| Variable | Value |
|---|---|
| `AWS_ENDPOINT_URL` | `https://127.0.0.1:<port>` |
| `AWS_SHARED_CREDENTIALS_FILE`, `AWS_CONFIG_FILE` | The files that the emulator writes |
| `AWS_CA_BUNDLE`, `NODE_EXTRA_CA_CERTS` | The emulator CA certificate |
| `SSL_CERT_FILE` | System CAs + emulator CA (Rust, .NET, Go fallback) |
| `JAVA_TOOL_OPTIONS` | A truststore with the JDK CAs + emulator CA |
| `AWS_REGION` | `us-east-1` |

Two rules apply in every language:

- S3 uses path-style addresses (`forcePathStyle`), because the endpoint is an IP address.
- SQS calls must go to the emulator. The emulator returns queue URLs with the real AWS
  host (`https://sqs.us-east-1.amazonaws.com/000000000000/<name>`). Some SDKs send the
  request to the host in the queue URL. The AWS SDK for JavaScript v3 does this, so the
  TypeScript example sets `useQueueUrlAsEndpoint: false`.
- Each test suite stops at once if `AWS_ENDPOINT_URL` does not point to `127.0.0.1` or
  `localhost`. This guard prevents a run against a real AWS account.

## Use the emulator in your own workflow

```yaml
steps:
  - uses: actions/checkout@v5
  - uses: xSAVIKx/run-aws-emulator@v1
  - run: aws sts get-caller-identity   # and then your own tests
```

The action is [Run AWS Emulator](https://github.com/marketplace/actions/run-aws-emulator) on the GitHub Marketplace
([source](https://github.com/xSAVIKx/run-aws-emulator)).
Pin the image to a build tag or a digest for reproducible runs.
