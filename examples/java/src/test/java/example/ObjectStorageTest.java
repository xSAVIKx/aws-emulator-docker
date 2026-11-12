package example;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.charset.StandardCharsets;

import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import software.amazon.awssdk.core.sync.RequestBody;
import software.amazon.awssdk.services.s3.S3Client;
import software.amazon.awssdk.services.s3.model.NoSuchBucketException;
import software.amazon.awssdk.services.s3.model.S3Object;

/** Object storage scenario: create a bucket, put an object, list and get it, delete both. */
class ObjectStorageTest {

    private static S3Client s3;
    private static final String BUCKET = Emulator.uniqueName();
    private static final String KEY = "greetings/hello.txt";

    @BeforeAll
    static void createClient() {
        // The endpoint is an IP address, so S3 must use path-style addresses
        // (https://127.0.0.1:<port>/<bucket>/<key>), not <bucket>.<host>.
        s3 = S3Client.builder().forcePathStyle(true).build();
    }

    @AfterAll
    static void cleanUp() {
        if (s3 == null) {
            return;
        }
        // The test deletes the object and the bucket. This is a second attempt if the test failed.
        try {
            s3.deleteObject(b -> b.bucket(BUCKET).key(KEY));
            s3.deleteBucket(b -> b.bucket(BUCKET));
        } catch (NoSuchBucketException alreadyDeleted) {
            // The test deleted the bucket. There is nothing to do.
        } finally {
            s3.close();
        }
    }

    @Test
    void putListGetAndDeleteAnObject() {
        s3.createBucket(b -> b.bucket(BUCKET));

        s3.putObject(b -> b.bucket(BUCKET).key(KEY).contentType("text/plain"),
                RequestBody.fromString("Hello from Java", StandardCharsets.UTF_8));

        assertThat(s3.listObjectsV2(b -> b.bucket(BUCKET)).contents())
                .extracting(S3Object::key)
                .containsExactly(KEY);

        String body = s3.getObjectAsBytes(b -> b.bucket(BUCKET).key(KEY)).asUtf8String();
        assertThat(body).isEqualTo("Hello from Java");

        s3.deleteObject(b -> b.bucket(BUCKET).key(KEY));
        s3.deleteBucket(b -> b.bucket(BUCKET));

        // ListBuckets no longer shows the bucket.
        assertThat(s3.listBuckets().buckets()).noneMatch(bucket -> bucket.name().equals(BUCKET));
    }
}
