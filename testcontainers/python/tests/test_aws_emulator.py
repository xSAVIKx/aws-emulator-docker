# SPDX-License-Identifier: Apache-2.0
"""Tests of AwsEmulatorContainer with Boto3."""

import json
import os
import ssl
import subprocess
import sys
import urllib.error
import urllib.request

import pytest
from testcontainers.core.container import DockerContainer
from testcontainers.core.network import Network

from testcontainers_aws_emulator import ACCOUNT_ID, AwsEmulatorContainer

CURL_IMAGE = "curlimages/curl:8.23.0@sha256:3fb7e13e5d179a845c97d78204f3f1064d1907037f26a3ebf91710571151fd93"


def test_sts_caller_identity(sts):
    assert sts.get_caller_identity()["Account"] == ACCOUNT_ID


def test_s3_bucket_put_get(s3, unique_name):
    body = b"Hello from Testcontainers"
    s3.create_bucket(Bucket=unique_name)
    try:
        s3.put_object(Bucket=unique_name, Key="hello.txt", Body=body)
        assert s3.get_object(Bucket=unique_name, Key="hello.txt")["Body"].read() == body
    finally:
        for obj in s3.list_objects_v2(Bucket=unique_name).get("Contents", []):
            s3.delete_object(Bucket=unique_name, Key=obj["Key"])
        s3.delete_bucket(Bucket=unique_name)


def test_sqs_send_receive(sqs, unique_name):
    queue_url = sqs.create_queue(QueueName=unique_name)["QueueUrl"]
    try:
        sqs.send_message(QueueUrl=queue_url, MessageBody="hello queue")
        messages = sqs.receive_message(QueueUrl=queue_url, MaxNumberOfMessages=1, WaitTimeSeconds=5).get("Messages", [])
        assert [m["Body"] for m in messages] == ["hello queue"]
    finally:
        sqs.delete_queue(QueueUrl=queue_url)


def test_ssl_context_trusts_only_the_ca(emulator):
    context = emulator.get_ssl_context()
    with urllib.request.urlopen(f"{emulator.get_endpoint_url()}/__admin/health", context=context) as response:
        assert response.read().decode().strip() == "ok"
    # The system trust store does not know the CA of the container.
    with pytest.raises(urllib.error.URLError) as info:
        urllib.request.urlopen(f"{emulator.get_endpoint_url()}/__admin/health", context=ssl.create_default_context())
    assert isinstance(info.value.reason, ssl.SSLCertVerificationError)


def test_env_for_child_process(emulator):
    env = emulator.get_env()
    assert env["AWS_ENDPOINT_URL"] == emulator.get_endpoint_url()
    assert env["AWS_REGION"] == env["AWS_DEFAULT_REGION"] == "us-east-1"
    assert env["AWS_EC2_METADATA_DISABLED"] == "true"
    assert os.path.isfile(env["AWS_CA_BUNDLE"])
    # A child process with only these variables reaches the emulator.
    script = "import boto3, json; print(json.dumps(boto3.client('sts').get_caller_identity()['Account']))"
    child_env = {"PATH": os.environ.get("PATH", ""), "HOME": "/nonexistent", **env}
    out = subprocess.run([sys.executable, "-c", script], env=child_env, check=True, capture_output=True, text=True)
    assert json.loads(out.stdout) == ACCOUNT_ID


def test_network_alias(image):
    # A second container on the same network calls the emulator by its alias.
    # The certificate has the name "aws", because the alias is in EMULATOR_TLS_NAMES.
    with (
        Network() as network,
        AwsEmulatorContainer(image).with_network(network).with_network_aliases("aws") as emulator,
    ):
        assert emulator.env["EMULATOR_TLS_NAMES"].split(",")[-1] == "aws"
        client = (
            DockerContainer(CURL_IMAGE)
            .with_network(network)
            .with_copy_into_container(emulator.get_ca_pem_bytes(), "/ca.pem")
            .with_command(["-sf", "--cacert", "/ca.pem", "https://aws:9297/__admin/health"])
        )
        with client:
            exit_code = client.wait()
            stdout, stderr = client.get_logs()
        assert exit_code == 0, stderr.decode()
        assert stdout.decode().strip() == "ok"


def test_tls_names_include_aliases_and_extras(image):
    # Not started: the method only builds the name list.
    container = AwsEmulatorContainer(image, tls_names=["aws.test"]).with_network_aliases("a1")
    names = container._tls_name_list()
    assert names[-2:] == ["a1", "aws.test"]


def test_invalid_tls_name_is_rejected():
    with pytest.raises(ValueError):
        AwsEmulatorContainer(tls_names=["a,b"])
