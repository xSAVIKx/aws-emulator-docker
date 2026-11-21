// A started AWS Emulator container: the endpoint, the credentials, the CA and the
// helpers that trust only that CA.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using System.Globalization;
using System.Net.Http;
using System.Net.Security;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using DotNet.Testcontainers.Containers;

namespace AwsEmulator.Testcontainers;

/// <summary>
/// An AWS Emulator container. Create it with <see cref="AwsEmulatorBuilder" />.
/// After <see cref="DockerContainer.StartAsync" />, the container has the credentials
/// and the CA certificate.
/// </summary>
public sealed class AwsEmulatorContainer : DockerContainer
{
    /// <summary>The AWS account ID of the emulator.</summary>
    public const string AccountId = "000000000000";

    /// <summary>The AWS region of the emulator.</summary>
    public const string Region = "us-east-1";

    /// <summary>The folder in the container where the emulator writes its files.</summary>
    public const string RunDirectory = "/home/emulator/.aws/emulator/9297";

    /// <summary>The credentials file in the container.</summary>
    public const string CredentialsPath = RunDirectory + "/emulator_credentials";

    /// <summary>The CA certificate file (PEM) in the container.</summary>
    public const string CaCertificatePath = RunDirectory + "/tls-ca.pem";

    private string? _accessKeyId;
    private string? _secretAccessKey;
    private string? _caCertificatePem;
    private X509Certificate2? _caCertificate;
    private string? _caBundlePath;

    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorContainer" /> class.
    /// </summary>
    /// <param name="configuration">The container configuration.</param>
    public AwsEmulatorContainer(AwsEmulatorConfiguration configuration)
        : base(configuration)
    {
    }

    /// <summary>Gets the access key ID. The container must be started.</summary>
    public string AccessKeyId => _accessKeyId ?? throw NotStarted();

    /// <summary>Gets the secret access key. The container must be started.</summary>
    public string SecretAccessKey => _secretAccessKey ?? throw NotStarted();

    /// <summary>Gets the CA certificate in PEM format. The container must be started.</summary>
    public string CaCertificatePem => _caCertificatePem ?? throw NotStarted();

    /// <summary>
    /// Gets the CA certificate that signs the server certificate. The container must be
    /// started.
    /// </summary>
    public X509Certificate2 CaCertificate => _caCertificate ?? throw NotStarted();

    /// <summary>
    /// Returns the endpoint URL, for example <c>https://127.0.0.1:32768</c>.
    /// </summary>
    /// <returns>The endpoint URL, without a trailing slash.</returns>
    public string GetEndpointUrl()
    {
        var port = GetMappedPublicPort(AwsEmulatorBuilder.EmulatorPort);
        return new UriBuilder(Uri.UriSchemeHttps, Hostname, port).Uri.ToString().TrimEnd('/');
    }

    /// <summary>Returns the endpoint URL. This is the same as <see cref="GetEndpointUrl" />.</summary>
    /// <returns>The endpoint URL.</returns>
    public string GetConnectionString() => GetEndpointUrl();

    /// <summary>
    /// Writes the CA certificate to a temporary file and returns its path. The method
    /// writes the file one time. The container deletes the file when you dispose it.
    /// Use the path for <c>AWS_CA_BUNDLE</c>, <c>SSL_CERT_FILE</c> or
    /// <c>NODE_EXTRA_CA_CERTS</c>.
    /// </summary>
    /// <returns>The path of the PEM file.</returns>
    public string WriteCaCertificateToTempFile()
    {
        var pem = CaCertificatePem;
        lock (this)
        {
            if (_caBundlePath == null)
            {
                var path = Path.Combine(Path.GetTempPath(), $"aws-emulator-ca-{Guid.NewGuid():N}.pem");
                File.WriteAllText(path, pem, new UTF8Encoding(false));
                _caBundlePath = path;
            }

            return _caBundlePath;
        }
    }

    /// <summary>
    /// Returns the environment variables for a child process or an AWS SDK:
    /// <c>AWS_ENDPOINT_URL</c>, <c>AWS_ACCESS_KEY_ID</c>, <c>AWS_SECRET_ACCESS_KEY</c>,
    /// <c>AWS_REGION</c>, <c>AWS_DEFAULT_REGION</c>, <c>AWS_CA_BUNDLE</c> (a temporary
    /// file, see <see cref="WriteCaCertificateToTempFile" />) and
    /// <c>AWS_EC2_METADATA_DISABLED=true</c>.
    /// </summary>
    /// <returns>The variables.</returns>
    public IReadOnlyDictionary<string, string> GetEnvironmentVariables()
    {
        return new Dictionary<string, string>
        {
            ["AWS_ENDPOINT_URL"] = GetEndpointUrl(),
            ["AWS_ACCESS_KEY_ID"] = AccessKeyId,
            ["AWS_SECRET_ACCESS_KEY"] = SecretAccessKey,
            ["AWS_REGION"] = Region,
            ["AWS_DEFAULT_REGION"] = Region,
            ["AWS_CA_BUNDLE"] = WriteCaCertificateToTempFile(),
            ["AWS_EC2_METADATA_DISABLED"] = "true",
        };
    }

