# SPDX-License-Identifier: Apache-2.0
"""Shared pytest fixtures: one emulator container for the test session.

AWS_EMULATOR_IMAGE selects the image. The default is the released image.
"""

import os
import uuid

import boto3
import pytest
from botocore.config import Config

from testcontainers_aws_emulator import DEFAULT_IMAGE, AwsEmulatorContainer

IMAGE = os.environ.get("AWS_EMULATOR_IMAGE") or DEFAULT_IMAGE


@pytest.fixture(scope="session")
def image() -> str:
    return IMAGE


@pytest.fixture(scope="session")
def emulator():
    with AwsEmulatorContainer(IMAGE) as container:
        yield container


@pytest.fixture(scope="session")
def boto_session(emulator) -> boto3.session.Session:
    # Give the keys and the Region explicitly. The tests do not read
    # ~/.aws or the AWS_* variables of the host.
    return boto3.session.Session(
        aws_access_key_id=emulator.get_access_key_id(),
        aws_secret_access_key=emulator.get_secret_access_key(),
        region_name=emulator.get_region(),
    )


@pytest.fixture(scope="session")
def client_kwargs(emulator) -> dict:
    # verify takes the path of a PEM file. Boto3 then trusts only that CA.
    return {"endpoint_url": emulator.get_endpoint_url(), "verify": str(emulator.write_ca_file())}


@pytest.fixture
def unique_name() -> str:
    """Return a new lowercase resource name, for example 'tc-py-1a2b3c4d'."""
    return f"tc-py-{uuid.uuid4().hex[:8]}"


@pytest.fixture(scope="session")
def sts(boto_session, client_kwargs):
    return boto_session.client("sts", **client_kwargs)


@pytest.fixture(scope="session")
def s3(boto_session, client_kwargs):
    # Path-style puts the bucket in the path, not in the host name.
    return boto_session.client("s3", config=Config(s3={"addressing_style": "path"}), **client_kwargs)


@pytest.fixture(scope="session")
def sqs(boto_session, client_kwargs):
    return boto_session.client("sqs", **client_kwargs)
