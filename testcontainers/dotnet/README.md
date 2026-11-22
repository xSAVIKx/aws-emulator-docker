<!--
Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
SPDX-License-Identifier: Apache-2.0
-->

# AwsEmulator.Testcontainers (.NET)

This package is a [Testcontainers for .NET](https://dotnet.testcontainers.org/) module
for the AWS Emulator Docker image. The module starts the image with the TLS front end.
It gives you the HTTPS endpoint, the credentials and the CA certificate.

The package does not depend on the AWS SDK. It gives an `HttpMessageHandler` that
trusts only the emulator CA. You give that handler to the AWS SDK.

The package supports .NET 8 and later (`net8.0` and `net10.0` assets).

## Install

The package is not on NuGet yet. Use a project reference to the source.

1. Clone the repository:

   ```sh
   git clone https://github.com/xSAVIKx/aws-emulator-docker.git
   ```

2. Add a project reference to your test project:

   ```sh
   dotnet add reference aws-emulator-docker/testcontainers/dotnet/src/AwsEmulator.Testcontainers/AwsEmulator.Testcontainers.csproj
   ```

You can also make a local package with `dotnet pack` in `src/AwsEmulator.Testcontainers`.

## Usage with the AWS SDK for .NET

```csharp
using Amazon.Runtime;
using Amazon.S3;
using AwsEmulator.Testcontainers;

await using var emulator = new AwsEmulatorBuilder("ghcr.io/xsavikx/aws-emulator:latest").Build();
await emulator.StartAsync();

var credentials = new BasicAWSCredentials(emulator.AccessKeyId, emulator.SecretAccessKey);
var config = new AmazonS3Config
{
    ServiceURL = emulator.GetEndpointUrl(),
    AuthenticationRegion = AwsEmulatorContainer.Region,
    ForcePathStyle = true,
    HttpClientFactory = new EmulatorHttpClientFactory(emulator),
};
using var s3 = new AmazonS3Client(credentials, config);
await s3.PutBucketAsync("my-bucket");

// The factory gives the SDK an HttpClient that trusts only the emulator CA.
sealed class EmulatorHttpClientFactory(AwsEmulatorContainer emulator) : HttpClientFactory
{
    public override HttpClient CreateHttpClient(IClientConfig clientConfig) =>
        new(emulator.CreateHttpMessageHandler(), disposeHandler: true);
}
```

Use the same `ServiceURL`, `AuthenticationRegion` and `HttpClientFactory` for the other
clients, for example `AmazonSQSConfig` and `AmazonSecurityTokenServiceConfig`.
`ForcePathStyle = true` is necessary for S3, because the endpoint host can be an IP
address.

For a child process, use `GetEnvironmentVariables()`. It returns `AWS_ENDPOINT_URL`,
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `AWS_DEFAULT_REGION`,
`AWS_CA_BUNDLE` and `AWS_EC2_METADATA_DISABLED=true`.

## API

| Member | Description |
|---|---|
| `new AwsEmulatorBuilder()` | Uses the image `ghcr.io/xsavikx/aws-emulator:latest`. |
| `new AwsEmulatorBuilder(image)` | Uses a different image. |
| `WithTlsNames(params string[])` | Adds host names or IP addresses to the certificate. |
| `WithTelemetry(bool)` | Turns the emulator telemetry on. The default is off. |
| `GetEndpointUrl()` | Returns `https://<host>:<mapped port>`. `GetConnectionString()` returns the same value. |
| `AwsEmulatorContainer.Region` | `us-east-1`. |
| `AwsEmulatorContainer.AccountId` | `000000000000`. |
| `AccessKeyId`, `SecretAccessKey` | The credentials from the container. |
| `CaCertificatePem`, `CaCertificate` | The CA certificate as PEM text and as `X509Certificate2`. |
| `WriteCaCertificateToTempFile()` | Writes the CA to a temporary file and returns the path. Disposal of the container deletes the file. |
| `GetEnvironmentVariables()` | Returns the variables for a child process or an SDK. |
| `CreateHttpMessageHandler()` | Returns a `SocketsHttpHandler` that trusts only the CA. |
| `CreateHttpClient()` | Returns an `HttpClient` with that handler and the endpoint as the base address. |
| `ValidateServerCertificate(...)` | A `RemoteCertificateValidationCallback` that trusts only the CA. |
| `AwsEmulatorTls` | The same TLS helpers for a CA certificate that you supply. |

The container properties are available after `StartAsync`. Before the start, they throw
`InvalidOperationException`.

## How the module starts the container

- The command is `-p 9297`. The module maps port 9297 to a random host port.
- The module sets `EMULATOR_TELEMETRY=off`, unless you call `WithTelemetry(true)`.
- The module sets `EMULATOR_TLS_NAMES` to these names:
  - The Testcontainers host, if the module can know it before the start.
  - All network aliases of the container.
  - The names from `WithTlsNames`.
- The module waits for the log line `TLS front end ready`. The time limit is 120 seconds.
- After the start, the module copies `emulator_credentials` and `tls-ca.pem` from
  `/home/emulator/.aws/emulator/9297/`. It uses the Docker archive API, not exec.

## Run the tests

```sh
AWS_EMULATOR_IMAGE=aws-emulator:tls-al ./run-tests.sh
```

The script uses `dotnet` if it is on the `PATH`. Otherwise it runs the tests in the
`mcr.microsoft.com/dotnet/sdk:11.0` container with the Docker socket. The script finds
rootless Docker and then sets `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE`. The tests need the
.NET SDK in `global.json`.

## Limits

- The image `ghcr.io/xsavikx/aws-emulator:latest` must have the TLS front end. Older
  images do not write `tls-ca.pem`, and the start fails after 120 seconds.
- When the tests run in a container and Docker uses a unix socket, Testcontainers uses
  the gateway of the container network as the host. The module cannot know the gateway
  before the start, so the gateway is not in the certificate. In this case, set
  `TESTCONTAINERS_HOST_OVERRIDE`, or add the gateway with `WithTlsNames`.
- With a remote Docker host, the module adds the host name of `DOCKER_HOST`. If you
  connect to the host with a different name, add that name with `WithTlsNames`.
- The emulator uses one region, `us-east-1`, and one account, `000000000000`.
