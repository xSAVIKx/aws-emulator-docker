// The configuration of an AWS Emulator container: the Testcontainers container
// configuration plus the extra TLS names and the telemetry setting.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using Docker.DotNet.Models;
using DotNet.Testcontainers.Builders;
using DotNet.Testcontainers.Configurations;

namespace AwsEmulator.Testcontainers;

/// <inheritdoc cref="ContainerConfiguration" />
public sealed class AwsEmulatorConfiguration : ContainerConfiguration
{
    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorConfiguration" /> class.
    /// </summary>
    /// <param name="extraTlsNames">More host names or IP addresses for the TLS certificate.</param>
    /// <param name="telemetry">True to turn the emulator telemetry on. Null or false keeps it off.</param>
    public AwsEmulatorConfiguration(IEnumerable<string>? extraTlsNames = null, bool? telemetry = null)
    {
        ExtraTlsNames = extraTlsNames ?? [];
        Telemetry = telemetry;
    }

    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorConfiguration" /> class.
    /// </summary>
    /// <param name="resourceConfiguration">The Docker resource configuration.</param>
    public AwsEmulatorConfiguration(IResourceConfiguration<CreateContainerParameters> resourceConfiguration)
        : base(resourceConfiguration)
    {
        ExtraTlsNames = [];
    }

    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorConfiguration" /> class.
    /// </summary>
    /// <param name="resourceConfiguration">The Docker resource configuration.</param>
    public AwsEmulatorConfiguration(IContainerConfiguration resourceConfiguration)
        : base(resourceConfiguration)
    {
        ExtraTlsNames = [];
    }

    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorConfiguration" /> class.
    /// </summary>
    /// <param name="resourceConfiguration">The Docker resource configuration.</param>
    public AwsEmulatorConfiguration(AwsEmulatorConfiguration resourceConfiguration)
        : this(new AwsEmulatorConfiguration(), resourceConfiguration)
    {
    }

    /// <summary>
    /// Initializes a new instance of the <see cref="AwsEmulatorConfiguration" /> class.
    /// </summary>
    /// <param name="oldValue">The old Docker resource configuration.</param>
    /// <param name="newValue">The new Docker resource configuration.</param>
    public AwsEmulatorConfiguration(AwsEmulatorConfiguration oldValue, AwsEmulatorConfiguration newValue)
        : base(oldValue, newValue)
    {
        ExtraTlsNames = BuildConfiguration.Combine(oldValue.ExtraTlsNames, newValue.ExtraTlsNames);
        Telemetry = BuildConfiguration.Combine(oldValue.Telemetry, newValue.Telemetry);
    }

    /// <summary>
    /// Gets the host names and IP addresses that the user adds to the TLS certificate.
    /// </summary>
    public IEnumerable<string> ExtraTlsNames { get; }

    /// <summary>
    /// Gets the telemetry setting. True turns telemetry on. Null or false keeps it off.
    /// </summary>
    public bool? Telemetry { get; }
}
