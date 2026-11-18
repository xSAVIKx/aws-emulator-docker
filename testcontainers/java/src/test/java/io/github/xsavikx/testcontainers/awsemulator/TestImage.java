// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
package io.github.xsavikx.testcontainers.awsemulator;

import java.util.Locale;
import java.util.UUID;

import software.amazon.awssdk.auth.credentials.AwsBasicCredentials;
import software.amazon.awssdk.auth.credentials.StaticCredentialsProvider;
import software.amazon.awssdk.awscore.client.builder.AwsClientBuilder;
import software.amazon.awssdk.awscore.client.builder.AwsSyncClientBuilder;
import software.amazon.awssdk.http.apache5.Apache5HttpClient;
import software.amazon.awssdk.regions.Region;

/** Shared helpers for the tests. */
final class TestImage {

    private TestImage() {
    }

    /**
     * Returns the image for the tests. The variable {@code AWS_EMULATOR_IMAGE} overrides the
     * default image, so CI can test a locally built image.
     */
    static String name() {
        String image = System.getenv("AWS_EMULATOR_IMAGE");
        if (image == null || image.isBlank()) {
            return AwsEmulatorContainer.DEFAULT_IMAGE_NAME.asCanonicalNameString();
        }
        return image;
    }

    /**
     * Configures an SDK client builder for the emulator: the endpoint, the Region, the
     * credentials, and an Apache 5 HTTP client that trusts only the emulator CA.
     */
    static <B extends AwsClientBuilder<B, C> & AwsSyncClientBuilder<B, C>, C> B configure(
            B builder, AwsEmulatorContainer emulator) {
        return builder
                .endpointOverride(emulator.getEndpoint())
                .region(Region.of(emulator.getRegion()))
                .credentialsProvider(StaticCredentialsProvider.create(AwsBasicCredentials.create(
                        emulator.getAccessKeyId(), emulator.getSecretAccessKey())))
                .httpClientBuilder(Apache5HttpClient.builder()
                        .tlsTrustManagersProvider(emulator::getTrustManagers));
    }

    /** Returns a lowercase name that is unique for this run, for example {@code tc-java-3f9c1a2b}. */
    static String uniqueName() {
        return ("tc-java-" + UUID.randomUUID().toString().substring(0, 8)).toLowerCase(Locale.ROOT);
    }
}
