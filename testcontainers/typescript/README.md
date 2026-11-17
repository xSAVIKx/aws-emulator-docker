<!--
Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
SPDX-License-Identifier: Apache-2.0
-->

# testcontainers-aws-emulator (TypeScript)

This package is a [Testcontainers for Node.js](https://node.testcontainers.org/)
module for the AWS Emulator Docker image. The module starts the image with the
TLS front end on. Your tests then call the emulator over HTTPS with a
certificate that a test CA signs.

The package is ESM only. It includes TypeScript type declarations. It does not
depend on the AWS SDK.

## Requirements

- Node.js 22.22 or later.
- Docker, or a container runtime that Testcontainers supports.
- An image that has the TLS front end. The image must write the line
  `TLS front end ready` to its log.

## Install

The package is not on the npm registry yet. Install it from the source.

1. Clone the repository:

   ```sh
   git clone https://github.com/xSAVIKx/aws-emulator-docker.git
   ```

2. Build a package file:

   ```sh
   cd aws-emulator-docker/testcontainers/typescript
   npm ci
   npm pack
   ```

3. Install the package file in your project:

   ```sh
   npm install --save-dev /path/to/aws-emulator-docker/testcontainers/typescript/testcontainers-aws-emulator-0.1.0.tgz
   ```

`npm ci` runs the `prepare` script. This script compiles `src/` to `dist/`.

## Usage

The example uses the AWS SDK for JavaScript v3 and Vitest. Install the SDK
clients and `@smithy/node-http-handler` in your project.

```ts
import { CreateBucketCommand, S3Client } from "@aws-sdk/client-s3";
import { SQSClient } from "@aws-sdk/client-sqs";
import { NodeHttpHandler } from "@smithy/node-http-handler";
import { AwsEmulatorContainer, type StartedAwsEmulatorContainer } from "testcontainers-aws-emulator";
import { afterAll, beforeAll, it } from "vitest";

let emulator: StartedAwsEmulatorContainer;

beforeAll(async () => {
  emulator = await new AwsEmulatorContainer().start();
}, 180_000);

afterAll(async () => {
  await emulator?.stop();
});

function clientConfig() {
  return {
    // endpoint, region and credentials
    ...emulator.getClientConfig(),
    // An HTTPS agent that trusts only the CA of the emulator.
    requestHandler: new NodeHttpHandler({ httpsAgent: emulator.getHttpsAgent() }),
  };
}

it("creates a bucket", async () => {
  // Path-style addresses: the bucket name does not go into the host name.
  const s3 = new S3Client({ ...clientConfig(), forcePathStyle: true });
  await s3.send(new CreateBucketCommand({ Bucket: "my-bucket" }));
});

// The emulator returns AWS-style queue URLs. This option keeps all SQS
// requests on the emulator endpoint.
const sqs = () => new SQSClient({ ...clientConfig(), useQueueUrlAsEndpoint: false });
```

Set `forcePathStyle: true` for S3. Without it, the SDK puts the bucket name
in the host name, and that host does not resolve to the emulator.

Set `useQueueUrlAsEndpoint: false` for SQS. Without it, the SDK sends requests
to the host in the queue URL, which is real AWS.

### A child process or a tool that reads the environment

`getEnvironment()` returns these variables:

| Variable | Value |
| --- | --- |
| `AWS_ENDPOINT_URL` | The endpoint, for example `https://localhost:32768` |
| `AWS_ACCESS_KEY_ID` | The access key ID |
| `AWS_SECRET_ACCESS_KEY` | The secret access key |
| `AWS_REGION` | `us-east-1` |
| `AWS_DEFAULT_REGION` | `us-east-1` |
| `AWS_CA_BUNDLE` | The path of a temporary file with the CA certificate |
| `AWS_EC2_METADATA_DISABLED` | `true` |

Give these variables to a child process, for example the AWS CLI:

```ts
import { execFileSync } from "node:child_process";

execFileSync("aws", ["s3", "ls"], { env: { ...process.env, ...emulator.getEnvironment() } });
```

A Node.js child process does not read `AWS_CA_BUNDLE` for TLS. Set
`NODE_EXTRA_CA_CERTS` to `emulator.getCaCertificatePath()` for such a process.

### Calls from other containers

Put the emulator on a Testcontainers network and give it an alias. The alias
goes into the server certificate.

```ts
import { Network } from "testcontainers";

const network = await new Network().start();
const emulator = await new AwsEmulatorContainer()
  .withNetwork(network)
  .withNetworkAliases("aws")
  .start();
// Other containers on the network use https://aws:9297.
// Copy emulator.getCaCertificate() into them as the trusted CA.
```

## API

### `new AwsEmulatorContainer(image?)`

The default image is `ghcr.io/xsavikx/aws-emulator:latest`. The class extends
`GenericContainer`, so all its methods are available.

| Method | Effect |
| --- | --- |
| `withTlsNames(...names)` | Adds host names or IP addresses to the server certificate. A name cannot be empty, and it cannot contain a comma or a space. |
| `withTelemetry(enabled)` | Turns the telemetry of the emulator on or off. The default is off (`EMULATOR_TELEMETRY=off`). |
| `start()` | Starts the container and returns a `StartedAwsEmulatorContainer`. |

The server certificate always has these names: `localhost`, `*.localhost`,
`000000000000.localhost`, `127.0.0.1`, `::1`, the host name and the IP
addresses of the container. The module adds the Testcontainers host, the
network aliases, and the names from `withTlsNames()`.

### `StartedAwsEmulatorContainer`

The class extends `AbstractStartedContainer`.

| Method | Return value |
| --- | --- |
| `getEndpoint()` | `https://<host>:<mapped port>` |
| `getRegion()` | `us-east-1` |
| `getAccessKeyId()` | The access key ID |
| `getSecretAccessKey()` | The secret access key |
| `getCredentials()` | `{ accessKeyId, secretAccessKey }` |
| `getClientConfig()` | `{ endpoint, region, credentials }` for an SDK client |
| `getCaCertificate()` | The CA certificate (PEM text) |
| `getCaCertificatePath()` | The path of a temporary file with the CA certificate. `stop()` removes the file. |
| `getHttpsAgent(options?)` | A `node:https` `Agent` that trusts only the CA |
| `getEnvironment()` | The environment variables in the table above |

The module also exports the constants `DEFAULT_IMAGE`, `EMULATOR_PORT`
(`9297`), `REGION` (`us-east-1`) and `ACCOUNT_ID` (`000000000000`).

## How the module starts the container

1. The module sets `EMULATOR_TLS_NAMES` and `EMULATOR_TELEMETRY=off`.
2. The module starts the image with the arguments `-p 9297`.
3. The module waits for the log line `TLS front end ready`. The timeout is 120 s.
4. The module copies two files from the container with the Docker archive API:
   `emulator_credentials` and `tls-ca.pem`.

The module does not use the Docker health check. Its first run is after 10 s.
The module does not use `exec`. The distroless image has no shell.

## Limits

- The emulator requires SigV4 signatures with the credentials of the container.
  Other credentials do not work.
- The account is always `000000000000`. The Region is always `us-east-1`.
- The image `ghcr.io/xsavikx/aws-emulator:latest` must have the TLS front end.
  An image without it never writes `TLS front end ready`, and the start fails
  after 120 s.
- The module sets `EMULATOR_TLS_NAMES` and `EMULATOR_TELEMETRY` itself. Values
  that you set with `withEnvironment()` for these variables have no effect.
- `getHttpsAgent()` trusts only the emulator CA. Do not use this agent for
  calls to other HTTPS servers.

## Tests

`run-tests.sh` installs the dependencies, builds the library, checks the
types, and runs the Vitest tests. The tests start real containers.

```sh
AWS_EMULATOR_IMAGE=aws-emulator:tls-al ./run-tests.sh
```

`AWS_EMULATOR_IMAGE` selects the image for the tests. The default is
`ghcr.io/xsavikx/aws-emulator:latest`. The library does not read this variable.

## License

Apache-2.0. See the `LICENSE` file in the repository root.
