// Network alias: a second container on the same network calls https://aws:9297 and
// verifies the certificate with the emulator CA.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using System.Text;
using DotNet.Testcontainers.Builders;

namespace AwsEmulator.Testcontainers.Tests;

public class NetworkAliasTests(EmulatorFixture aws)
{
    private const string CurlImage =
        "curlimages/curl:8.23.0@sha256:3fb7e13e5d179a845c97d78204f3f1064d1907037f26a3ebf91710571151fd93";

    [Fact]
    public async Task Alias_IsInCertificate()
    {
        var ct = TestContext.Current.CancellationToken;

        await using var curl = new ContainerBuilder(CurlImage)
            .WithNetwork(aws.Network)
            .WithResourceMapping(Encoding.UTF8.GetBytes(aws.Container.CaCertificatePem), FilePath.Of("/ca.pem"))
            .WithCommand("-sf", "--cacert", "/ca.pem", $"https://{EmulatorFixture.Alias}:{AwsEmulatorBuilder.EmulatorPort}/__admin/health")
            .Build();
        await curl.StartAsync(ct);

        var exitCode = await curl.GetExitCodeAsync(ct);
        var (stdout, stderr) = await curl.GetLogsAsync(timestampsEnabled: false, ct: ct);

        Assert.True(exitCode == 0, $"curl exit code {exitCode}: {stderr}");
        Assert.Equal("ok", stdout.Trim());
    }
}
