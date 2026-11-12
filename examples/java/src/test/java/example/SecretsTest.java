package example;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import software.amazon.awssdk.services.secretsmanager.SecretsManagerClient;
import software.amazon.awssdk.services.secretsmanager.model.SecretsManagerException;

/** Secrets scenario: create a JSON secret, read it back, delete it. */
class SecretsTest {

    private static SecretsManagerClient secrets;
    private static final String SECRET = Emulator.uniqueName();
    private static boolean created;
    private static boolean deleted;

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
            // Delete the secret only when the test created it and did not delete it.
            if (created && !deleted) {
                deleteNow();
            }
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
        created = true;

        String value = secrets.getSecretValue(b -> b.secretId(SECRET)).secretString();
        assertThat(value).isEqualTo(json);

        deleteNow();
        deleted = true;

        // After the delete, a read fails. The deletion is asynchronous, so right after the
        // delete real AWS can also return InvalidRequestException instead of
        // ResourceNotFoundException. The test checks only the common parent class.
        assertThatThrownBy(() -> secrets.getSecretValue(b -> b.secretId(SECRET)))
                .isInstanceOf(SecretsManagerException.class);
    }
}
