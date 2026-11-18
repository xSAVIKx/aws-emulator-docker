// Crate testcontainers-aws-emulator is a Testcontainers module for the
// AWS Emulator image.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

//! A [Testcontainers](https://docs.rs/testcontainers) module for the AWS
//! Emulator image (`ghcr.io/xsavikx/aws-emulator`).
//!
//! [`AwsEmulator`] implements [`testcontainers::Image`]. The container serves
//! HTTPS on port 9297 with a certificate that its own CA signs. After the
//! start, [`StartedAwsEmulator`] gives the endpoint, the credentials and the
//! CA certificate.
//!
//! ```no_run
//! use testcontainers_aws_emulator::AwsEmulator;
//!
//! # async fn example() -> Result<(), testcontainers_aws_emulator::Error> {
//! let emulator = AwsEmulator::default().run().await?;
//! println!("endpoint: {}", emulator.endpoint());
//! println!("access key ID: {}", emulator.access_key_id());
//! # Ok(())
//! # }
//! ```
//!
//! The crate does not depend on the AWS SDK. To trust the CA with the AWS
//! SDK for Rust, give [`StartedAwsEmulator::ca_pem`] to the TLS trust store
//! of `aws-smithy-http-client`. The README shows the full code.

use std::borrow::Cow;
use std::collections::BTreeMap;
use std::fmt;
use std::io::Write as _;
use std::path::Path;
use std::time::Duration;

use testcontainers::core::error::TestcontainersError;
use testcontainers::core::{ContainerPort, WaitFor};
use testcontainers::runners::AsyncRunner;
use testcontainers::{ContainerAsync, ContainerRequest, Image, ImageExt};

/// The default image.
pub const DEFAULT_IMAGE: &str = "ghcr.io/xsavikx/aws-emulator:latest";
/// The port of the emulator in the container (HTTPS).
pub const PORT: u16 = 9297;
/// The AWS Region of the emulator. It is constant.
pub const REGION: &str = "us-east-1";
/// The AWS account ID of the emulator.
pub const ACCOUNT_ID: &str = "000000000000";
/// The maximum time for the start of the container.
pub const STARTUP_TIMEOUT: Duration = Duration::from_secs(120);
/// The log line (on stderr) that tells that the TLS front end is ready.
pub const READY_MESSAGE: &str = "TLS front end ready";
/// The environment variable that overrides the host of the endpoint.
///
/// When it is set, the module uses its value as the host of the endpoint and
/// adds it to the certificate names. Other Testcontainers libraries use the
/// same name.
pub const HOST_OVERRIDE_ENV: &str = "TESTCONTAINERS_HOST_OVERRIDE";

const CREDENTIALS_FILE: &str = "/home/emulator/.aws/emulator/9297/emulator_credentials";
const CA_FILE: &str = "/home/emulator/.aws/emulator/9297/tls-ca.pem";
const EXPOSED_PORTS: [ContainerPort; 1] = [ContainerPort::Tcp(PORT)];

/// The AWS Emulator image with its options.
///
/// Use [`AwsEmulator::run`] to start the container and to read the
/// credentials and the CA. You can also start the container with
/// [`AsyncRunner::start`] and then call [`StartedAwsEmulator::from_container`].
#[derive(Debug, Clone)]
pub struct AwsEmulator {
    name: String,
    tag: String,
    tls_names: Vec<String>,
    network: Option<(String, String)>,
    telemetry: bool,
}

impl Default for AwsEmulator {
    fn default() -> Self {
        Self::new(DEFAULT_IMAGE)
    }
}

impl AwsEmulator {
    /// Makes a module for an image reference, for example
    /// `ghcr.io/xsavikx/aws-emulator:latest`.
    ///
    /// A reference without a tag gets the tag `latest`. A reference with a
    /// digest (`name@sha256:...`) is also accepted.
    pub fn new(image: impl AsRef<str>) -> Self {
        let (name, tag) = split_image(image.as_ref());
        Self {
            name,
            tag,
            tls_names: Vec::new(),
            network: None,
            telemetry: false,
        }
    }

    /// Adds a host name or an IP address to the server certificate.
    pub fn with_tls_name(mut self, name: impl Into<String>) -> Self {
        self.tls_names.push(name.into());
        self
    }

