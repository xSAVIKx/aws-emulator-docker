package example;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

import software.amazon.awssdk.services.secretsmanager.SecretsManagerClient;
import software.amazon.awssdk.services.secretsmanager.model.ResourceNotFoundException;
import software.amazon.awssdk.services.secretsmanager.model.SecretsManagerException;

/** Secrets scenario: create a JSON secret, read it back, delete it. */
@ExtendWith(Emulator.class)
class SecretsTest {

    private static SecretsManagerClient secrets;
    private static final String SECRET = Emulator.uniqueName("secret");

    @BeforeAll
    static void createClient() {
        secrets = SecretsManagerClient.create();
    }

    @AfterAll
    static void cleanUp() {
        if (secrets == null) {
            return;
        }
        try {
            deleteNow();
        } catch (ResourceNotFoundException alreadyDeleted) {
            // The test deleted the secret. There is nothing to do.
        } finally {
            secrets.close();
        }
    }

    /** Deletes the secret at once, without the recovery window of 7 to 30 days. */
    private static void deleteNow() {
        secrets.deleteSecret(b -> b.secretId(SECRET).forceDeleteWithoutRecovery(true));
    }

    @Test
    void createReadAndDeleteAJsonSecret() {
        String json = "{\"username\":\"app\",\"password\":\"s3cr3t\"}";
        secrets.createSecret(b -> b.name(SECRET).secretString(json));

        String value = secrets.getSecretValue(b -> b.secretId(SECRET)).secretString();
        assertThat(value).isEqualTo(json);

        deleteNow();

        // After the delete, a read fails. The emulator returns InvalidRequestException here,
        // not ResourceNotFoundException, so the test checks only the common parent class.
        assertThatThrownBy(() -> secrets.getSecretValue(b -> b.secretId(SECRET)))
                .isInstanceOf(SecretsManagerException.class);
    }
}
