# testcontainers-aws-emulator (Python)

<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev> -->

This package is a [Testcontainers](https://testcontainers-python.readthedocs.io/) module for the
AWS Emulator Docker image. It starts the emulator in a container with HTTPS. It gives you the
endpoint, the keys and the CA certificate for the official AWS SDK (Boto3).

- Distribution: `testcontainers-aws-emulator`
- Import: `testcontainers_aws_emulator`
- Class: `AwsEmulatorContainer`
- Python 3.10 or later. Docker must be available.

## Install

The package is not on PyPI yet. Install it from Git or from a local copy of the repository.

With uv:

```sh
uv add --dev "testcontainers-aws-emulator @ git+https://github.com/xSAVIKx/aws-emulator-docker#subdirectory=testcontainers/python"
```

With pip:

```sh
pip install "testcontainers-aws-emulator @ git+https://github.com/xSAVIKx/aws-emulator-docker#subdirectory=testcontainers/python"
```

From a local copy:

```sh
pip install ./testcontainers/python
```

The package does not depend on Boto3. Install Boto3 in your project.

## Usage with Boto3

```python
import boto3
from botocore.config import Config
from testcontainers_aws_emulator import AwsEmulatorContainer

with AwsEmulatorContainer() as emulator:
    session = boto3.session.Session(
        aws_access_key_id=emulator.get_access_key_id(),
        aws_secret_access_key=emulator.get_secret_access_key(),
        region_name=emulator.get_region(),
    )
    ca_file = str(emulator.write_ca_file())

    sts = session.client("sts", endpoint_url=emulator.get_endpoint_url(), verify=ca_file)
    assert sts.get_caller_identity()["Account"] == "000000000000"

    # Use path-style addressing for S3. The bucket goes in the path, not in the host name.
    s3 = session.client(
        "s3",
        endpoint_url=emulator.get_endpoint_url(),
        verify=ca_file,
        config=Config(s3={"addressing_style": "path"}),
    )
    s3.create_bucket(Bucket="my-bucket")
```

A pytest fixture for the whole test session:

```python
import pytest
from testcontainers_aws_emulator import AwsEmulatorContainer


@pytest.fixture(scope="session")
def emulator():
    with AwsEmulatorContainer() as container:
        yield container
```

### Environment for a child process

`get_env()` returns the variables for an AWS SDK or a CLI in a child process:

| Variable | Value |
| --- | --- |
| `AWS_ENDPOINT_URL` | `https://<host>:<mapped port>` |
| `AWS_ACCESS_KEY_ID` | The access key ID of the emulator |
| `AWS_SECRET_ACCESS_KEY` | The secret access key of the emulator |
| `AWS_REGION` | `us-east-1` |
| `AWS_DEFAULT_REGION` | `us-east-1` |
| `AWS_CA_BUNDLE` | The path of a temporary file with the CA certificate |
| `AWS_EC2_METADATA_DISABLED` | `true` |

```python
import os
import subprocess

subprocess.run(["aws", "s3", "ls"], env={**os.environ, **emulator.get_env()}, check=True)
```

### Other HTTPS clients

`get_ssl_context()` returns an `ssl.SSLContext` that trusts only the CA of the container. It
does not trust the system CA certificates. Use it with `urllib`, `http.client`, `httpx` or
`aiohttp`:

```python
import urllib.request

url = f"{emulator.get_endpoint_url()}/__admin/health"
with urllib.request.urlopen(url, context=emulator.get_ssl_context()) as response:
    print(response.read())  # b"ok"
```

## API

| Member | Description |
| --- | --- |
| `AwsEmulatorContainer(image=DEFAULT_IMAGE, *, tls_names=(), telemetry=False, **kwargs)` | Make the container. `kwargs` go to `DockerContainer`. |
| `with_tls_names(*names)` | Add host names or IP addresses to the certificate. |
| `with_telemetry(enabled=True)` | Turn the telemetry of the emulator on or off. |
| `get_endpoint_url()` | The HTTPS endpoint URL. |
| `get_region()` | `us-east-1`. |
| `get_access_key_id()` | The access key ID. |
| `get_secret_access_key()` | The secret access key. |
| `get_ca_pem()` | The CA certificate as PEM text (`str`). |
| `get_ca_pem_bytes()` | The CA certificate as PEM bytes. |
| `write_ca_file()` | Write the CA certificate to a temporary file. Return the path. |
| `get_env()` | The environment variables in the table above. |
| `get_ssl_context()` | An `ssl.SSLContext` that trusts only the CA. |

The module also exports the constants `DEFAULT_IMAGE`, `PORT` (9297), `REGION` (`us-east-1`) and
`ACCOUNT_ID` (`000000000000`).

The access methods work only after the container starts. Before the start, they raise
`RuntimeError`.

## Options

- **Image.** The default image is `ghcr.io/xsavikx/aws-emulator:latest`. Give another image as
  the first argument.
- **TLS names.** The certificate always has `localhost`, `*.localhost`, `127.0.0.1`, `::1`, the
  host name and the IP addresses of the container. The module adds the Testcontainers host, the
  network aliases of the container and the names from `tls_names` or `with_tls_names()`. Add a
  name when a client reaches the container by a name that the module does not know.
- **Telemetry.** The module sets `EMULATOR_TELEMETRY=off`. Set `telemetry=True` to turn the
  telemetry on. The module then does not set the variable.
- **Network.** Use `with_network()` and `with_network_aliases()` from `DockerContainer`. Other
  containers on the network then reach `https://<alias>:9297`. Give them the CA certificate
  from `get_ca_pem_bytes()`.

## How the module works

1. The module sets `EMULATOR_TLS_NAMES` and starts the image with the arguments `-p 9297`.
2. The module waits for the log line `TLS front end ready`. The timeout is 120 seconds.
3. The module copies `emulator_credentials` and `tls-ca.pem` out of the container with the
   Docker archive API.
4. The module deletes the temporary CA files when the container stops.

The module does not use the Docker health check. The module does not run commands in the
container, because the distroless image has no shell.

## Limits

- The image must have the TLS front end. Images without it never write the line
  `TLS front end ready`, and the start fails after 120 seconds.
- The Testcontainers host goes into the certificate only in the `docker_host` connection mode.
  This is the usual mode on a developer computer and in CI. In the `gateway_ip` mode (Docker in
  Docker), the gateway address is not known before the start. Add it with `with_tls_names()`.
- The Region is always `us-east-1`. The account is always `000000000000`.
- Each start makes new keys. Read them from the container. Do not write them in your code.

## Run the tests

The tests use pytest and Boto3. They start the containers through Testcontainers.

```sh
AWS_EMULATOR_IMAGE=aws-emulator:tls-al testcontainers/python/run-tests.sh
```

`AWS_EMULATOR_IMAGE` selects the image for the tests. The default is
`ghcr.io/xsavikx/aws-emulator:latest`. The library itself does not read this variable.

## License

Apache-2.0. See [LICENSE](LICENSE).
