// The module API: the endpoint, the trust helper, the CA file and the environment.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using System.Net.Security;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;

namespace AwsEmulator.Testcontainers.Tests;

public class TlsTests(EmulatorFixture aws)
{
    [Fact]
    public async Task HttpClient_TrustsEmulatorCa()
    {
        var ct = TestContext.Current.CancellationToken;
        using var client = aws.Container.CreateHttpClient();

        var body = await client.GetStringAsync("__admin/health", ct);

        Assert.Equal("ok", body.Trim());
    }

    [Fact]
    public void Validate_RejectsCertificateFromOtherCa()
    {
        using var key = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        var request = new CertificateRequest("CN=localhost", key, HashAlgorithmName.SHA256);
        using var other = request.CreateSelfSigned(DateTimeOffset.UtcNow.AddMinutes(-5), DateTimeOffset.UtcNow.AddHours(1));

        Assert.False(aws.Container.ValidateServerCertificate(null, other, null, SslPolicyErrors.None));
    }

    [Fact]
    public void Environment_HasAllVariables()
    {
        var env = aws.Container.GetEnvironmentVariables();

        Assert.StartsWith("https://", env["AWS_ENDPOINT_URL"]);
        Assert.Equal(aws.Container.AccessKeyId, env["AWS_ACCESS_KEY_ID"]);
        Assert.Equal(aws.Container.SecretAccessKey, env["AWS_SECRET_ACCESS_KEY"]);
        Assert.Equal("us-east-1", env["AWS_REGION"]);
        Assert.Equal("us-east-1", env["AWS_DEFAULT_REGION"]);
        Assert.Equal("true", env["AWS_EC2_METADATA_DISABLED"]);
        Assert.Equal(aws.Container.CaCertificatePem, File.ReadAllText(env["AWS_CA_BUNDLE"]));
    }

    [Fact]
    public void TlsNames_HaveAliasesAndExtraNames()
    {
        var names = new AwsEmulatorBuilder()
            .WithNetworkAliases("alias1")
            .WithTlsNames("a.test")
            .WithTlsNames("b.test", "a.test")
            .GetTlsNames();

        Assert.Contains("alias1", names);
        Assert.Contains("a.test", names);
        Assert.Contains("b.test", names);
        Assert.Equal(names.Count, names.Distinct(StringComparer.OrdinalIgnoreCase).Count());
    }

    [Fact]
    public void TlsNames_RejectInvalidName()
    {
        Assert.Throws<ArgumentException>(() => new AwsEmulatorBuilder().WithTlsNames("a,b"));
    }

    [Fact]
    public void Credentials_ParseDefaultSection()
    {
        var values = AwsEmulatorContainer.ParseCredentials("[other]\naws_access_key_id = x\n[default]\naws_access_key_id = AKID\naws_secret_access_key = SECRET\n");

        Assert.Equal("AKID", values["aws_access_key_id"]);
        Assert.Equal("SECRET", values["aws_secret_access_key"]);
    }
}
