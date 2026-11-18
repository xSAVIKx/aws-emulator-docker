// Package awsemulator is a Testcontainers for Go module for the AWS Emulator
// image (ghcr.io/xsavikx/aws-emulator).
//
// [Run] starts the container with the TLS front end. The front end serves
// HTTPS on port 9297 with a certificate for the Testcontainers host, for the
// network aliases of the container, and for the names from [WithTLSNames]. A
// CA in the container signs the certificate. After the start, the module
// copies the CA certificate and the generated credentials out of the
// container.
//
// The package does not depend on the AWS SDK. Use [Container.EndpointURL],
// [Container.AccessKeyID], [Container.SecretAccessKey], [Region] and
// [Container.HTTPClient] to configure an SDK client, or [Container.Env] to
// configure a child process.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
package awsemulator

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// DefaultImage is the image that the examples and the tests use when
	// the caller does not give one.
	DefaultImage = "ghcr.io/xsavikx/aws-emulator:latest"

	// Port is the HTTPS port of the emulator in the container.
	Port = "9297/tcp"

	// Region is the AWS region of the emulator. It does not change.
	Region = "us-east-1"

	// AccountID is the AWS account ID of the emulator.
	AccountID = "000000000000"

	// ReadyLog is the log line that tells that the TLS front end is ready.
	ReadyLog = "TLS front end ready"

	// StartupTimeout is the maximum time that Run waits for ReadyLog.
	StartupTimeout = 120 * time.Second

	runDir          = "/home/emulator/.aws/emulator/9297"
	credentialsPath = runDir + "/emulator_credentials"
	caPath          = runDir + "/tls-ca.pem"
)

// Container is a started AWS Emulator container.
type Container struct {
	testcontainers.Container

	accessKeyID     string
	secretAccessKey string
	caPEM           []byte

	mu      sync.Mutex
	caFiles []string
}

// Run creates and starts an AWS Emulator container from the image img.
//
// Pass generic testcontainers options (for example network.WithNetwork) and
// the options of this package ([WithTLSNames], [WithTelemetry]) in opts.
// Run returns a non-nil *Container also with an error when the container
// was created. Terminate it in that case too.
func Run(ctx context.Context, img string, opts ...testcontainers.ContainerCustomizer) (*Container, error) {
	settings := defaultOptions()
	for _, opt := range opts {
		if o, ok := opt.(Option); ok {
			if err := o(&settings); err != nil {
				return nil, err
			}
		}
	}

	moduleOpts := []testcontainers.ContainerCustomizer{
		testcontainers.WithCmd("-p", "9297"),
		testcontainers.WithExposedPorts(Port),
		testcontainers.WithEnv(map[string]string{"EMULATOR_TELEMETRY": "off"}),
		testcontainers.WithWaitStrategy(wait.ForLog(ReadyLog).WithStartupTimeout(StartupTimeout)),
	}
	moduleOpts = append(moduleOpts, opts...)
	// This option runs last, so it sees the network aliases from all
	// other options.
	moduleOpts = append(moduleOpts, finalize(ctx, settings))

	ctr, err := testcontainers.Run(ctx, img, moduleOpts...)
	var c *Container
	if ctr != nil {
		c = &Container{Container: ctr}
	}
	if err != nil {
		return c, fmt.Errorf("run aws emulator: %w", err)
	}

	if err := c.readFiles(ctx); err != nil {
		return c, fmt.Errorf("run aws emulator: %w", err)
	}
	return c, nil
}

// finalize sets EMULATOR_TLS_NAMES and the telemetry setting.
func finalize(ctx context.Context, settings options) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		var names []string
		if host := daemonHost(ctx, req.ProviderType); host != "" {
			names = append(names, host)
		}
		for _, network := range req.Networks {
			names = append(names, req.NetworkAliases[network]...)
		}
		names = append(names, settings.tlsNames...)
		names = uniqueNames(names)
		if len(names) == 0 {
			names = []string{"localhost"}
		}

		if req.Env == nil {
			req.Env = map[string]string{}
		}
		req.Env["EMULATOR_TLS_NAMES"] = strings.Join(names, ",")
		if settings.telemetry {
			delete(req.Env, "EMULATOR_TELEMETRY")
		}
		return nil
	}
}

// daemonHost returns the host that the tests use to reach mapped ports, or
// "" when the provider cannot tell it before the start.
func daemonHost(ctx context.Context, providerType testcontainers.ProviderType) string {
	provider, err := providerType.GetProvider()
	if err != nil {
		return ""
	}
	defer provider.Close()
	dp, ok := provider.(*testcontainers.DockerProvider)
	if !ok {
		return ""
	}
	host, err := dp.DaemonHost(ctx)
	if err != nil {
		return ""
	}
	return host
}

