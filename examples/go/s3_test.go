package example

import (
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

// TestObjectStorage creates a bucket, puts an object, lists and gets it, and
// deletes both.
func TestObjectStorage(t *testing.T) {
	ctx := t.Context()
	// The endpoint is an IP address, so S3 must use path-style addresses
	// (https://127.0.0.1:port/bucket/key) and not bucket.host addresses.
	client := s3.NewFromConfig(loadConfig(t), func(o *s3.Options) {
		o.UsePathStyle = true
		// The emulator does not return a checksum header with GetObject. The
		// SDK then skips the checksum check and logs a message about it. This
		// option turns that message off.
		o.DisableLogOutputChecksumValidationSkipped = true
	})

	bucket := uniqueName(t)
	const key = "greetings/hello.txt"
	const body = "Hello from Go"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	require.NoError(t, err)
	// Cleanup runs also when the test fails. The calls ignore errors, because
	// the test can delete the object and the bucket itself.
	t.Cleanup(func() {
		ctx := cleanupContext()
		_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(body),
		ContentType: aws.String("text/plain"),
	})
	require.NoError(t, err)

	list, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	require.NoError(t, err)
	require.Len(t, list.Contents, 1)
	require.Equal(t, key, aws.ToString(list.Contents[0].Key))

	got, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	require.NoError(t, err)
	defer got.Body.Close()
	data, err := io.ReadAll(got.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(data))

	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	require.NoError(t, err)
	_, err = client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	require.NoError(t, err)
}
