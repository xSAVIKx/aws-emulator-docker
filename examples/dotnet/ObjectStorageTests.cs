// Scenario "Object storage" (S3): create a bucket, put an object, list and get it,
// then delete both.

using Amazon.S3.Model;

namespace AwsEmulator.Examples;

public class ObjectStorageTests(EmulatorFixture aws)
{
    [Fact]
    public async Task Bucket_PutListGetObject()
    {
        var ct = TestContext.Current.CancellationToken;
        var bucket = EmulatorFixture.UniqueName();
        const string key = "greeting.txt";
        const string body = "Hello from .NET";

        await aws.S3.PutBucketAsync(new PutBucketRequest { BucketName = bucket }, ct);
        try
        {
            await aws.S3.PutObjectAsync(new PutObjectRequest
            {
                BucketName = bucket,
                Key = key,
                ContentBody = body,
                ContentType = "text/plain",
            }, ct);

            // The list contains exactly the one object.
            var list = await aws.S3.ListObjectsV2Async(new ListObjectsV2Request { BucketName = bucket }, ct);
            var item = Assert.Single(list.S3Objects);
            Assert.Equal(key, item.Key);

            // The object body is the text that we put.
            using var get = await aws.S3.GetObjectAsync(new GetObjectRequest { BucketName = bucket, Key = key }, ct);
            using var reader = new StreamReader(get.ResponseStream);
            Assert.Equal(body, await reader.ReadToEndAsync(ct));
        }
        finally
        {
            // Clean up also when an assertion fails. S3 deletes only an empty bucket.
            await EmulatorFixture.CleanUpAsync("DeleteObject", () =>
                aws.S3.DeleteObjectAsync(new DeleteObjectRequest { BucketName = bucket, Key = key }, CancellationToken.None));
            await EmulatorFixture.CleanUpAsync("DeleteBucket", () =>
                aws.S3.DeleteBucketAsync(new DeleteBucketRequest { BucketName = bucket }, CancellationToken.None));
        }
    }
}
