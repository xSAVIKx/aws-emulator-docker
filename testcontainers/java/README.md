# Testcontainers module for Java

This module starts the AWS Emulator image in a container for your JUnit tests.
The container serves HTTPS with a certificate from its own CA.
The module gives you the endpoint, the credentials, the CA, and a TLS context that trusts only that CA.

- Maven coordinates: `io.github.xsavikx:testcontainers-aws-emulator`.
- Package: `io.github.xsavikx.testcontainers.awsemulator`.
- Class: `AwsEmulatorContainer`. It extends `GenericContainer<AwsEmulatorContainer>`.
- License: Apache-2.0.

## Requirements

- Docker.
- JDK 17 or later. The module has Java 17 bytecode.
- Maven 3.9 or later.
- Testcontainers for Java 2.x. The library depends on `org.testcontainers:testcontainers` only.
  It does not depend on the AWS SDK.

## Install

The package is not on Maven Central yet. Install it from the source.

1. Clone the repository:

   ```sh
   git clone https://github.com/xSAVIKx/aws-emulator-docker.git
   ```

2. Install the module into your local Maven repository:

   ```sh
   cd aws-emulator-docker/testcontainers/java
   mvn -B -ntp install -DskipTests
   ```

3. Add the dependency to your `pom.xml`:

   ```xml
   <dependency>
     <groupId>io.github.xsavikx</groupId>
     <artifactId>testcontainers-aws-emulator</artifactId>
     <version>0.1.0-SNAPSHOT</version>
     <scope>test</scope>
   </dependency>
   ```

## Usage with the AWS SDK for Java 2.x

The example uses the `sts` and `apache5-client` modules of the SDK.
The Apache 5 HTTP client gets the trust managers of the container.
Thus the client trusts the emulator CA, and the JVM truststore stays unchanged.

```java
import io.github.xsavikx.testcontainers.awsemulator.AwsEmulatorContainer;
import software.amazon.awssdk.auth.credentials.AwsBasicCredentials;
import software.amazon.awssdk.auth.credentials.StaticCredentialsProvider;
import software.amazon.awssdk.http.apache5.Apache5HttpClient;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.sts.StsClient;

try (AwsEmulatorContainer emulator = new AwsEmulatorContainer()) {
    emulator.start();

    try (StsClient sts = StsClient.builder()
            .endpointOverride(emulator.getEndpoint())
            .region(Region.of(emulator.getRegion()))
            .credentialsProvider(StaticCredentialsProvider.create(AwsBasicCredentials.create(
                    emulator.getAccessKeyId(), emulator.getSecretAccessKey())))
            .httpClientBuilder(Apache5HttpClient.builder()
                    .tlsTrustManagersProvider(emulator::getTrustManagers))
            .build()) {
        System.out.println(sts.getCallerIdentity().account()); // 000000000000
    }
}
```

For S3, set `forcePathStyle(true)` on the client builder.
The endpoint host is not a DNS name for each bucket.

With the older `ApacheHttpClient` or the URL connection client, use the same `tlsTrustManagersProvider` method.
For example, with the URL connection client, use `UrlConnectionHttpClient.builder().tlsTrustManagersProvider(emulator::getTrustManagers)`.
With the JDK `HttpClient`, use `HttpClient.newBuilder().sslContext(emulator.getSslContext())`.

For a child process, for example the AWS CLI, use `emulator.getEnvironmentVariables()`.
The map has `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`,
`AWS_DEFAULT_REGION`, `AWS_CA_BUNDLE` and `AWS_EC2_METADATA_DISABLED=true`.

## Options and methods

| Item | What it does |
|---|---|
| `new AwsEmulatorContainer()` | Uses the image `ghcr.io/xsavikx/aws-emulator:latest`. |
| `new AwsEmulatorContainer("<image>")` | Uses a different image. A `DockerImageName` is also accepted. |
| `withTlsNames(String...)` | Adds host names or IP addresses to the certificate. A name must not contain a comma or a space. |
| `withTelemetry(boolean)` | Turns the telemetry of the emulator on or off. The default is off. |
| `getEndpoint()` | Returns the endpoint as a `URI`, for example `https://localhost:32768`. |
| `getRegion()` | Returns `us-east-1`. |
| `getAccountId()` | Returns `000000000000`. |
| `getAccessKeyId()`, `getSecretAccessKey()` | Return the credentials from the container. |
| `getCaCertificatePem()` | Returns the CA certificate as a PEM string. |
| `getCaCertificate()` | Returns the CA certificate as an `X509Certificate`. |
| `getCaCertificateFile()` | Writes the CA to a temporary file and returns its `Path`. The JVM deletes the file when it exits. |
| `getTrustStore()` | Returns a PKCS12 `KeyStore` that contains only the CA. |
| `getTrustManagers()` | Returns `TrustManager[]` that trust only the CA. |
| `getSslContext()` | Returns an `SSLContext` that trusts only the CA. |
| `getEnvironmentVariables()` | Returns the environment for a child process or an SDK. |

## How the container starts

1. The container starts the image with the arguments `-p 9297` and exposes port 9297.
2. The container sets `EMULATOR_TELEMETRY=off`, unless you turn telemetry on.
3. The container sets `EMULATOR_TLS_NAMES` to the Testcontainers host, all network aliases, and the extra names.
4. The wait strategy waits for the log line `TLS front end ready`. The timeout is 120 seconds.
5. The container copies `emulator_credentials` and `tls-ca.pem` from `/home/emulator/.aws/emulator/9297/`.
   It uses the Docker archive API, so the distroless image also works.

## Network aliases

Other containers on the same Testcontainers network can call the emulator by its alias.
The certificate contains each alias.
Give the CA to the other container, for example with `withCopyToContainer`.

```java
Network network = Network.newNetwork();
AwsEmulatorContainer emulator = new AwsEmulatorContainer()
        .withNetwork(network)
        .withNetworkAliases("aws");
```

The other container then calls `https://aws:9297`.

## Run the tests

The tests use JUnit 5 and the AWS SDK for Java 2.x.
They test STS, S3, SQS, the TLS context, the environment map, and a call from a second container by the network alias.

```sh
AWS_EMULATOR_IMAGE=aws-emulator:local testcontainers/java/run-tests.sh
```

`AWS_EMULATOR_IMAGE` changes the image in the tests only. The default is `ghcr.io/xsavikx/aws-emulator:latest`.

## Limits

- The image must have the TLS front end. An older image does not write the log line `TLS front end ready`, and the start fails after 120 seconds.
- The Region is always `us-east-1`. The account is always `000000000000`.
- The container uses port 9297 in the container. You cannot change it with this module.
- `getCaCertificateFile()` writes a file in the default temporary folder. Delete it yourself if the JVM does not exit normally.
- The library does not change the JVM truststore. Give the trust managers or the TLS context to each client.
