<!-- SPDX-License-Identifier: Apache-2.0 -->
# Testcontainers for Go: AWS Emulator module

This module starts the AWS Emulator image in a
[Testcontainers for Go](https://golang.testcontainers.org/) test. The default
image is `ghcr.io/xsavikx/aws-emulator:latest`.

The module starts the TLS front end of the image. The front end serves HTTPS on
port 9297. The certificate is valid for the Testcontainers host, for all network
aliases of the container, and for the names that you add. A CA in the container
signs the certificate. After the start, the module copies the CA certificate and
the generated credentials out of the container.

The module does not depend on the AWS SDK. It gives an HTTP client and a TLS
configuration that trust only the CA of the container.

Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>

## Install

The module is not published yet. Install it from the Git repository.

1. Get the module at a commit of the repository:

   ```sh
   go get github.com/xSAVIKx/aws-emulator-docker/testcontainers/go@<commit>
   ```

2. Or clone the repository and add a `replace` directive to your `go.mod`:

   ```text
   require github.com/xSAVIKx/aws-emulator-docker/testcontainers/go v0.0.0
   replace github.com/xSAVIKx/aws-emulator-docker/testcontainers/go => ../aws-emulator-docker/testcontainers/go
   ```

The module requires Go 1.26 or later and testcontainers-go v0.45.0 or later.

## Usage with the AWS SDK for Go v2

```go
package app_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/testcontainers/testcontainers-go"

	awsemulator "github.com/xSAVIKx/aws-emulator-docker/testcontainers/go"
)

func TestCallerIdentity(t *testing.T) {
	ctx := t.Context()

	ctr, err := awsemulator.Run(ctx, awsemulator.DefaultImage)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}

	endpoint, err := ctr.EndpointURL(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(ctr.Region()),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			ctr.AccessKeyID(), ctr.SecretAccessKey(), "")),
		config.WithBaseEndpoint(endpoint),
		config.WithHTTPClient(ctr.HTTPClient()),
	)
	if err != nil {
		t.Fatal(err)
	}

	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatal(err)
	}
	if aws.ToString(out.Account) != awsemulator.AccountID {
		t.Fatalf("account: %s", aws.ToString(out.Account))
	}
}
```

For S3, set `o.UsePathStyle = true` in the S3 client options. The endpoint host
is `localhost` or an IP address, so virtual-hosted addresses do not work.

## API

| Name | Description |
| --- | --- |
| `Run(ctx, img, opts...)` | Creates and starts the container. On an error, terminate the returned container if it is not nil. |
| `(*Container).EndpointURL(ctx)` | Returns `https://<host>:<mapped port>`. |
| `(*Container).Region()`, `Region` | Return `us-east-1`. |
| `AccountID` | The account ID `000000000000`. |
| `(*Container).AccessKeyID()`, `(*Container).SecretAccessKey()` | Return the credentials that the emulator generated. |
| `(*Container).CACertPEM()` | Returns the CA certificate (PEM). |
| `(*Container).CACertFile()` | Writes the CA certificate to a temporary file and returns the path. `Terminate` removes the file. |
| `(*Container).Env(ctx)` | Returns `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `AWS_DEFAULT_REGION`, `AWS_CA_BUNDLE` and `AWS_EC2_METADATA_DISABLED=true` for a child process. |
| `(*Container).CertPool()` | Returns a certificate pool with only the CA. |
| `(*Container).TLSConfig()` | Returns a `*tls.Config` that trusts only the CA. |
| `(*Container).HTTPClient()` | Returns an `*http.Client` that trusts only the CA. Give it to `config.WithHTTPClient`. |

## Options

Give the options to `Run`. You can also give all generic testcontainers options,
for example `network.WithNetwork`.

| Option | Description |
| --- | --- |
| `WithTLSNames(names...)` | Adds host names or IP addresses to the certificate. |
| `WithTelemetry(on)` | Turns the telemetry of the emulator on or off. The default is off (`EMULATOR_TELEMETRY=off`). With `true`, the module does not set the variable. |

The module sets `EMULATOR_TLS_NAMES` to the Testcontainers host, the network
aliases of the container, and the names from `WithTLSNames`. If the list is
empty, the module uses `localhost`.

### Calls from other containers

1. Create a network with `network.New`.
2. Start the emulator with `network.WithNetwork([]string{"aws"}, nw)`.
3. Copy `CACertPEM()` into the other container.
4. Call `https://aws:9297` from the other container.

## Tests

The tests use the AWS SDK for Go v2. They test STS, S3, SQS, the environment
variables, the extra TLS names, and a call from a second container through the
network alias `aws`.

1. Start Docker.
2. Run the tests:

   ```sh
   AWS_EMULATOR_IMAGE=aws-emulator:tls-al ./run-tests.sh
   ```

`AWS_EMULATOR_IMAGE` selects the image. The default is
`ghcr.io/xsavikx/aws-emulator:latest`.

## Limits

- The image must have the TLS front end. An image without it does not write
  the line `TLS front end ready`, and `Run` stops after 120 seconds.
- The module learns the Testcontainers host before the start from the Docker
  provider. With another provider, the certificate has only the network
  aliases, the extra names, and the names that the image always adds.
- If you reach the Docker host through a name that the module does not know,
  for example through an SSH tunnel, add that name with `WithTLSNames`.
- The region is always `us-east-1`.

## License

Apache-2.0. See the `LICENSE` file in the root of the repository.
