// Package example shows how to test Go code that uses the AWS SDK for Go v2
// against the AWS Emulator.
//
// The tests contain no endpoint, keys or certificate paths. The SDK reads them
// from the standard environment variables that examples/with-emulator.sh sets.
package example

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/require"
)

// TestMain runs once before all tests. It stops the run if the tests could
// reach a real AWS account.
func TestMain(m *testing.M) {
	if err := checkEndpoint(os.Getenv("AWS_ENDPOINT_URL")); err != nil {
		fmt.Fprintln(os.Stderr, "refusing to run:", err)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

// checkEndpoint accepts only an endpoint on this computer.
func checkEndpoint(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("AWS_ENDPOINT_URL is not set; start the emulator with examples/with-emulator.sh")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("AWS_ENDPOINT_URL %q is not a URL: %w", endpoint, err)
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost":
		return nil
	default:
		return fmt.Errorf("AWS_ENDPOINT_URL %q does not point to 127.0.0.1 or localhost", endpoint)
	}
}

// loadConfig returns the shared SDK configuration.
//
// config.LoadDefaultConfig reads these environment variables:
//   - AWS_ENDPOINT_URL: the base endpoint for every service client
//   - AWS_CA_BUNDLE: the emulator CA certificate for HTTPS
//   - AWS_REGION, AWS_SHARED_CREDENTIALS_FILE, AWS_CONFIG_FILE
func loadConfig(t *testing.T) aws.Config {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(t.Context())
	require.NoError(t, err)
	return cfg
}

// uniqueName returns a name like "ex-go-3f9a1c2b" that is unique for each run.
// The name uses lowercase letters, digits and hyphens only, so it is also a
// valid S3 bucket name.
func uniqueName(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return "ex-go-" + hex.EncodeToString(b)
}

// cleanupContext returns a context for t.Cleanup functions. t.Context() is
// canceled before cleanup functions run, so cleanup cannot use it.
func cleanupContext() context.Context {
	return context.Background()
}
