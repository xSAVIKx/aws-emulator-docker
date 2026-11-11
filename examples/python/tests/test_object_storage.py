"""Object storage scenario: S3 bucket and object."""


def test_put_list_get_delete_object(s3, unique_name):
    bucket = unique_name
    key = "greetings/hello.txt"
    body = b"Hello from the Python example"

    # us-east-1 is the default Region, so no CreateBucketConfiguration is needed.
    s3.create_bucket(Bucket=bucket)
    try:
        s3.put_object(Bucket=bucket, Key=key, Body=body, ContentType="text/plain")

        listing = s3.list_objects_v2(Bucket=bucket)
        assert [obj["Key"] for obj in listing["Contents"]] == [key]

        obj = s3.get_object(Bucket=bucket, Key=key)
        assert obj["Body"].read() == body
        assert obj["ContentType"] == "text/plain"

        s3.delete_object(Bucket=bucket, Key=key)
        assert s3.list_objects_v2(Bucket=bucket)["KeyCount"] == 0
    finally:
        # Clean up also when an assertion fails. A bucket must be empty
        # before you can delete it.
        for obj in s3.list_objects_v2(Bucket=bucket).get("Contents", []):
            s3.delete_object(Bucket=bucket, Key=obj["Key"])
        s3.delete_bucket(Bucket=bucket)
