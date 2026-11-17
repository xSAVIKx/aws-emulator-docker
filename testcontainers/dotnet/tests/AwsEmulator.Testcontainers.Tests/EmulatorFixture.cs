// One AWS Emulator container for all tests, on a Testcontainers network with the
// alias "aws". The AWS clients use a custom HttpClientFactory that trusts only the
// emulator CA.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using Amazon.Runtime;
using Amazon.S3;
using Amazon.SecurityToken;
using Amazon.SQS;
using DotNet.Testcontainers.Builders;
using DotNet.Testcontainers.Networks;

[assembly: AssemblyFixture(typeof(AwsEmulator.Testcontainers.Tests.EmulatorFixture))]

namespace AwsEmulator.Testcontainers.Tests;

public sealed class EmulatorFixture : IAsyncLifetime
{
    /// <summary>The network alias of the emulator.</summary>
    public const string Alias = "aws";

    /// <summary>
    /// The image under test. AWS_EMULATOR_IMAGE overrides the default image, so CI can
    /// test a locally built image.
    /// </summary>
    public static string Image =>
        Environment.GetEnvironmentVariable("AWS_EMULATOR_IMAGE") is { Length: > 0 } image
            ? image
            : AwsEmulatorBuilder.DefaultImage;

    public INetwork Network { get; } = new NetworkBuilder().Build();

    public AwsEmulatorContainer Container { get; private set; } = null!;

    public IAmazonSecurityTokenService Sts { get; private set; } = null!;

    public IAmazonS3 S3 { get; private set; } = null!;

    public IAmazonSQS Sqs { get; private set; } = null!;

    public async ValueTask InitializeAsync()
    {
        await Network.CreateAsync();
        Container = new AwsEmulatorBuilder(Image)
            .WithNetwork(Network)
            .WithNetworkAliases(Alias)
            .Build();
        await Container.StartAsync();

        var credentials = new BasicAWSCredentials(Container.AccessKeyId, Container.SecretAccessKey);
        Sts = new AmazonSecurityTokenServiceClient(credentials, Configure(new AmazonSecurityTokenServiceConfig()));
        // Path-style addresses: https://host:port/bucket/key. The endpoint host can be
        // an IP address, so virtual-host addresses do not work.
        S3 = new AmazonS3Client(credentials, Configure(new AmazonS3Config { ForcePathStyle = true }));
        Sqs = new AmazonSQSClient(credentials, Configure(new AmazonSQSConfig()));
    }

    /// <summary>
    /// Sets the endpoint, the region and the HTTP client factory of an AWS client
    /// configuration.
    /// </summary>
    private T Configure<T>(T config)
        where T : ClientConfig
    {
        config.ServiceURL = Container.GetEndpointUrl();
        config.AuthenticationRegion = AwsEmulatorContainer.Region;
        config.HttpClientFactory = new EmulatorHttpClientFactory(Container);
        return config;
    }

    public static string UniqueName() => $"tc-net-{Guid.NewGuid():N}"[..15];

    public async ValueTask DisposeAsync()
    {
        Sts?.Dispose();
        S3?.Dispose();
        Sqs?.Dispose();
        if (Container != null)
        {
            await Container.DisposeAsync();
        }

        await Network.DisposeAsync();
    }
}

/// <summary>
/// Gives the AWS SDK an <see cref="HttpClient" /> that trusts only the emulator CA.
/// </summary>
public sealed class EmulatorHttpClientFactory(AwsEmulatorContainer container) : HttpClientFactory
{
    public override HttpClient CreateHttpClient(IClientConfig clientConfig)
    {
        return new HttpClient(container.CreateHttpMessageHandler(), disposeHandler: true);
    }
}