    /// Adds host names or IP addresses to the server certificate.
    pub fn with_tls_names(mut self, names: impl IntoIterator<Item = impl Into<String>>) -> Self {
        self.tls_names.extend(names.into_iter().map(Into::into));
        self
    }

    /// Puts the container on a Docker network with a DNS name.
    ///
    /// testcontainers-rs has no API for network aliases. The DNS name of a
    /// container on a user-defined network is its container name. Thus
    /// [`AwsEmulator::request`] sets the container name to `alias`. A
    /// container name is unique on the Docker host, so use a unique alias.
    /// The module adds the alias to the server certificate.
    pub fn with_network_alias(
        mut self,
        network: impl Into<String>,
        alias: impl Into<String>,
    ) -> Self {
        self.network = Some((network.into(), alias.into()));
        self
    }

    /// Turns the telemetry of the emulator on or off. The default is off.
    ///
    /// When telemetry is off, the module sets `EMULATOR_TELEMETRY=off`.
    /// When telemetry is on, the module does not set the variable.
    pub fn with_telemetry(mut self, enabled: bool) -> Self {
        self.telemetry = enabled;
        self
    }

    /// Returns the names that the module puts in `EMULATOR_TLS_NAMES`.
    ///
    /// The list has the Testcontainers host, the network alias and the extra
    /// names, without duplicates. The list is never empty.
    pub fn tls_names(&self) -> Vec<String> {
        let mut names: Vec<String> = Vec::new();
        let candidates = expected_host()
            .into_iter()
            .chain(self.network.iter().map(|(_, alias)| alias.clone()))
            .chain(self.tls_names.iter().cloned());
        for name in candidates {
            let name = name.trim().to_string();
            if !name.is_empty() && !names.contains(&name) {
                names.push(name);
            }
        }
        if names.is_empty() {
            names.push("localhost".to_string());
        }
        names
    }

    /// Returns a container request with the startup timeout (120 s) and,
    /// when it is set, the network and the alias.
    ///
    /// Use this method when you want to add more settings with [`ImageExt`]
    /// before the start.
    pub fn request(self) -> ContainerRequest<Self> {
        let network = self.network.clone();
        let request = self.with_startup_timeout(STARTUP_TIMEOUT);
        match network {
            Some((network, alias)) => request.with_network(network).with_container_name(alias),
            None => request,
        }
    }

    /// Starts the container and reads the credentials and the CA.
    pub async fn run(self) -> Result<StartedAwsEmulator, Error> {
        let container = self.request().start().await?;
        StartedAwsEmulator::from_container(container).await
    }
}

impl Image for AwsEmulator {
    fn name(&self) -> &str {
        &self.name
    }

    fn tag(&self) -> &str {
        &self.tag
    }

    fn ready_conditions(&self) -> Vec<WaitFor> {
        // Do not use the Docker health check: its first run is after 10 s.
        vec![WaitFor::message_on_stderr(READY_MESSAGE)]
    }

    fn env_vars(
        &self,
    ) -> impl IntoIterator<Item = (impl Into<Cow<'_, str>>, impl Into<Cow<'_, str>>)> {
        let mut vars = vec![("EMULATOR_TLS_NAMES".to_string(), self.tls_names().join(","))];
        if !self.telemetry {
            vars.push(("EMULATOR_TELEMETRY".to_string(), "off".to_string()));
        }
        vars
    }

    fn cmd(&self) -> impl IntoIterator<Item = impl Into<Cow<'_, str>>> {
        ["-p", "9297"]
    }

    fn expose_ports(&self) -> &[ContainerPort] {
        &EXPOSED_PORTS
    }
}

/// A started AWS Emulator container with its endpoint, credentials and CA.
///
/// The container stops and Docker removes it when this value is dropped.
pub struct StartedAwsEmulator {
    container: ContainerAsync<AwsEmulator>,
    endpoint: String,
    access_key_id: String,
    secret_access_key: String,
    ca_pem: String,
    ca_file: tempfile::TempPath,
}

impl fmt::Debug for StartedAwsEmulator {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("StartedAwsEmulator")
            .field("container", &self.container.id())
            .field("endpoint", &self.endpoint)
            .field("access_key_id", &self.access_key_id)
            .finish_non_exhaustive()
    }
}

