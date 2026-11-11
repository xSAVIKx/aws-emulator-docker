// Shared set-up for all tests: the endpoint guard, the AWS clients and unique names.
//
// The AWS SDK for .NET v4 reads the standard environment and shared files:
//   AWS_ENDPOINT_URL                               -> the service endpoint (the emulator)
//   AWS_SHARED_CREDENTIALS_FILE, AWS_CONFIG_FILE   -> the credentials
//   AWS_REGION                                     -> the region
//   SSL_CERT_FILE                                  -> the trusted CAs (OpenSSL on Linux)
// This file therefore contains no endpoint, keys or certificate paths.

using Amazon.DynamoDBv2;
using Amazon.S3;
using Amazon.SecretsManager;
using Amazon.SecurityToken;
using Amazon.SimpleNotificationService;
using Amazon.SQS;

// One fixture instance for the whole test assembly (an xUnit v3 assembly fixture).
[assembly: AssemblyFixture(typeof(AwsEmulator.Examples.EmulatorFixture))]

namespace AwsEmulator.Examples;

public sealed class EmulatorFixture : IDisposable
{
    public EmulatorFixture()
    {
        // Stop before the first request if the endpoint is not the local emulator.
        // This guard prevents a run against a real AWS account.
        EnsureLocalEndpoint();

        // The clients take the endpoint from AWS_ENDPOINT_URL. We do not set ServiceURL.
        Sts = new AmazonSecurityTokenServiceClient();
        // The endpoint is an IP address, so S3 must use path-style addresses
        // (https://127.0.0.1:port/bucket/key) and not virtual-host addresses.
        S3 = new AmazonS3Client(new AmazonS3Config { ForcePathStyle = true });
        DynamoDb = new AmazonDynamoDBClient();
        Sqs = new AmazonSQSClient();
        Sns = new AmazonSimpleNotificationServiceClient();
        SecretsManager = new AmazonSecretsManagerClient();
    }

    public IAmazonSecurityTokenService Sts { get; }
    public IAmazonS3 S3 { get; }
    public IAmazonDynamoDB DynamoDb { get; }
    public IAmazonSQS Sqs { get; }
    public IAmazonSimpleNotificationService Sns { get; }
    public IAmazonSecretsManager SecretsManager { get; }

    /// <summary>
    /// Returns a name that is unique for this run, for example "ex-net-3f9a1c2b".
    /// The name is lowercase, so it is also a valid S3 bucket name.
    /// </summary>
    public static string UniqueName() => $"ex-net-{Guid.NewGuid():N}"[..15];

    private static void EnsureLocalEndpoint()
    {
        var value = Environment.GetEnvironmentVariable("AWS_ENDPOINT_URL");
        if (string.IsNullOrEmpty(value) || !Uri.TryCreate(value, UriKind.Absolute, out var uri))
        {
            throw new InvalidOperationException(
                "AWS_ENDPOINT_URL is not set. Start the emulator with examples/with-emulator.sh.");
        }

        if (uri.Host is not ("127.0.0.1" or "localhost"))
        {
            throw new InvalidOperationException(
                $"AWS_ENDPOINT_URL must point to 127.0.0.1 or localhost, not '{value}'. " +
                "The tests stop to prevent a run against a real AWS account.");
        }
    }

    public void Dispose()
    {
        Sts.Dispose();
        S3.Dispose();
        DynamoDb.Dispose();
        Sqs.Dispose();
        Sns.Dispose();
        SecretsManager.Dispose();
    }
}
