// The Testcontainers builder for the AWS Emulator image with the TLS front end.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using Docker.DotNet.Models;
using DotNet.Testcontainers.Builders;
using DotNet.Testcontainers.Configurations;
using DotNet.Testcontainers.Images;

namespace AwsEmulator.Testcontainers;

/// <summary>
/// Builds an <see cref="AwsEmulatorContainer" />. The container serves HTTPS on port
/// <see cref="EmulatorPort" /> with a certificate that a per-container CA signs.
/// </summary>
/// <remarks>
/// The builder sets <c>EMULATOR_TLS_NAMES</c> to the Testcontainers host (when the
/// builder can know it before the start), the network aliases of the container and the
/// names from <see cref="WithTlsNames" />. It waits for the log line
/// <c>TLS front end ready</c>.
/// </remarks>
public sealed class AwsEmulatorBuilder
    : ContainerBuilder<AwsEmulatorBuilder, AwsEmulatorContainer, AwsEmulatorConfiguration>
{
    /// <summary>The default image.</summary>
    public const string DefaultImage = "ghcr.io/xsavikx/aws-emulator:latest";

    /// <summary>The port of the emulator in the container (HTTPS).</summary>
    public const ushort EmulatorPort = 9297;

    /// <summary>The log line that tells that the TLS front end is ready.</summary>
    public const string ReadyMessage = "TLS front end ready";

    /// <summary>The maximum time for the start of the container.</summary>
    public static readonly TimeSpan StartupTimeout = TimeSpan.FromSeconds(120);

    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorBuilder" /> class with
    /// the image <see cref="DefaultImage" />.
    /// </summary>
    public AwsEmulatorBuilder()
        : this(DefaultImage)
    {
    }

    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorBuilder" /> class.
    /// </summary>
    /// <param name="image">The full image name, for example <c>ghcr.io/xsavikx/aws-emulator:latest</c>.</param>
    public AwsEmulatorBuilder(string image)
        : this(new DockerImage(image))
    {
    }

    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorBuilder" /> class.
    /// </summary>
    /// <param name="image">The image.</param>
    public AwsEmulatorBuilder(IImage image)
        : this(new AwsEmulatorConfiguration())
    {
        DockerResourceConfiguration = Init().WithImage(image).DockerResourceConfiguration;
    }

    private AwsEmulatorBuilder(AwsEmulatorConfiguration resourceConfiguration)
        : base(resourceConfiguration)
    {
        DockerResourceConfiguration = resourceConfiguration;
    }

    /// <inheritdoc />
    protected override AwsEmulatorConfiguration DockerResourceConfiguration { get; }

    /// <summary>
    /// Adds host names or IP addresses to the TLS certificate. Use this option for a
    /// name that the builder does not know, for example a remote Docker host name.
    /// </summary>
    /// <param name="names">The host names or IP addresses. A name can be a wildcard, for example <c>*.example.test</c>.</param>
    /// <returns>A configured instance of <see cref="AwsEmulatorBuilder" />.</returns>
    public AwsEmulatorBuilder WithTlsNames(params string[] names)
    {
        foreach (var name in names)
        {
            if (string.IsNullOrWhiteSpace(name) || name.Any(c => c == ',' || char.IsWhiteSpace(c)))
            {
                throw new ArgumentException($"'{name}' is not a valid TLS name.", nameof(names));
            }
        }

        return Merge(DockerResourceConfiguration, new AwsEmulatorConfiguration(extraTlsNames: names.ToArray()));
    }

    /// <summary>
    /// Turns the emulator telemetry on or off. The default is off
    /// (<c>EMULATOR_TELEMETRY=off</c>).
    /// </summary>
    /// <param name="enabled">True to turn telemetry on.</param>
    /// <returns>A configured instance of <see cref="AwsEmulatorBuilder" />.</returns>
    public AwsEmulatorBuilder WithTelemetry(bool enabled)
    {
        return Merge(DockerResourceConfiguration, new AwsEmulatorConfiguration(telemetry: enabled));
    }

    /// <inheritdoc />
    public override AwsEmulatorContainer Build()
    {
        Validate();

        var builder = WithEnvironment("EMULATOR_TLS_NAMES", string.Join(",", GetTlsNames(DockerResourceConfiguration)));
        if (DockerResourceConfiguration.Telemetry != true
            && !DockerResourceConfiguration.Environments.ContainsKey("EMULATOR_TELEMETRY"))
        {
            builder = builder.WithEnvironment("EMULATOR_TELEMETRY", "off");
        }

        return new AwsEmulatorContainer(builder.DockerResourceConfiguration);
    }

    /// <summary>Returns the names for <c>EMULATOR_TLS_NAMES</c> of this builder.</summary>
    internal IReadOnlyList<string> GetTlsNames() => GetTlsNames(DockerResourceConfiguration);

    /// <summary>
    /// Returns the names for <c>EMULATOR_TLS_NAMES</c>: the Testcontainers host (if it
    /// is known before the start), the network aliases and the extra names. The list
    /// is never empty.
    /// </summary>
    /// <param name="configuration">The container configuration.</param>
    /// <returns>The names, without duplicates.</returns>
    internal static IReadOnlyList<string> GetTlsNames(AwsEmulatorConfiguration configuration)
    {
        var names = new List<string>();
        var host = GetKnownHost(configuration);
        if (host != null)
        {
            names.Add(host);
        }

        names.AddRange(configuration.NetworkAliases ?? []);
        names.AddRange(configuration.ExtraTlsNames);

        var result = names
            .Where(name => !string.IsNullOrWhiteSpace(name))
            .Select(name => name.Trim())
            .Distinct(StringComparer.OrdinalIgnoreCase)
            .ToList();
        if (result.Count == 0)
        {
            result.Add("localhost");
        }

        return result;
    }

    /// <summary>
    /// Returns the host that <c>DockerContainer.Hostname</c> returns after the start,
    /// or null if the builder cannot know it before the start.
    /// </summary>
    private static string? GetKnownHost(AwsEmulatorConfiguration configuration)
    {
        if (!string.IsNullOrEmpty(TestcontainersSettings.DockerHostOverride))
        {
            return TestcontainersSettings.DockerHostOverride;
        }

        var endpoint = configuration.DockerEndpointAuthConfig?.Endpoint;
        if (endpoint == null)
        {
            return null;
        }

        switch (endpoint.Scheme)
        {
            case "http":
            case "https":
            case "tcp":
                return endpoint.Host;
            case "unix":
            case "npipe":
                // Inside a container, Testcontainers uses the gateway of the container
                // network. The gateway is not known before the start.
                return IsRunningInsideDocker() ? null : "127.0.0.1";
            default:
                return null;
        }
    }

    private static bool IsRunningInsideDocker()
    {
        var root = Path.GetPathRoot(Directory.GetCurrentDirectory()) ?? "/";
        return File.Exists(Path.Combine(root, ".dockerenv"));
    }

    /// <inheritdoc />
    protected override AwsEmulatorBuilder Init()
    {
        return base.Init()
            .WithPortBinding(EmulatorPort, true)
            .WithCommand("-p", EmulatorPort.ToString(System.Globalization.CultureInfo.InvariantCulture))
            .WithWaitStrategy(Wait.ForUnixContainer().UntilMessageIsLogged(
                ReadyMessage, strategy => strategy.WithTimeout(StartupTimeout)));
    }

    /// <inheritdoc />
    protected override AwsEmulatorBuilder Clone(IResourceConfiguration<CreateContainerParameters> resourceConfiguration)
    {
        return Merge(DockerResourceConfiguration, new AwsEmulatorConfiguration(resourceConfiguration));
    }

    /// <inheritdoc />
    protected override AwsEmulatorBuilder Clone(IContainerConfiguration resourceConfiguration)
    {
        return Merge(DockerResourceConfiguration, new AwsEmulatorConfiguration(resourceConfiguration));
    }

    /// <inheritdoc />
    protected override AwsEmulatorBuilder Merge(AwsEmulatorConfiguration oldValue, AwsEmulatorConfiguration newValue)
    {
        return new AwsEmulatorBuilder(new AwsEmulatorConfiguration(oldValue, newValue));
    }
}
