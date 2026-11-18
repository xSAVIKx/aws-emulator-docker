// S3: create a bucket, put an object, get the object (path-style addresses).
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

using Amazon.S3.Model;

namespace AwsEmulator.Testcontainers.Tests;

public class ObjectStorageTests(EmulatorFixture aws)
{
    [Fact]
    public async Task Bucket_PutAndGetObject()
    {
        var ct = TestContext.Current.CancellationToken;
        var bucket = EmulatorFixture.UniqueName();

        await aws.S3.PutBucketAsync(new PutBucketRequest { BucketName = bucket }, ct);
        await aws.S3.PutObjectAsync(new PutObjectRequest
        {
            BucketName = bucket,
            Key = "hello.txt",
            ContentBody = "hello from .NET",
        }, ct);

        using var response = await aws.S3.GetObjectAsync(bucket, "hello.txt", ct);
        using var reader = new StreamReader(response.ResponseStream);
        Assert.Equal("hello from .NET", await reader.ReadToEndAsync(ct));
    }
}
