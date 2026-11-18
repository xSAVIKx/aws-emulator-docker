// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
package io.github.xsavikx.testcontainers.awsemulator;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.security.GeneralSecurityException;
import java.security.KeyStore;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;

import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManager;
import javax.net.ssl.TrustManagerFactory;

import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.wait.strategy.Wait;
import org.testcontainers.utility.DockerImageName;

import com.github.dockerjava.api.command.InspectContainerResponse;

/**
 * A Testcontainers container for the AWS Emulator image with its TLS front end.
 *
 * <p>The container serves HTTPS on port 9297. A CA in the container signs the certificate.
 * After the start, the container object has the endpoint, the credentials and the CA. Use
 * {@link #getTrustManagers()} or {@link #getSslContext()} to trust only that CA.
 *
 * <p>The emulator requires SigV4 with the credentials from {@link #getAccessKeyId()} and
 * {@link #getSecretAccessKey()}. The account is {@value #ACCOUNT_ID}. The Region is
 * {@value #REGION}.
 *
 * <pre>{@code
 * try (AwsEmulatorContainer emulator = new AwsEmulatorContainer()) {
 *     emulator.start();
 *     StsClient sts = StsClient.builder()
 *             .endpointOverride(emulator.getEndpoint())
 *             .region(Region.of(emulator.getRegion()))
 *             .credentialsProvider(StaticCredentialsProvider.create(AwsBasicCredentials.create(
 *                     emulator.getAccessKeyId(), emulator.getSecretAccessKey())))
 *             .httpClientBuilder(Apache5HttpClient.builder()
 *                     .tlsTrustManagersProvider(emulator::getTrustManagers))
 *             .build();
 * }
 * }</pre>
 */
public class AwsEmulatorContainer extends GenericContainer<AwsEmulatorContainer> {

    /** The default image. */
    public static final DockerImageName DEFAULT_IMAGE_NAME =
            DockerImageName.parse("ghcr.io/xsavikx/aws-emulator:latest");

    /** The port of the emulator in the container (HTTPS). */
    public static final int PORT = 9297;

    /** The Region of the emulator. */
    public static final String REGION = "us-east-1";

    /** The account ID of the emulator. */
    public static final String ACCOUNT_ID = "000000000000";

    /** The folder in the container where the emulator writes its files. */
    static final String DATA_DIR = "/home/emulator/.aws/emulator/" + PORT + "/";

    static final String CREDENTIALS_FILE = DATA_DIR + "emulator_credentials";
    static final String CA_FILE = DATA_DIR + "tls-ca.pem";

    private static final Duration STARTUP_TIMEOUT = Duration.ofSeconds(120);

    private final Set<String> extraTlsNames = new LinkedHashSet<>();
    private boolean telemetry;

    private String accessKeyId;
    private String secretAccessKey;
    private String caCertificatePem;
    private Path caCertificateFile;

    /** Creates a container for the default image {@code ghcr.io/xsavikx/aws-emulator:latest}. */
    public AwsEmulatorContainer() {
        this(DEFAULT_IMAGE_NAME);
    }

    /** Creates a container for the given image, for example {@code aws-emulator:local}. */
    public AwsEmulatorContainer(String dockerImageName) {
        this(DockerImageName.parse(dockerImageName));
    }

    /** Creates a container for the given image. */
    public AwsEmulatorContainer(DockerImageName dockerImageName) {
        super(dockerImageName);
        withExposedPorts(PORT);
        withCommand("-p", String.valueOf(PORT));
        // The health check of the image runs first after 10 s. The log line comes earlier.
        waitingFor(Wait.forLogMessage(".*TLS front end ready.*", 1)
                .withStartupTimeout(STARTUP_TIMEOUT));
    }

    /**
     * Adds host names or IP addresses to the TLS certificate. The container always adds the
     * Testcontainers host and its network aliases.
     *
     * @param names host names or IP addresses, without commas or spaces
     * @return this container
     */
    public AwsEmulatorContainer withTlsNames(String... names) {
        for (String name : names) {
            Objects.requireNonNull(name, "name");
            String trimmed = name.trim();
            if (trimmed.isEmpty() || trimmed.matches(".*[,\\s].*")) {
                throw new IllegalArgumentException("Not a valid TLS name: '" + name + "'");
            }
            extraTlsNames.add(trimmed);
        }
        return self();
    }

