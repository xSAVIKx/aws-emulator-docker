# Java example

This example tests the AWS Emulator with JUnit 5, Maven and the AWS SDK for Java 2.x.
It has one test class for each scenario in the [examples overview](../README.md).

## Prerequisites

- Docker.
- JDK 17 or later. CI uses JDK 21.
- Maven 3.9 or later.

## Run the tests

From the repository root, run:

```sh
examples/with-emulator.sh examples/java/run-tests.sh
```

The script starts a fresh emulator, runs `mvn -B -ntp test`, and stops the emulator.

If an emulator runs already and your shell has its environment, run
`examples/java/run-tests.sh` directly.

## Files

| File | What it does |
|---|---|
| `pom.xml` | Imports the AWS SDK BOM and the JUnit BOM. Pins the plugin versions. |
| `run-tests.sh` | Runs `mvn -B -ntp test` in this directory. |
| `src/test/java/example/Emulator.java` | A JUnit extension. It stops the run if `AWS_ENDPOINT_URL` is not local. It also makes unique names. |
| `src/test/resources/junit-platform.properties` | Turns on JUnit extension auto-detection. |
| `src/test/resources/META-INF/services/org.junit.jupiter.api.extension.Extension` | Registers `Emulator` for every test class. |
| `IdentityTest.java` | STS `GetCallerIdentity` returns account `000000000000`. |
| `ObjectStorageTest.java` | S3: bucket and object, with path-style addresses. |
| `KeyValueTableTest.java` | DynamoDB: table, waiter, put, get, query. |
| `QueueTest.java` | SQS: send, receive, delete. |
| `FanOutTest.java` | SNS to SQS with a queue policy and raw delivery. |
| `SecretsTest.java` | Secrets Manager: a JSON secret. |

## How the tests connect

The tests contain no endpoint, keys or certificate paths. Each client comes from
`XxxClient.create()` or `XxxClient.builder()`. The SDK reads the environment:

- `AWS_ENDPOINT_URL` sets the endpoint for all services. SDK 2.21 and later read it.
- `AWS_SHARED_CREDENTIALS_FILE` and `AWS_CONFIG_FILE` give the credentials.
- `AWS_REGION` gives the Region.

S3 uses `forcePathStyle(true)`, because the endpoint is an IP address.

Each resource name is `ex-java-` and 8 random hex characters. Each test class deletes its
resources in an `@AfterAll` method, also when the test fails.

## TLS and the truststore

The emulator uses HTTPS with a self-signed CA. The JVM does not read `AWS_CA_BUNDLE`.
The start script sets `JAVA_TOOL_OPTIONS` to a truststore that contains the JDK CAs
and the emulator CA. Maven and the tests both use this truststore. Thus Maven can still
download from Maven Central, and the SDK trusts the emulator.

Do not use a truststore that contains the emulator CA only. The JVM then trusts no
public CA, and Maven fails on downloads with `PKIX path building failed`.

## Emulator behaviour that the tests handle

- `CreateTable` returns status `CREATING`. The test waits with `waitUntilTableExists`.
- `DeleteTable` returns status `DELETING`. A second `DeleteTable` then fails with
  `ResourceInUseException` ("Table is being deleted"). The test waits with
  `waitUntilTableNotExists` and the clean-up step does not delete the table again.
- The default DynamoDB waiter polls every 20 seconds. The test polls every second.
- `CreateQueue` returns a URL like `https://sqs.us-east-1.amazonaws.com/000000000000/<name>`.
  The tests give this URL back unchanged. The AWS SDK for Java 2.x sends every SQS request
  to `AWS_ENDPOINT_URL` and uses the queue URL as a parameter only. It does not send
  requests to the URL host. (The AWS SDK for JavaScript v3 does, unless you set
  `useQueueUrlAsEndpoint: false`.) The queue test shows this: the message that it receives
  can only come from the emulator.
- After `DeleteSecret` with `forceDeleteWithoutRecovery(true)`, `GetSecretValue` fails.
  The error can be `ResourceNotFoundException` or `InvalidRequestException`. Real AWS can
  also return `InvalidRequestException` right after the delete, because the deletion is
  asynchronous. The test checks only that the read fails.
- SNS delivers to SQS asynchronously. The fan-out test polls the queue for up to 10 seconds
  in total.