// uniqueNames removes empty and repeated names, and keeps the order.
func uniqueNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// readFiles copies the credentials and the CA certificate out of the
// container. It uses the Docker archive API, so it works also with the
// distroless image, which has no shell.
func (c *Container) readFiles(ctx context.Context) error {
	creds, err := c.readFile(ctx, credentialsPath)
	if err != nil {
		return err
	}
	c.accessKeyID, c.secretAccessKey, err = parseCredentials(creds)
	if err != nil {
		return err
	}

	c.caPEM, err = c.readFile(ctx, caPath)
	if err != nil {
		return err
	}
	if !x509.NewCertPool().AppendCertsFromPEM(c.caPEM) {
		return fmt.Errorf("%s has no PEM certificate", caPath)
	}
	return nil
}

func (c *Container) readFile(ctx context.Context, path string) ([]byte, error) {
	r, err := c.CopyFileFromContainer(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("copy %s from container: %w", path, err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// parseCredentials reads the [default] section of an INI credentials file.
func parseCredentials(data []byte) (keyID, secret string, err error) {
	section := ""
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section != "default" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "aws_access_key_id":
			keyID = strings.TrimSpace(v)
		case "aws_secret_access_key":
			secret = strings.TrimSpace(v)
		}
	}
	if err := sc.Err(); err != nil {
		return "", "", err
	}
	if keyID == "" || secret == "" {
		return "", "", errors.New("emulator_credentials has no [default] access key ID and secret access key")
	}
	return keyID, secret, nil
}

// EndpointURL returns the HTTPS endpoint URL, for example
// "https://localhost:32768".
func (c *Container) EndpointURL(ctx context.Context) (string, error) {
	return c.PortEndpoint(ctx, Port, "https")
}

// Region returns the AWS region of the emulator, "us-east-1".
func (c *Container) Region() string {
	return Region
}

// AccessKeyID returns the access key ID that the emulator generated.
func (c *Container) AccessKeyID() string {
	return c.accessKeyID
}

// SecretAccessKey returns the secret access key that the emulator generated.
func (c *Container) SecretAccessKey() string {
	return c.secretAccessKey
}

// CACertPEM returns the CA certificate (PEM) that signs the TLS certificate
// of the container. The caller can change the returned slice.
func (c *Container) CACertPEM() []byte {
	return bytes.Clone(c.caPEM)
}

// CACertFile writes the CA certificate to a new temporary file and returns
// its path. Use the path for AWS_CA_BUNDLE, SSL_CERT_FILE or
// NODE_EXTRA_CA_CERTS. Terminate removes the file.
func (c *Container) CACertFile() (string, error) {
	f, err := os.CreateTemp("", "aws-emulator-ca-*.pem")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(c.caPEM); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	c.mu.Lock()
	c.caFiles = append(c.caFiles, f.Name())
	c.mu.Unlock()
	return f.Name(), nil
}

// Env returns the environment variables that configure an AWS SDK or the
// AWS CLI in a child process. AWS_CA_BUNDLE is the path of a new file from
// [Container.CACertFile].
func (c *Container) Env(ctx context.Context) (map[string]string, error) {
	endpoint, err := c.EndpointURL(ctx)
	if err != nil {
		return nil, err
	}
	caFile, err := c.CACertFile()
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"AWS_ENDPOINT_URL":          endpoint,
		"AWS_ACCESS_KEY_ID":         c.accessKeyID,
		"AWS_SECRET_ACCESS_KEY":     c.secretAccessKey,
		"AWS_REGION":                Region,
		"AWS_DEFAULT_REGION":        Region,
		"AWS_CA_BUNDLE":             caFile,
		"AWS_EC2_METADATA_DISABLED": "true",
	}, nil
}

// CertPool returns a new certificate pool that holds only the CA
// certificate of the container.
func (c *Container) CertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(c.caPEM)
	return pool
}

// TLSConfig returns a new TLS configuration that trusts only the CA of the
// container.
func (c *Container) TLSConfig() *tls.Config {
	return &tls.Config{
		RootCAs:    c.CertPool(),
		MinVersion: tls.VersionTLS12,
	}
}

// HTTPClient returns a new HTTP client that trusts only the CA of the
// container. Give it to the AWS SDK for Go v2 with config.WithHTTPClient.
func (c *Container) HTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = c.TLSConfig()
	return &http.Client{Transport: transport}
}

// Terminate stops and removes the container, and removes the files from
// [Container.CACertFile].
func (c *Container) Terminate(ctx context.Context, opts ...testcontainers.TerminateOption) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	files := c.caFiles
	c.caFiles = nil
	c.mu.Unlock()

	var errs []error
	if c.Container != nil {
		errs = append(errs, c.Container.Terminate(ctx, opts...))
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