    /**
     * Turns the telemetry of the emulator on or off. The default is off: the container sets
     * {@code EMULATOR_TELEMETRY=off}. When it is on, the container does not set the variable.
     *
     * @param enabled {@code true} to turn telemetry on
     * @return this container
     */
    public AwsEmulatorContainer withTelemetry(boolean enabled) {
        this.telemetry = enabled;
        return self();
    }

    @Override
    protected void configure() {
        super.configure();
        if (telemetry) {
            getEnvMap().remove("EMULATOR_TELEMETRY");
        } else {
            withEnv("EMULATOR_TELEMETRY", "off");
        }
        withEnv("EMULATOR_TLS_NAMES", String.join(",", tlsNames()));
    }

    /**
     * Returns the names for {@code EMULATOR_TLS_NAMES}: the Testcontainers host, the network
     * aliases and the extra names. The list is never empty.
     */
    List<String> tlsNames() {
        Set<String> names = new LinkedHashSet<>();
        try {
            names.add(getHost());
        } catch (RuntimeException e) {
            // The host is not known before the start. Use the aliases and the extra names.
        }
        names.addAll(getNetworkAliases());
        names.addAll(extraTlsNames);
        names.removeIf(n -> n == null || n.isBlank());
        if (names.isEmpty()) {
            names.add("localhost");
        }
        return new ArrayList<>(names);
    }

    @Override
    protected void containerIsStarted(InspectContainerResponse containerInfo) {
        super.containerIsStarted(containerInfo);
        String credentials = copyFileFromContainer(CREDENTIALS_FILE,
                in -> new String(in.readAllBytes(), StandardCharsets.UTF_8));
        Map<String, String> values = parseCredentials(credentials);
        accessKeyId = require(values, "aws_access_key_id");
        secretAccessKey = require(values, "aws_secret_access_key");
        caCertificatePem = copyFileFromContainer(CA_FILE,
                in -> new String(in.readAllBytes(), StandardCharsets.UTF_8));
        caCertificateFile = null;
    }

    /** Reads the keys of the {@code [default]} section of an INI file. */
    static Map<String, String> parseCredentials(String ini) {
        Map<String, String> values = new LinkedHashMap<>();
        String section = "";
        for (String raw : ini.split("\\R")) {
            String line = raw.strip();
            if (line.isEmpty() || line.startsWith("#") || line.startsWith(";")) {
                continue;
            }
            if (line.startsWith("[") && line.endsWith("]")) {
                section = line.substring(1, line.length() - 1).strip();
                continue;
            }
            int eq = line.indexOf('=');
            if (eq > 0 && "default".equals(section)) {
                values.put(line.substring(0, eq).strip(), line.substring(eq + 1).strip());
            }
        }
        return values;
    }

    private static String require(Map<String, String> values, String key) {
        String value = values.get(key);
        if (value == null || value.isEmpty()) {
            throw new IllegalStateException(CREDENTIALS_FILE + " has no " + key);
        }
        return value;
    }

    /** Returns the endpoint, for example {@code https://localhost:32768}. */
    public URI getEndpoint() {
        return URI.create("https://" + hostForUrl() + ":" + getMappedPort(PORT));
    }

    private String hostForUrl() {
        String host = getHost();
        // An IPv6 address needs brackets in a URL.
        return host.contains(":") && !host.startsWith("[") ? "[" + host + "]" : host;
    }

    /** Returns the Region, {@value #REGION}. */
    public String getRegion() {
        return REGION;
    }

    /** Returns the account ID, {@value #ACCOUNT_ID}. */
    public String getAccountId() {
        return ACCOUNT_ID;
    }

    /** Returns the access key ID that the emulator accepts. */
    public String getAccessKeyId() {
        requireStarted();
        return accessKeyId;
    }

    /** Returns the secret access key that the emulator accepts. */
    public String getSecretAccessKey() {
        requireStarted();
        return secretAccessKey;
    }