impl StartedAwsEmulator {
    /// Reads the endpoint, the credentials and the CA from a started container.
    ///
    /// The method copies the files out of the container with the Docker
    /// archive API. It does not run a command in the container.
    pub async fn from_container(container: ContainerAsync<AwsEmulator>) -> Result<Self, Error> {
        let host = match std::env::var(HOST_OVERRIDE_ENV) {
            Ok(host) if !host.trim().is_empty() => host.trim().to_string(),
            _ => container.get_host().await?.to_string(),
        };
        let port = container.get_host_port_ipv4(PORT).await?;
        let endpoint = format!("https://{}:{port}", url_host(&host));

        let credentials = container
            .copy_file_from(CREDENTIALS_FILE, Vec::new())
            .await?;
        let (access_key_id, secret_access_key) =
            parse_credentials(&String::from_utf8_lossy(&credentials))?;

        let ca = container.copy_file_from(CA_FILE, Vec::new()).await?;
        let ca_pem = String::from_utf8(ca)
            .map_err(|_| Error::InvalidFile(format!("{CA_FILE} is not UTF-8 text")))?;
        if !ca_pem.contains("-----BEGIN CERTIFICATE-----") {
            return Err(Error::InvalidFile(format!(
                "{CA_FILE} has no PEM certificate"
            )));
        }

        let mut file = tempfile::Builder::new()
            .prefix("aws-emulator-ca-")
            .suffix(".pem")
            .tempfile()?;
        file.write_all(ca_pem.as_bytes())?;
        file.flush()?;
        let ca_file = file.into_temp_path();

        Ok(Self {
            container,
            endpoint,
            access_key_id,
            secret_access_key,
            ca_pem,
            ca_file,
        })
    }

    /// Returns the container.
    pub fn container(&self) -> &ContainerAsync<AwsEmulator> {
        &self.container
    }

    /// Returns the endpoint URL, `https://<host>:<mapped port>`.
    pub fn endpoint(&self) -> &str {
        &self.endpoint
    }

    /// Returns the AWS Region, `us-east-1`.
    pub fn region(&self) -> &'static str {
        REGION
    }

    /// Returns the access key ID.
    pub fn access_key_id(&self) -> &str {
        &self.access_key_id
    }

    /// Returns the secret access key.
    pub fn secret_access_key(&self) -> &str {
        &self.secret_access_key
    }

    /// Returns the CA certificate as PEM text.
    pub fn ca_pem(&self) -> &str {
        &self.ca_pem
    }

    /// Returns the CA certificate as PEM bytes.
    ///
    /// Give these bytes to
    /// `aws_smithy_http_client::tls::TrustStore::with_pem_certificate` to
    /// trust only this CA in the AWS SDK for Rust.
    pub fn ca_pem_bytes(&self) -> Vec<u8> {
        self.ca_pem.as_bytes().to_vec()
    }

    /// Returns the path of a temporary file that contains the CA (PEM).
    ///
    /// The file exists until this value is dropped. Use it for
    /// `AWS_CA_BUNDLE`, `SSL_CERT_FILE` or `NODE_EXTRA_CA_CERTS`.
    pub fn ca_file(&self) -> &Path {
        &self.ca_file
    }

    /// Returns the environment variables for a child process or an SDK.
    ///
    /// The map has `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`,
    /// `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `AWS_DEFAULT_REGION`,
    /// `AWS_CA_BUNDLE` (the path of [`StartedAwsEmulator::ca_file`]) and
    /// `AWS_EC2_METADATA_DISABLED=true`.
    pub fn environment(&self) -> BTreeMap<String, String> {
        [
            ("AWS_ENDPOINT_URL", self.endpoint.clone()),
            ("AWS_ACCESS_KEY_ID", self.access_key_id.clone()),
            ("AWS_SECRET_ACCESS_KEY", self.secret_access_key.clone()),
            ("AWS_REGION", REGION.to_string()),
            ("AWS_DEFAULT_REGION", REGION.to_string()),
            ("AWS_CA_BUNDLE", self.ca_file.display().to_string()),
            ("AWS_EC2_METADATA_DISABLED", "true".to_string()),
        ]
        .into_iter()
        .map(|(key, value)| (key.to_string(), value))
        .collect()
    }

    /// Returns a rustls root store that contains only the CA.
    ///
    /// This method needs the `rustls` feature.
    #[cfg(feature = "rustls")]
    pub fn rustls_root_store(&self) -> Result<rustls::RootCertStore, Error> {
        use rustls_pki_types::CertificateDer;
        use rustls_pki_types::pem::PemObject;

        let mut store = rustls::RootCertStore::empty();
        for cert in CertificateDer::pem_slice_iter(self.ca_pem.as_bytes()) {
            let cert = cert.map_err(|e| Error::InvalidFile(format!("{CA_FILE}: {e}")))?;
            store
                .add(cert)
                .map_err(|e| Error::InvalidFile(format!("{CA_FILE}: {e}")))?;
        }
        Ok(store)
    }
}

