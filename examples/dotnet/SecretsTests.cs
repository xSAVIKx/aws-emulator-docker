// Scenario "Secrets" (Secrets Manager): create a JSON secret, read it back, delete it,
// and check that a read after the delete fails.

using System.Text.Json;
using Amazon.SecretsManager;
using Amazon.SecretsManager.Model;

namespace AwsEmulator.Examples;

public class SecretsTests(EmulatorFixture aws)
{
    private sealed record DbCredentials(string Username, string Password);

    [Fact]
    public async Task Secret_CreateReadDelete()
    {
        var ct = TestContext.Current.CancellationToken;
        var name = EmulatorFixture.UniqueName();
        var expected = new DbCredentials("app", "s3cr3t");

        await aws.SecretsManager.CreateSecretAsync(new CreateSecretRequest
        {
            Name = name,
            SecretString = JsonSerializer.Serialize(expected),
        }, ct);
        var deleted = false;
        try
        {
            var value = await aws.SecretsManager.GetSecretValueAsync(new GetSecretValueRequest { SecretId = name }, ct);
            var actual = JsonSerializer.Deserialize<DbCredentials>(value.SecretString);
            Assert.Equal(expected, actual);

            // Delete at once. Without this flag, the secret waits 7 to 30 days for recovery.
            await aws.SecretsManager.DeleteSecretAsync(new DeleteSecretRequest
            {
                SecretId = name,
                ForceDeleteWithoutRecovery = true,
            }, ct);
            deleted = true;

            // A read after the delete fails. The emulator returns InvalidRequestException and
            // not ResourceNotFoundException, so the test checks only the common base type.
            await Assert.ThrowsAnyAsync<AmazonSecretsManagerException>(() =>
                aws.SecretsManager.GetSecretValueAsync(new GetSecretValueRequest { SecretId = name }, ct));
        }
        finally
        {
            // Clean up when an assertion failed before the delete.
            if (!deleted)
            {
                await aws.SecretsManager.DeleteSecretAsync(new DeleteSecretRequest
                {
                    SecretId = name,
                    ForceDeleteWithoutRecovery = true,
                }, CancellationToken.None);
            }
        }
    }
}
