// SPDX-License-Identifier: Apache-2.0

package awsemulator_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	awsemulator "github.com/xSAVIKx/aws-emulator-docker/testcontainers/go"
)

const curlImage = "curlimages/curl:8.23.0@sha256:3fb7e13e5d179a845c97d78204f3f1064d1907037f26a3ebf91710571151fd93"

// image returns the image under test. AWS_EMULATOR_IMAGE overrides the
// default image, so CI can test a locally built image.
func image() string {
	if img := os.Getenv("AWS_EMULATOR_IMAGE"); img != "" {
		return img
	}
	return awsemulator.DefaultImage
}

// run starts a container for one test and terminates it at the end.
func run(t *testing.T, opts ...testcontainers.ContainerCustomizer) *awsemulator.Container {
	t.Helper()
	ctr, err := awsemulator.Run(t.Context(), image(), opts...)
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)
	return ctr
}

// awsConfig returns an SDK configuration for the container. The HTTP client
// trusts only the CA of the container.
func awsConfig(t *testing.T, ctr *awsemulator.Container) aws.Config {
	t.Helper()
	ctx := t.Context()
	endpoint, err := ctr.EndpointURL(ctx)
	require.NoError(t, err)
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(ctr.Region()),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			ctr.AccessKeyID(), ctr.SecretAccessKey(), "")),
		config.WithBaseEndpoint(endpoint),
		config.WithHTTPClient(ctr.HTTPClient()),
	)
	require.NoError(t, err)
	return cfg
}

// uniqueName returns a name like "tc-go-3f9a1c2b". It is also a valid S3
// bucket name.
func uniqueName(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return "tc-go-" + hex.EncodeToString(b)
}

// TestServices runs the STS, S3 and SQS tests on one container.
func TestServices(t *testing.T) {
	ctr := run(t)
	cfg := awsConfig(t, ctr)

	t.Run("STS", func(t *testing.T) {
		out, err := sts.NewFromConfig(cfg).GetCallerIdentity(t.Context(), &sts.GetCallerIdentityInput{})
		require.NoError(t, err)
		require.Equal(t, awsemulator.AccountID, aws.ToString(out.Account))
	})

	t.Run("S3", func(t *testing.T) {
		ctx := t.Context()
		client := s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = true
			// The emulator does not return a checksum header with
			// GetObject. This option turns off the log message about it.
			o.DisableLogOutputChecksumValidationSkipped = true
		})
		bucket := uniqueName(t)
		const key = "greetings/hello.txt"
		const body = "Hello from Testcontainers"

		_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
		require.NoError(t, err)
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   strings.NewReader(body),
		})
		require.NoError(t, err)

		got, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		require.NoError(t, err)
		defer got.Body.Close()
		data, err := io.ReadAll(got.Body)
		require.NoError(t, err)
		require.Equal(t, body, string(data))
	})

	t.Run("SQS", func(t *testing.T) {
		ctx := t.Context()
		client := sqs.NewFromConfig(cfg)
		created, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(uniqueName(t))})
		require.NoError(t, err)

		const body = "order-created"
		_, err = client.SendMessage(ctx, &sqs.SendMessageInput{QueueUrl: created.QueueUrl, MessageBody: aws.String(body)})
		require.NoError(t, err)

		got, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            created.QueueUrl,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     5,
		})
		require.NoError(t, err)
		require.Len(t, got.Messages, 1)
		require.Equal(t, body, aws.ToString(got.Messages[0].Body))
	})

	t.Run("Env", func(t *testing.T) {
		env, err := ctr.Env(t.Context())
		require.NoError(t, err)
		endpoint, err := ctr.EndpointURL(t.Context())
		require.NoError(t, err)
		require.Equal(t, endpoint, env["AWS_ENDPOINT_URL"])
		require.Equal(t, ctr.AccessKeyID(), env["AWS_ACCESS_KEY_ID"])
		require.Equal(t, ctr.SecretAccessKey(), env["AWS_SECRET_ACCESS_KEY"])
		require.Equal(t, "us-east-1", env["AWS_REGION"])
		require.Equal(t, "us-east-1", env["AWS_DEFAULT_REGION"])
		require.Equal(t, "true", env["AWS_EC2_METADATA_DISABLED"])
		data, err := os.ReadFile(env["AWS_CA_BUNDLE"])
		require.NoError(t, err)
		require.Equal(t, ctr.CACertPEM(), data)
	})
}

// TestNetworkAlias calls the emulator from a second container on the same
// network by its alias "aws". The certificate must be valid for that name.
func TestNetworkAlias(t *testing.T) {
	ctx := t.Context()
	nw, err := network.New(ctx)
	testcontainers.CleanupNetwork(t, nw)
	require.NoError(t, err)

	ctr := run(t, network.WithNetwork([]string{"aws"}, nw))

	curl, err := testcontainers.Run(ctx, curlImage,
		network.WithNetwork(nil, nw),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			Reader:            bytes.NewReader(ctr.CACertPEM()),
			ContainerFilePath: "/ca.pem",
			FileMode:          0o644,
		}),
		testcontainers.WithCmd("-sf", "--cacert", "/ca.pem", "https://aws:9297/__admin/health"),
		testcontainers.WithWaitStrategy(wait.ForExit().WithExitTimeout(60*time.Second)),
	)
	testcontainers.CleanupContainer(t, curl)
	require.NoError(t, err)

	state, err := curl.State(ctx)
	require.NoError(t, err)
	logs, err := curl.Logs(ctx)
	require.NoError(t, err)
	defer logs.Close()
	out, err := io.ReadAll(logs)
	require.NoError(t, err)
	require.Equal(t, 0, state.ExitCode, "curl output: %s", out)
	require.Equal(t, "ok", strings.TrimSpace(string(out)))
}

// TestTLSNames checks that the extra names are in the certificate and that
// the client rejects other names.
func TestTLSNames(t *testing.T) {
	ctr := run(t, awsemulator.WithTLSNames("emulator.test", "10.1.2.3"))
	ctx := t.Context()
	endpoint, err := ctr.EndpointURL(ctx)
	require.NoError(t, err)

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, awsemulator.Port)
	require.NoError(t, err)

	handshake := func(serverName string) error {
		cfg := ctr.TLSConfig()
		cfg.ServerName = serverName
		conn, err := tlsDial(ctx, host+":"+port.Port(), cfg)
		if err != nil {
			return err
		}
		return conn.Close()
	}
	require.NoError(t, handshake("emulator.test"))
	require.NoError(t, handshake("10.1.2.3"))
	require.Error(t, handshake("other.test"))

	resp, err := ctr.HTTPClient().Get(endpoint + "/__admin/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", strings.TrimSpace(string(body)))

	// The default settings: telemetry is off, and the names are in the
	// variable.
	info, err := ctr.Inspect(ctx)
	require.NoError(t, err)
	require.Contains(t, info.Config.Env, "EMULATOR_TELEMETRY=off")
	require.Contains(t, info.Config.Env, "EMULATOR_TLS_NAMES="+host+",emulator.test,10.1.2.3")
}

// tlsDial opens a TLS connection with cfg. The caller closes it.
func tlsDial(ctx context.Context, addr string, cfg *tls.Config) (*tls.Conn, error) {
	d := tls.Dialer{Config: cfg}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn.(*tls.Conn), nil
}
