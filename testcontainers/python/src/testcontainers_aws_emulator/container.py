# Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
# SPDX-License-Identifier: Apache-2.0
"""Testcontainers module for the AWS Emulator Docker image.

The container starts the emulator with the TLS front end. The front end
serves a certificate for localhost, the Testcontainers host, the network
aliases of the container and the extra names that you add. A CA in the
container signs the certificate. After the start, the module copies the
credentials and the CA certificate out of the container with the Docker
archive API. It does not use exec, because the distroless image has no shell.
"""

from __future__ import annotations

import configparser
import os
import pathlib
import ssl
import tempfile
from collections.abc import Iterable
from typing import Any

from testcontainers.core.config import ConnectionMode
from testcontainers.core.container import DockerContainer
from testcontainers.core.wait_strategies import LogMessageWaitStrategy

try:  # Python 3.11 and later.
    from typing import Self
except ImportError:  # pragma: no cover - Python 3.10.
    from typing_extensions import Self

__all__ = [
    "ACCOUNT_ID",
    "DEFAULT_IMAGE",
    "PORT",
    "REGION",
    "AwsEmulatorContainer",
]

DEFAULT_IMAGE = "ghcr.io/xsavikx/aws-emulator:latest"
"""The image that the container uses when you do not give an image."""

PORT = 9297
"""The port of the emulator in the container."""

REGION = "us-east-1"
"""The Region of the emulator. It is a constant."""

ACCOUNT_ID = "000000000000"
"""The account ID of the emulator. It is a constant."""

READY_LOG_MESSAGE = "TLS front end ready"
STARTUP_TIMEOUT_SECONDS = 120
RUN_DIR = f"/home/emulator/.aws/emulator/{PORT}"
CREDENTIALS_PATH = f"{RUN_DIR}/emulator_credentials"
CA_PATH = f"{RUN_DIR}/tls-ca.pem"


