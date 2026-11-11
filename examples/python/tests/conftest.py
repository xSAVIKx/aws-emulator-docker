"""Shared pytest fixtures for the AWS Emulator examples.

The tests contain no endpoint, keys or certificate paths. Boto3 reads them
from the standard environment variables that the start script sets:
AWS_ENDPOINT_URL, AWS_SHARED_CREDENTIALS_FILE, AWS_CONFIG_FILE, AWS_CA_BUNDLE
and AWS_REGION. See ../README.md.
"""

import os
import uuid
from urllib.parse import urlparse

import boto3
import pytest
from botocore.config import Config

# The only hosts that the tests accept as an endpoint.
LOCAL_HOSTS = {"127.0.0.1", "localhost"}


def pytest_sessionstart(session: pytest.Session) -> None:
    """Stop the run before any AWS call if the endpoint is not local.

    This guard prevents a run against a real AWS account by accident.
    """
    endpoint = os.environ.get("AWS_ENDPOINT_URL", "")
    host = urlparse(endpoint).hostname
    if host not in LOCAL_HOSTS:
        pytest.exit(
            f"AWS_ENDPOINT_URL must point to 127.0.0.1 or localhost, got {endpoint!r}. "
            "Start the emulator first: examples/with-emulator.sh examples/python/run-tests.sh",
            returncode=2,
        )


@pytest.fixture
def unique_name() -> str:
    """Return a new resource name for each test, for example 'ex-py-1a2b3c4d'.

    The name is lowercase, so it is also a valid S3 bucket name.
    """
    return f"ex-py-{uuid.uuid4().hex[:8]}"


# One client per service for the whole session. Boto3 uses the default
# credential and configuration chain, so it reads everything from the env.


@pytest.fixture(scope="session")
def sts():
    return boto3.client("sts")


@pytest.fixture(scope="session")
def s3():
    # The endpoint is an IP address, so S3 cannot use virtual-hosted
    # addresses (<bucket>.<host>). Path-style puts the bucket in the path.
    return boto3.client("s3", config=Config(s3={"addressing_style": "path"}))


@pytest.fixture(scope="session")
def dynamodb():
    return boto3.client("dynamodb")


@pytest.fixture(scope="session")
def sqs():
    return boto3.client("sqs")


@pytest.fixture(scope="session")
def sns():
    return boto3.client("sns")


@pytest.fixture(scope="session")
def secretsmanager():
    return boto3.client("secretsmanager")