/// An error from the AWS Emulator module.
#[derive(Debug)]
pub enum Error {
    /// Testcontainers or Docker returned an error.
    Testcontainers(TestcontainersError),
    /// A file from the container does not have the expected content.
    InvalidFile(String),
    /// The module cannot write the temporary CA file.
    Io(std::io::Error),
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Testcontainers(e) => write!(f, "testcontainers: {e}"),
            Error::InvalidFile(message) => write!(f, "invalid file: {message}"),
            Error::Io(e) => write!(f, "I/O: {e}"),
        }
    }
}

impl std::error::Error for Error {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Error::Testcontainers(e) => Some(e),
            Error::InvalidFile(_) => None,
            Error::Io(e) => Some(e),
        }
    }
}

impl From<TestcontainersError> for Error {
    fn from(e: TestcontainersError) -> Self {
        Error::Testcontainers(e)
    }
}

impl From<std::io::Error> for Error {
    fn from(e: std::io::Error) -> Self {
        Error::Io(e)
    }
}

/// Splits an image reference into the name and the tag for
/// [`Image::name`] and [`Image::tag`].
///
/// Testcontainers joins them as `name:tag`. For a digest reference
/// `repo@sha256:abc`, the name is `repo@sha256` and the tag is `abc`, so
/// the joined reference is correct again.
fn split_image(image: &str) -> (String, String) {
    if let Some((repo, digest)) = image.split_once('@')
        && let Some((algorithm, hex)) = digest.split_once(':')
    {
        return (format!("{repo}@{algorithm}"), hex.to_string());
    }
    let last_slash = image.rfind('/').map_or(0, |i| i + 1);
    match image[last_slash..].rfind(':') {
        Some(i) => (
            image[..last_slash + i].to_string(),
            image[last_slash + i + 1..].to_string(),
        ),
        None => (image.to_string(), "latest".to_string()),
    }
}

/// Returns the host that the test uses to reach the mapped port, when the
/// module can know it before the start.
///
/// The order is: `TESTCONTAINERS_HOST_OVERRIDE`, the host of a TCP
/// `DOCKER_HOST`, `localhost` for a local socket. testcontainers-rs reads
/// the Docker host also from `~/.testcontainers.properties`; the module does
/// not read that file.
fn expected_host() -> Option<String> {
    if let Ok(host) = std::env::var(HOST_OVERRIDE_ENV)
        && !host.trim().is_empty()
    {
        return Some(host.trim().to_string());
    }
    match std::env::var("DOCKER_HOST") {
        Ok(docker_host) if !docker_host.trim().is_empty() => host_of_docker_host(&docker_host),
        _ => Some("localhost".to_string()),
    }
}

fn host_of_docker_host(docker_host: &str) -> Option<String> {
    let (scheme, rest) = docker_host.split_once("://")?;
    match scheme {
        "tcp" | "http" | "https" => {
            let authority = rest.split('/').next().unwrap_or("");
            let host = if let Some(stripped) = authority.strip_prefix('[') {
                stripped.split(']').next().unwrap_or("")
            } else {
                authority.split(':').next().unwrap_or("")
            };
            (!host.is_empty()).then(|| host.to_string())
        }
        "unix" | "npipe" => Some("localhost".to_string()),
        _ => None,
    }
}

