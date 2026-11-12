# Python example: pytest + Boto3

This example tests the AWS Emulator with [pytest](https://docs.pytest.org/) and
[Boto3](https://boto3.amazonaws.com/v1/documentation/api/latest/index.html), the AWS SDK
for Python. It has one test for each scenario in the [examples overview](../README.md):
STS, S3, DynamoDB, SQS, SNS to SQS, and Secrets Manager.

## Prerequisites

- Docker.
- [uv](https://docs.astral.sh/uv/). uv installs Python 3.13 (see `.python-version`) and
  the packages. You do not need to install Python yourself.

## Run the tests

From the repository root, run:

```sh
examples/with-emulator.sh examples/python/run-tests.sh
```

The script starts a fresh emulator, runs the tests, and stops the emulator.
`run-tests.sh` runs `uv run --locked pytest -v`. Extra arguments go to pytest, for
example `examples/python/run-tests.sh -k queue`.

## Files

| File | Purpose |
|---|---|
| `pyproject.toml` | The project, the pinned versions of Boto3 and pytest, and pytest settings. |
| `uv.lock` | The exact versions of all packages. `--locked` uses this file and stops if it does not match `pyproject.toml`. |
| `.python-version` | The Python version that uv uses. |
| `run-tests.sh` | The entry point for local runs and for CI. |
| `tests/conftest.py` | The endpoint guard, one Boto3 client per service, and the `unique_name` fixture. |
| `tests/test_identity.py` | STS: `GetCallerIdentity` returns account `000000000000`. |
| `tests/test_object_storage.py` | S3: create a bucket, put, list, get and delete an object. |
| `tests/test_key_value_table.py` | DynamoDB: create a table, wait for `ACTIVE`, put, get and query items. |
| `tests/test_queue.py` | SQS: send, receive and delete a message. |
| `tests/test_fan_out.py` | SNS + SQS: queue policy, raw subscription, publish, receive. |
| `tests/test_secrets.py` | Secrets Manager: create, read and delete a JSON secret. |

Each test makes names like `ex-py-1a2b3c4d`, so runs do not collide. Each test deletes
its resources in a `finally` block, also when an assertion fails.

## Environment variables

The code contains no endpoint, keys or certificate paths. Boto3 reads the standard
variables that the start script sets:

- `AWS_ENDPOINT_URL`: the emulator address, `https://127.0.0.1:<port>`.
- `AWS_SHARED_CREDENTIALS_FILE` and `AWS_CONFIG_FILE`: the credentials and the
  configuration that the emulator writes.
- `AWS_CA_BUNDLE`: the emulator CA certificate. Boto3 uses it to verify HTTPS.
- `AWS_REGION`: `us-east-1`.

The S3 client uses path-style addresses, because the endpoint is an IP address.

`tests/conftest.py` stops the run at once if the host in `AWS_ENDPOINT_URL` is not
`127.0.0.1` or `localhost`. This guard prevents a run against a real AWS account.
