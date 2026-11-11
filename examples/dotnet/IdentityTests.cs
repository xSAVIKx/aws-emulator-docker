// Scenario "Identity" (STS): GetCallerIdentity returns the emulator account.

using Amazon.SecurityToken.Model;

namespace AwsEmulator.Examples;

public class IdentityTests(EmulatorFixture aws)
{
    [Fact]
    public async Task GetCallerIdentity_ReturnsEmulatorAccount()
    {
        var ct = TestContext.Current.CancellationToken;

        var identity = await aws.Sts.GetCallerIdentityAsync(new GetCallerIdentityRequest(), ct);

        Assert.Equal("000000000000", identity.Account);
    }
}