    /** Returns the CA certificate in PEM format. */
    public String getCaCertificatePem() {
        requireStarted();
        return caCertificatePem;
    }

    /** Returns the CA certificate. */
    public X509Certificate getCaCertificate() {
        try {
            CertificateFactory factory = CertificateFactory.getInstance("X.509");
            return (X509Certificate) factory.generateCertificate(
                    new ByteArrayInputStream(getCaCertificatePem().getBytes(StandardCharsets.US_ASCII)));
        } catch (GeneralSecurityException e) {
            throw new IllegalStateException("Cannot read the CA certificate", e);
        }
    }

    /**
     * Writes the CA certificate to a temporary file and returns its path. Use the path for
     * {@code AWS_CA_BUNDLE}, {@code SSL_CERT_FILE} or {@code NODE_EXTRA_CA_CERTS}. The JVM
     * deletes the file when it exits. The method writes the file once for each start.
     */
    public synchronized Path getCaCertificateFile() {
        String pem = getCaCertificatePem();
        if (caCertificateFile == null || !Files.exists(caCertificateFile)) {
            try {
                Path file = Files.createTempFile("aws-emulator-ca-", ".pem");
                file.toFile().deleteOnExit();
                Files.writeString(file, pem, StandardCharsets.US_ASCII);
                try {
                    Files.setPosixFilePermissions(file, PosixFilePermissions.fromString("rw-r--r--"));
                } catch (UnsupportedOperationException notPosix) {
                    // The file system has no POSIX permissions. The default is sufficient.
                }
                caCertificateFile = file;
            } catch (IOException e) {
                throw new UncheckedIOException("Cannot write the CA certificate", e);
            }
        }
        return caCertificateFile;
    }

    /** Returns a key store (PKCS12, no password) that contains only the CA certificate. */
    public KeyStore getTrustStore() {
        try {
            KeyStore store = KeyStore.getInstance("PKCS12");
            store.load(null, null);
            store.setCertificateEntry("aws-emulator-ca", getCaCertificate());
            return store;
        } catch (GeneralSecurityException | IOException e) {
            throw new IllegalStateException("Cannot make the trust store", e);
        }
    }

    /** Returns trust managers that trust only the CA of this container. */
    public TrustManager[] getTrustManagers() {
        try {
            TrustManagerFactory factory =
                    TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
            factory.init(getTrustStore());
            return factory.getTrustManagers();
        } catch (GeneralSecurityException e) {
            throw new IllegalStateException("Cannot make the trust managers", e);
        }
    }

    /** Returns a TLS context that trusts only the CA of this container. */
    public SSLContext getSslContext() {
        try {
            SSLContext context = SSLContext.getInstance("TLS");
            context.init(null, getTrustManagers(), null);
            return context;
        } catch (GeneralSecurityException e) {
            throw new IllegalStateException("Cannot make the TLS context", e);
        }
    }

    /**
     * Returns the environment for a child process or an SDK: {@code AWS_ENDPOINT_URL},
     * {@code AWS_ACCESS_KEY_ID}, {@code AWS_SECRET_ACCESS_KEY}, {@code AWS_REGION},
     * {@code AWS_DEFAULT_REGION}, {@code AWS_CA_BUNDLE} and
     * {@code AWS_EC2_METADATA_DISABLED=true}. {@code AWS_CA_BUNDLE} is the path from
     * {@link #getCaCertificateFile()}.
     */
    public Map<String, String> getEnvironmentVariables() {
        Map<String, String> env = new LinkedHashMap<>();
        env.put("AWS_ENDPOINT_URL", getEndpoint().toString());
        env.put("AWS_ACCESS_KEY_ID", getAccessKeyId());
        env.put("AWS_SECRET_ACCESS_KEY", getSecretAccessKey());
        env.put("AWS_REGION", REGION);
        env.put("AWS_DEFAULT_REGION", REGION);
        env.put("AWS_CA_BUNDLE", getCaCertificateFile().toString());
        env.put("AWS_EC2_METADATA_DISABLED", "true");
        return Collections.unmodifiableMap(env);
    }

    private void requireStarted() {
        if (caCertificatePem == null) {
            throw new IllegalStateException("The container is not started");
        }
    }
}
