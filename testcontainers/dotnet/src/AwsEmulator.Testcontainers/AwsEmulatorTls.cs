// TLS helpers that trust only one CA certificate. They do not use the system CAs.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using System.Net.Http;
using System.Net.Security;
using System.Security.Cryptography.X509Certificates;

namespace AwsEmulator.Testcontainers;

/// <summary>
/// Validates a server certificate against one CA certificate only.
/// </summary>
public static class AwsEmulatorTls
{
    /// <summary>
    /// Returns a <see cref="RemoteCertificateValidationCallback" /> that accepts a server
    /// certificate only if <paramref name="caCertificate" /> signs it and the host name
    /// matches.
    /// </summary>
    /// <param name="caCertificate">The CA certificate to trust.</param>
    /// <returns>The callback.</returns>
    public static RemoteCertificateValidationCallback CreateValidationCallback(X509Certificate2 caCertificate)
    {
        ArgumentNullException.ThrowIfNull(caCertificate);
        return (_, certificate, _, errors) => Validate(caCertificate, certificate, errors);
    }

    /// <summary>
    /// Returns a new <see cref="SocketsHttpHandler" /> that trusts only
    /// <paramref name="caCertificate" />.
    /// </summary>
    /// <param name="caCertificate">The CA certificate to trust.</param>
    /// <returns>A new handler. The caller disposes it.</returns>
    public static HttpMessageHandler CreateHttpMessageHandler(X509Certificate2 caCertificate)
    {
        return new SocketsHttpHandler
        {
            SslOptions = new SslClientAuthenticationOptions
            {
                RemoteCertificateValidationCallback = CreateValidationCallback(caCertificate),
            },
        };
    }

    /// <summary>
    /// Returns true if <paramref name="caCertificate" /> signs <paramref name="certificate" />
    /// and the platform found no name error.
    /// </summary>
    /// <param name="caCertificate">The CA certificate to trust.</param>
    /// <param name="certificate">The server certificate.</param>
    /// <param name="sslPolicyErrors">The errors that the platform found.</param>
    /// <returns>True if the certificate is valid.</returns>
    public static bool Validate(X509Certificate2 caCertificate, X509Certificate? certificate, SslPolicyErrors sslPolicyErrors)
    {
        ArgumentNullException.ThrowIfNull(caCertificate);
        if (certificate == null)
        {
            return false;
        }

        // The platform checks the host name. A name error or a missing certificate is
        // final. A chain error is expected, because the system does not know the CA.
        if ((sslPolicyErrors & (SslPolicyErrors.RemoteCertificateNameMismatch | SslPolicyErrors.RemoteCertificateNotAvailable)) != 0)
        {
            return false;
        }

        using var serverCertificate = new X509Certificate2(certificate);
        using var chain = new X509Chain();
        chain.ChainPolicy.TrustMode = X509ChainTrustMode.CustomRootTrust;
        chain.ChainPolicy.CustomTrustStore.Add(caCertificate);
        chain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
        return chain.Build(serverCertificate);
    }
}
