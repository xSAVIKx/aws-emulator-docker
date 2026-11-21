# SPDX-License-Identifier: Apache-2.0
"""Testcontainers module for the AWS Emulator Docker image."""

from .container import ACCOUNT_ID, DEFAULT_IMAGE, PORT, REGION, AwsEmulatorContainer

__all__ = ["ACCOUNT_ID", "DEFAULT_IMAGE", "PORT", "REGION", "AwsEmulatorContainer"]