class AwsEmulatorContainer(DockerContainer):
    """The AWS Emulator in a Docker container, with HTTPS on port 9297.

    Example::

        import boto3
        from testcontainers_aws_emulator import AwsEmulatorContainer

        with AwsEmulatorContainer() as emulator:
            sts = boto3.client(
                "sts",
                endpoint_url=emulator.get_endpoint_url(),
                region_name=emulator.get_region(),
                aws_access_key_id=emulator.get_access_key_id(),
                aws_secret_access_key=emulator.get_secret_access_key(),
                verify=str(emulator.write_ca_file()),
            )
            assert sts.get_caller_identity()["Account"] == "000000000000"

    Args:
        image: The image to start. The default is ``DEFAULT_IMAGE``.
        tls_names: More host names or IP addresses for the certificate.
        telemetry: ``True`` turns the telemetry of the emulator on. The
            default is ``False``, which sets ``EMULATOR_TELEMETRY=off``.
        **kwargs: Arguments for ``DockerContainer``.
    """

    def __init__(
        self,
        image: str = DEFAULT_IMAGE,
        *,
        tls_names: Iterable[str] = (),
        telemetry: bool = False,
        **kwargs: Any,
    ) -> None:
        super().__init__(image, **kwargs)
        self._tls_names: list[str] = []
        self._telemetry = telemetry
        self._access_key_id: str | None = None
        self._secret_access_key: str | None = None
        self._ca_pem: bytes | None = None
        self._ca_files: list[pathlib.Path] = []
        self.with_tls_names(*tls_names)
        self.with_exposed_ports(PORT)
        self.with_command(["-p", str(PORT)])
        self.waiting_for(LogMessageWaitStrategy(READY_LOG_MESSAGE).with_startup_timeout(STARTUP_TIMEOUT_SECONDS))

    # Options.

    def with_tls_names(self, *names: str) -> Self:
        """Add host names or IP addresses to the certificate of the TLS front end.

        Use this option when a client reaches the container by a name that
        the module does not know, for example a DNS name of a remote Docker host.
        """
        for name in names:
            name = name.strip()
            if not name or "," in name or any(ch.isspace() for ch in name):
                raise ValueError(f"not a valid TLS name: {name!r}")
            self._tls_names.append(name)
        return self

    def with_telemetry(self, enabled: bool = True) -> Self:
        """Turn the telemetry of the emulator on or off. The default is off."""
        self._telemetry = enabled
        return self

    def _configure(self) -> None:
        if self._telemetry:
            self.env.pop("EMULATOR_TELEMETRY", None)
        else:
            self.env["EMULATOR_TELEMETRY"] = "off"
        self.env["EMULATOR_TLS_NAMES"] = ",".join(self._tls_name_list())

    def _tls_name_list(self) -> list[str]:
        """Return the names for EMULATOR_TLS_NAMES. The list is never empty."""
        names: list[str] = []
        # In the docker_host mode the host is known before the start. In the
        # other modes the host is an address of the container network. The
        # front end adds the IP addresses of the container itself.
        client = self.get_docker_client()
        if client.get_connection_mode() == ConnectionMode.docker_host:
            names.append(client.host())
        names.extend(self._network_aliases or [])
        names.extend(self._tls_names)
        unique = list(dict.fromkeys(n for n in names if n))
        return unique or ["localhost"]

    # Life cycle.

    def start(self) -> Self:
        super().start()
        try:
            self._read_run_files()
        except Exception:
            self.stop()
            raise
        return self

    def stop(self, force: bool = True, delete_volume: bool = True) -> None:
        try:
            super().stop(force=force, delete_volume=delete_volume)
        finally:
            for path in self._ca_files:
                path.unlink(missing_ok=True)
            self._ca_files.clear()

    def _read_run_files(self) -> None:
        with tempfile.TemporaryDirectory(prefix="aws-emulator-") as tmp:
            credentials_file = pathlib.Path(tmp, "emulator_credentials")
            ca_file = pathlib.Path(tmp, "tls-ca.pem")
            self.copy_from_container(CREDENTIALS_PATH, credentials_file)
            self.copy_from_container(CA_PATH, ca_file)
            credentials_text = credentials_file.read_text(encoding="utf-8")
            ca_pem = ca_file.read_bytes()

        parser = configparser.ConfigParser()
        parser.read_string(credentials_text)
        section = parser["default"]
        self._access_key_id = section["aws_access_key_id"].strip()
        self._secret_access_key = section["aws_secret_access_key"].strip()
        if b"BEGIN CERTIFICATE" not in ca_pem:
            raise RuntimeError(f"{CA_PATH} in the container does not contain a PEM certificate")
        self._ca_pem = ca_pem

    def _require_started(self, value: Any | None, what: str) -> Any:
        if value is None:
            raise RuntimeError(f"start the container before you read the {what}")
        return value

    # Connection details.

    def get_endpoint_url(self) -> str:
        """Return the HTTPS endpoint, for example ``https://localhost:32768``."""
        host = self.get_container_host_ip()
        if ":" in host and not host.startswith("["):
            host = f"[{host}]"
        return f"https://{host}:{self.get_exposed_port(PORT)}"

    def get_region(self) -> str:
        """Return the Region of the emulator, ``us-east-1``."""
        return REGION

    def get_access_key_id(self) -> str:
        """Return the access key ID that the emulator accepts."""
        return self._require_started(self._access_key_id, "access key ID")

    def get_secret_access_key(self) -> str:
        """Return the secret access key that the emulator accepts."""
        return self._require_started(self._secret_access_key, "secret access key")

    def get_ca_pem(self) -> str:
        """Return the CA certificate of the TLS front end as PEM text."""
        return self.get_ca_pem_bytes().decode("ascii")

    def get_ca_pem_bytes(self) -> bytes:
        """Return the CA certificate of the TLS front end as PEM bytes."""
        return self._require_started(self._ca_pem, "CA certificate")

    def write_ca_file(self) -> pathlib.Path:
        """Write the CA certificate to a new temporary file and return its path.

        Use the path for AWS_CA_BUNDLE, SSL_CERT_FILE or the ``verify``
        argument of Boto3. The module deletes the file when the container stops.
        """
        pem = self.get_ca_pem_bytes()
        fd, name = tempfile.mkstemp(prefix="aws-emulator-ca-", suffix=".pem")
        with os.fdopen(fd, "wb") as f:
            f.write(pem)
        path = pathlib.Path(name)
        self._ca_files.append(path)
        return path

    def get_env(self) -> dict[str, str]:
        """Return the environment variables for an AWS SDK or a child process.

        AWS_CA_BUNDLE is the path of a new temporary CA file.
        """
        endpoint = self.get_endpoint_url()
        return {
            "AWS_ENDPOINT_URL": endpoint,
            "AWS_ACCESS_KEY_ID": self.get_access_key_id(),
            "AWS_SECRET_ACCESS_KEY": self.get_secret_access_key(),
            "AWS_REGION": REGION,
            "AWS_DEFAULT_REGION": REGION,
            "AWS_CA_BUNDLE": str(self.write_ca_file()),
            "AWS_EC2_METADATA_DISABLED": "true",
        }

    def get_ssl_context(self) -> ssl.SSLContext:
        """Return an SSL context that trusts only the CA of this container.

        The context does not load the system CA certificates. It checks the
        host name. Use it with urllib, http.client, aiohttp or httpx.
        """
        return ssl.create_default_context(cadata=self.get_ca_pem())
