// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
package io.github.xsavikx.testcontainers.awsemulator;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;

/** Checks that do not start a container. */
class ParseTest {

    @Test
    void readsTheDefaultSectionOfTheCredentialsFile() {
        String ini = "[other]\naws_access_key_id = wrong\n"
                + "[default]\naws_access_key_id = AKID\naws_secret_access_key = se=cret\n";

        assertThat(AwsEmulatorContainer.parseCredentials(ini))
                .containsEntry("aws_access_key_id", "AKID")
                .containsEntry("aws_secret_access_key", "se=cret");
    }

    @Test
    void rejectsATlsNameWithACommaOrASpace() {
        AwsEmulatorContainer container = new AwsEmulatorContainer("aws-emulator:unused");

        assertThatThrownBy(() -> container.withTlsNames("a,b")).isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> container.withTlsNames("a b")).isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> container.withTlsNames(" ")).isInstanceOf(IllegalArgumentException.class);
    }

    @Test
    void accessorsFailBeforeTheStart() {
        AwsEmulatorContainer container = new AwsEmulatorContainer("aws-emulator:unused");

        assertThatThrownBy(container::getAccessKeyId).isInstanceOf(IllegalStateException.class);
    }
}
