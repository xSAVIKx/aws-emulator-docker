// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
package io.github.xsavikx.testcontainers.awsemulator;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;

import org.junit.jupiter.api.Test;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.Network;
import org.testcontainers.containers.startupcheck.OneShotStartupCheckStrategy;
import org.testcontainers.images.builder.Transferable;

/**
 * A second container on the same network calls the emulator by its network alias. The
 * certificate has the alias, so curl accepts it with the CA only.
 */
class NetworkAliasTest {

    private static final String CURL_IMAGE = "curlimages/curl:8.23.0"
            + "@sha256:3fb7e13e5d179a845c97d78204f3f1064d1907037f26a3ebf91710571151fd93";

    @Test
    void anotherContainerCallsTheEmulatorByItsAlias() {
        try (Network network = Network.newNetwork();
                AwsEmulatorContainer emulator = new AwsEmulatorContainer(TestImage.name())
                        .withNetwork(network)
                        .withNetworkAliases("aws")) {
            emulator.start();

            try (GenericContainer<?> curl = new GenericContainer<>(CURL_IMAGE)
                    .withNetwork(network)
                    .withCopyToContainer(Transferable.of(emulator.getCaCertificatePem()), "/ca.pem")
                    .withCommand("-sf", "--cacert", "/ca.pem", "https://aws:9297/__admin/health")
                    .withStartupCheckStrategy(
                            new OneShotStartupCheckStrategy().withTimeout(Duration.ofSeconds(60)))) {
                // The start fails when curl exits with a code other than 0.
                curl.start();

                assertThat(curl.getLogs().strip()).isEqualTo("ok");
            }
        }
    }
}
