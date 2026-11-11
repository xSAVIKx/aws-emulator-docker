package example;

import java.net.URI;
import java.util.Locale;
import java.util.UUID;

import org.junit.jupiter.api.extension.BeforeAllCallback;
import org.junit.jupiter.api.extension.ExtensionContext;

/**
 * Shared helpers for the tests.
 *
 * <p>The tests contain no endpoint, keys or certificate paths. The SDK reads them from the
 * environment that start-emulator.sh sets:
 *
 * <ul>
 *   <li>{@code AWS_ENDPOINT_URL}: the emulator endpoint for every service.
 *   <li>{@code AWS_SHARED_CREDENTIALS_FILE} and {@code AWS_CONFIG_FILE}: the credentials.
 *   <li>{@code AWS_REGION}: the Region.
 *   <li>{@code JAVA_TOOL_OPTIONS}: a truststore that trusts the emulator CA.
 * </ul>
 *
 * <p>Register this class with {@code @ExtendWith(Emulator.class)}. Before the first test of a
 * class, it stops the run if the endpoint is not local. This guard prevents a run against a
 * real AWS account.
 */
public final class Emulator implements BeforeAllCallback {

    @Override
    public void beforeAll(ExtensionContext context) {
        requireLocalEndpoint();
    }

    /** Throws an exception unless AWS_ENDPOINT_URL points to 127.0.0.1 or localhost. */
    static void requireLocalEndpoint() {
        String endpoint = System.getenv("AWS_ENDPOINT_URL");
        if (endpoint == null || endpoint.isBlank()) {
            throw new IllegalStateException(
                    "AWS_ENDPOINT_URL is not set. Start the emulator first: "
                            + "examples/with-emulator.sh examples/java/run-tests.sh");
        }
        String host = URI.create(endpoint).getHost();
        if (!"127.0.0.1".equals(host) && !"localhost".equals(host)) {
            throw new IllegalStateException(
                    "AWS_ENDPOINT_URL must point to 127.0.0.1 or localhost, not " + endpoint);
        }
    }

    /**
     * Returns a name that is unique for this run, for example {@code ex-java-3f9c1a2b-queue}.
     * The name is lowercase, so it is also a valid S3 bucket name.
     */
    static String uniqueName(String suffix) {
        String random = UUID.randomUUID().toString().substring(0, 8);
        return ("ex-java-" + random + "-" + suffix).toLowerCase(Locale.ROOT);
    }
}