/// Puts an IPv6 address in brackets for use in a URL.
fn url_host(host: &str) -> Cow<'_, str> {
    if host.contains(':') && !host.starts_with('[') {
        Cow::Owned(format!("[{host}]"))
    } else {
        Cow::Borrowed(host)
    }
}

/// Reads the keys of the `[default]` profile from an INI credentials file.
fn parse_credentials(text: &str) -> Result<(String, String), Error> {
    let mut section = String::new();
    let mut key_id = None;
    let mut secret = None;
    for line in text.lines() {
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') || line.starts_with(';') {
            continue;
        }
        if let Some(name) = line.strip_prefix('[').and_then(|l| l.strip_suffix(']')) {
            section = name.trim().to_string();
            continue;
        }
        if section != "default" {
            continue;
        }
        if let Some((key, value)) = line.split_once('=') {
            match key.trim() {
                "aws_access_key_id" => key_id = Some(value.trim().to_string()),
                "aws_secret_access_key" => secret = Some(value.trim().to_string()),
                _ => {}
            }
        }
    }
    match (key_id, secret) {
        (Some(k), Some(s)) if !k.is_empty() && !s.is_empty() => Ok((k, s)),
        _ => Err(Error::InvalidFile(format!(
            "{CREDENTIALS_FILE} has no aws_access_key_id and aws_secret_access_key in [default]"
        ))),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn splits_image_references() {
        let cases = [
            (
                "ghcr.io/xsavikx/aws-emulator:latest",
                "ghcr.io/xsavikx/aws-emulator",
                "latest",
            ),
            ("aws-emulator:tls-al", "aws-emulator", "tls-al"),
            ("aws-emulator", "aws-emulator", "latest"),
            (
                "localhost:5000/aws-emulator",
                "localhost:5000/aws-emulator",
                "latest",
            ),
            (
                "localhost:5000/aws-emulator:1.0",
                "localhost:5000/aws-emulator",
                "1.0",
            ),
            ("repo/img@sha256:abcd", "repo/img@sha256", "abcd"),
        ];
        for (image, name, tag) in cases {
            assert_eq!(
                split_image(image),
                (name.to_string(), tag.to_string()),
                "{image}"
            );
        }
    }

    #[test]
    fn reads_docker_host() {
        assert_eq!(
            host_of_docker_host("tcp://10.0.0.5:2375").as_deref(),
            Some("10.0.0.5")
        );
        assert_eq!(
            host_of_docker_host("tcp://[::1]:2375").as_deref(),
            Some("::1")
        );
        assert_eq!(
            host_of_docker_host("unix:///var/run/docker.sock").as_deref(),
            Some("localhost")
        );
        assert_eq!(host_of_docker_host("ssh://user@host"), None);
    }

    #[test]
    fn parses_credentials() {
        let text = "[other]\naws_access_key_id = X\n[default]\naws_access_key_id = AKID\naws_secret_access_key = SECRET\n";
        let (k, s) = parse_credentials(text).unwrap();
        assert_eq!((k.as_str(), s.as_str()), ("AKID", "SECRET"));
        assert!(parse_credentials("[default]\n").is_err());
    }

    #[test]
    fn env_vars_have_tls_names_and_telemetry() {
        let image = AwsEmulator::new("x:y")
            .with_network_alias("net", "aws")
            .with_tls_names(["aws", "aws.test"]);
        let names = image.tls_names();
        assert!(names.contains(&"aws".to_string()));
        assert!(names.contains(&"aws.test".to_string()));
        assert_eq!(names.iter().filter(|n| *n == "aws").count(), 1);

        let vars: Vec<(String, String)> = image
            .env_vars()
            .into_iter()
            .map(|(k, v)| (k.into().into_owned(), v.into().into_owned()))
            .collect();
        assert!(
            vars.iter()
                .any(|(k, v)| k == "EMULATOR_TELEMETRY" && v == "off")
        );
        assert!(
            vars.iter()
                .any(|(k, v)| k == "EMULATOR_TLS_NAMES" && !v.is_empty())
        );

        let image = image.with_telemetry(true);
        let has_telemetry = image.env_vars().into_iter().any(|(k, _)| {
            let k: Cow<'_, str> = k.into();
            k == "EMULATOR_TELEMETRY"
        });
        assert!(!has_telemetry);
    }
}