    /// <summary>
    /// Validates a server certificate. The method accepts the certificate only if the
    /// emulator CA signs it and the host name matches. It does not use the system CAs.
    /// Use it as <see cref="RemoteCertificateValidationCallback" />.
    /// </summary>
    /// <param name="sender">Not used.</param>
    /// <param name="certificate">The server certificate.</param>
    /// <param name="chain">The chain that the platform built. Not used for trust.</param>
    /// <param name="sslPolicyErrors">The errors that the platform found.</param>
    /// <returns>True if the CA signs the certificate and the name matches.</returns>
    public bool ValidateServerCertificate(object? sender, X509Certificate? certificate, X509Chain? chain, SslPolicyErrors sslPolicyErrors)
    {
        return AwsEmulatorTls.Validate(CaCertificate, certificate, sslPolicyErrors);
    }

    /// <summary>
    /// Returns a new <see cref="HttpMessageHandler" /> that trusts only the emulator CA.
    /// Give it to an <see cref="HttpClient" />, for example from an AWS SDK
    /// <c>HttpClientFactory</c>.
    /// </summary>
    /// <returns>A new handler. The caller disposes it.</returns>
    public HttpMessageHandler CreateHttpMessageHandler()
    {
        return AwsEmulatorTls.CreateHttpMessageHandler(CaCertificate);
    }

    /// <summary>
    /// Returns a new <see cref="HttpClient" /> with a handler from
    /// <see cref="CreateHttpMessageHandler" /> and the endpoint as the base address.
    /// </summary>
    /// <returns>A new client. The caller disposes it.</returns>
    public HttpClient CreateHttpClient()
    {
        return new HttpClient(CreateHttpMessageHandler(), disposeHandler: true)
        {
            BaseAddress = new Uri(GetEndpointUrl() + "/"),
        };
    }

    /// <inheritdoc />
    public override async Task StartAsync(CancellationToken ct = default)
    {
        await base.StartAsync(ct).ConfigureAwait(false);

        // The docker archive API copies the files. The distroless image has no shell,
        // so the module does not use exec.
        var credentials = Encoding.UTF8.GetString(await ReadFileAsync(CredentialsPath, ct).ConfigureAwait(false));
        var pem = Encoding.UTF8.GetString(await ReadFileAsync(CaCertificatePath, ct).ConfigureAwait(false));

        var values = ParseCredentials(credentials);
        _accessKeyId = values.TryGetValue("aws_access_key_id", out var key) && key.Length > 0
            ? key
            : throw new InvalidOperationException($"{CredentialsPath} has no aws_access_key_id.");
        _secretAccessKey = values.TryGetValue("aws_secret_access_key", out var secret) && secret.Length > 0
            ? secret
            : throw new InvalidOperationException($"{CredentialsPath} has no aws_secret_access_key.");
        _caCertificate = X509Certificate2.CreateFromPem(pem);
        _caCertificatePem = pem;
    }

    /// <inheritdoc />
    protected override async ValueTask DisposeAsyncCore()
    {
        var path = _caBundlePath;
        if (path != null)
        {
            try
            {
                File.Delete(path);
            }
            catch (IOException)
            {
                // The file stays in the temporary folder. This is not an error.
            }
            catch (UnauthorizedAccessException)
            {
                // The same as above.
            }
        }

        _caCertificate?.Dispose();
        await base.DisposeAsyncCore().ConfigureAwait(false);
    }

    /// <summary>
    /// Reads the keys of the <c>[default]</c> section of an INI credentials file.
    /// </summary>
    internal static Dictionary<string, string> ParseCredentials(string text)
    {
        var result = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
        string? section = null;
        foreach (var raw in text.Split('\n'))
        {
            var line = raw.Trim();
            if (line.Length == 0 || line[0] is '#' or ';')
            {
                continue;
            }

            if (line[0] == '[' && line[^1] == ']')
            {
                section = line[1..^1].Trim();
                continue;
            }

            var index = line.IndexOf('=', StringComparison.Ordinal);
            if (index > 0 && string.Equals(section, "default", StringComparison.Ordinal))
            {
                result[line[..index].Trim()] = line[(index + 1)..].Trim();
            }
        }

        return result;
    }

    private static InvalidOperationException NotStarted()
    {
        return new InvalidOperationException(string.Format(
            CultureInfo.InvariantCulture,
            "The AWS Emulator container is not started. Call {0} first.",
            nameof(StartAsync)));
    }
}
