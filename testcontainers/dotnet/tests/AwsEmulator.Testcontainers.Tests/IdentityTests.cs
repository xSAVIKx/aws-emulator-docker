// STS: GetCallerIdentity returns the emulator account.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using Amazon.SecurityToken.Model;

namespace AwsEmulator.Testcontainers.Tests;

public class IdentityTests(EmulatorFixture aws)
{
    [Fact]
    public async Task GetCallerIdentity_ReturnsEmulatorAccount()
    {
        var ct = TestContext.Current.CancellationToken;

        var identity = await aws.Sts.GetCallerIdentityAsync(new GetCallerIdentityRequest(), ct);

        Assert.Equal(AwsEmulatorContainer.AccountId, identity.Account);
    }
}
