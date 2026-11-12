package example;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

import software.amazon.awssdk.services.sts.StsClient;
import software.amazon.awssdk.services.sts.model.GetCallerIdentityResponse;

/** Identity scenario: STS GetCallerIdentity returns the emulator account. */
class IdentityTest {

    @Test
    void callerIdentityIsTheEmulatorAccount() {
        // create() uses the default chains: endpoint, credentials and Region from the environment.
        try (StsClient sts = StsClient.create()) {
            GetCallerIdentityResponse identity = sts.getCallerIdentity();

            assertThat(identity.account()).isEqualTo("000000000000");
        }
    }
}
