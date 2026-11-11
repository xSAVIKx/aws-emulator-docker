"""Secrets scenario: Secrets Manager JSON secret."""

import json


def test_create_read_delete_json_secret(secretsmanager, unique_name):
    value = {"username": "app", "password": "not-a-real-password"}

    secretsmanager.create_secret(Name=unique_name, SecretString=json.dumps(value))
    try:
        stored = secretsmanager.get_secret_value(SecretId=unique_name)
        assert json.loads(stored["SecretString"]) == value
    finally:
        # Without this flag the secret waits 7 to 30 days in a recovery window.
        secretsmanager.delete_secret(SecretId=unique_name, ForceDeleteWithoutRecovery=True)
