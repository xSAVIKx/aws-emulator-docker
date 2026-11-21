// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
package io.github.xsavikx.testcontainers.awsemulator;

import static org.assertj.core.api.Assertions.assertThat;

import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;

import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import software.amazon.awssdk.core.sync.RequestBody;
import software.amazon.awssdk.services.s3.S3Client;
import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.Message;
import software.amazon.awssdk.services.sts.StsClient;

/** Starts one emulator container and calls it with the AWS SDK for Java 2.x. */
class AwsEmulatorContainerTest {

    private static AwsEmulatorContainer emulator;

    @BeforeAll
    static void start() {
        emulator = new AwsEmulatorContainer(TestImage.name()).withTlsNames("emulator.test");
        emulator.start();
    }

    @AfterAll
    static void stop() {
        if (emulator != null) {
            emulator.stop();
        }
    }

    @Test
    void callerIdentityIsTheEmulatorAccount() {
        try (StsClient sts = TestImage.configure(StsClient.builder(), emulator).build()) {
            assertThat(sts.getCallerIdentity().account()).isEqualTo("000000000000");
        }
    }

    @Test
    void putAndGetAnObject() {
        // Path-style addresses: the bucket name is not a part of the host name.
        try (S3Client s3 = TestImage.configure(S3Client.builder(), emulator)
                .forcePathStyle(true)
                .build()) {
            String bucket = TestImage.uniqueName();
            s3.createBucket(b -> b.bucket(bucket));
            s3.putObject(b -> b.bucket(bucket).key("greeting.txt"), RequestBody.fromString("hello"));

            String body = s3.getObjectAsBytes(b -> b.bucket(bucket).key("greeting.txt")).asUtf8String();

            assertThat(body).isEqualTo("hello");
            s3.deleteObject(b -> b.bucket(bucket).key("greeting.txt"));
            s3.deleteBucket(b -> b.bucket(bucket));
        }
    }

    @Test
    void sendAndReceiveAMessage() {
        try (SqsClient sqs = TestImage.configure(SqsClient.builder(), emulator).build()) {
            String queueUrl = sqs.createQueue(b -> b.queueName(TestImage.uniqueName())).queueUrl();
            sqs.sendMessage(b -> b.queueUrl(queueUrl).messageBody("hello from Testcontainers"));

            List<Message> messages = sqs.receiveMessage(b -> b
                    .queueUrl(queueUrl)
                    .maxNumberOfMessages(1)
                    .waitTimeSeconds(5)).messages();

            assertThat(messages).extracting(Message::body).containsExactly("hello from Testcontainers");
            sqs.deleteQueue(b -> b.queueUrl(queueUrl));
        }
    }

    @Test
    void sslContextTrustsTheEmulator() throws Exception {
        HttpClient client = HttpClient.newBuilder().sslContext(emulator.getSslContext()).build();
        HttpResponse<String> response = client.send(
                HttpRequest.newBuilder(emulator.getEndpoint().resolve("/__admin/health")).build(),
                HttpResponse.BodyHandlers.ofString());

        assertThat(response.statusCode()).isEqualTo(200);
        assertThat(response.body().strip()).isEqualTo("ok");
    }

    @Test
    void environmentVariablesDescribeTheEmulator() throws Exception {
        Map<String, String> env = emulator.getEnvironmentVariables();

        assertThat(env).containsEntry("AWS_ENDPOINT_URL", emulator.getEndpoint().toString())
                .containsEntry("AWS_ACCESS_KEY_ID", emulator.getAccessKeyId())
                .containsEntry("AWS_SECRET_ACCESS_KEY", emulator.getSecretAccessKey())
                .containsEntry("AWS_REGION", "us-east-1")
                .containsEntry("AWS_DEFAULT_REGION", "us-east-1")
                .containsEntry("AWS_EC2_METADATA_DISABLED", "true");
        Path ca = Path.of(env.get("AWS_CA_BUNDLE"));
        assertThat(Files.readString(ca)).isEqualTo(emulator.getCaCertificatePem());
        assertThat(emulator.getCaCertificatePem()).startsWith("-----BEGIN CERTIFICATE-----");
        assertThat(emulator.getEndpoint().getScheme()).isEqualTo("https");
    }

    @Test
    void tlsNamesContainTheHostTheAliasesAndTheExtraNames() {
        String names = emulator.getEnvMap().get("EMULATOR_TLS_NAMES");

        assertThat(names.split(",")).contains(emulator.getHost(), "emulator.test")
                .containsAll(emulator.getNetworkAliases());
        assertThat(emulator.getEnvMap()).containsEntry("EMULATOR_TELEMETRY", "off");
    }
}
