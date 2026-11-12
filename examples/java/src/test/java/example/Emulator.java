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
 * <p>JUnit registers this class for every test class. The file
 * {@code src/test/resources/junit-platform.properties} turns on extension auto-detection, and
 * {@code META-INF/services/org.junit.jupiter.api.extension.Extension} names this class. The
 * check of the endpoint runs once for the whole run. If the endpoint is not local, every test
 * class fails before its first test. This guard prevents a run against a real AWS account.
 */
public final class Emulator implements BeforeAllCallback {

    @Override
    public void beforeAll(ExtensionContext context) {
        // The root store is shared by all test classes, so the check runs only once.
        // Each test class then gets the same result.
        Object failure = context.getRoot()
                .getStore(ExtensionContext.Namespace.create(Emulator.class))
                .getOrComputeIfAbsent("endpoint-check", key -> checkEndpoint());
        if (failure instanceof IllegalStateException e) {
            throw e;
        }
    }

    /** Returns the guard failure, or a string when the endpoint is local. */
    private static Object checkEndpoint() {
        try {
            requireLocalEndpoint();
            return "ok";
        } catch (IllegalStateException e) {
            return e;
        }
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
     * Returns a name that is unique for this run, for example {@code ex-java-3f9c1a2b}.
     * The name is lowercase, so it is also a valid S3 bucket name.
     */
    static String uniqueName() {
        String random = UUID.randomUUID().toString().substring(0, 8);
        return ("ex-java-" + random).toLowerCase(Locale.ROOT);
    }
}
