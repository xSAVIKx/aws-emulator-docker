// SPDX-License-Identifier: Apache-2.0

package awsemulator_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"

	awsemulator "github.com/xSAVIKx/aws-emulator-docker/testcontainers/go"
)

// This example starts the emulator and calls STS with the AWS SDK for Go v2.
func ExampleRun() {
	ctx := context.Background()

	// AWS_EMULATOR_IMAGE selects another image, for example a local build.
	img := awsemulator.DefaultImage
	if v := os.Getenv("AWS_EMULATOR_IMAGE"); v != "" {
		img = v
	}

	ctr, err := awsemulator.Run(ctx, img)
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			log.Printf("terminate container: %s", err)
		}
	}()
	if err != nil {
		log.Printf("run container: %s", err)
		return
	}

	endpoint, err := ctr.EndpointURL(ctx)
	if err != nil {
		log.Printf("endpoint: %s", err)
		return
	}

	// The HTTP client trusts only the CA of the container.
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(ctr.Region()),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			ctr.AccessKeyID(), ctr.SecretAccessKey(), "")),
		config.WithBaseEndpoint(endpoint),
		config.WithHTTPClient(ctr.HTTPClient()),
	)
	if err != nil {
		log.Printf("load config: %s", err)
		return
	}

	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Printf("get caller identity: %s", err)
		return
	}
	fmt.Println(aws.ToString(out.Account))

	// Output:
	// 000000000000
}

// This example configures an S3 client. The endpoint is an IP address or
// "localhost", so the client must use path-style addresses.
func ExampleContainer_HTTPClient() {
	ctx := context.Background()

	ctr, err := awsemulator.Run(ctx, awsemulator.DefaultImage)
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			log.Printf("terminate container: %s", err)
		}
	}()
	if err != nil {
		log.Printf("run container: %s", err)
		return
	}

	endpoint, err := ctr.EndpointURL(ctx)
	if err != nil {
		log.Printf("endpoint: %s", err)
		return
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(ctr.Region()),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			ctr.AccessKeyID(), ctr.SecretAccessKey(), "")),
		config.WithBaseEndpoint(endpoint),
		config.WithHTTPClient(ctr.HTTPClient()),
	)
	if err != nil {
		log.Printf("load config: %s", err)
		return
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String("my-bucket")})
	if err != nil {
		log.Printf("create bucket: %s", err)
		return
	}
}

// This example gives the endpoint, the credentials and the CA certificate
// to a child process (here, the AWS CLI) through environment variables.
func ExampleContainer_Env() {
	ctx := context.Background()

	ctr, err := awsemulator.Run(ctx, awsemulator.DefaultImage)
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			log.Printf("terminate container: %s", err)
		}
	}()
	if err != nil {
		log.Printf("run container: %s", err)
		return
	}

	env, err := ctr.Env(ctx)
	if err != nil {
		log.Printf("env: %s", err)
		return
	}
	cmd := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity")
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Printf("aws cli: %s", err)
	}
}

// This example attaches the container to a network with the alias "aws".
// Other containers on the network reach the emulator at https://aws:9297.
// The TLS certificate is valid for the alias and for "emulator.internal".
func ExampleWithTLSNames() {
	ctx := context.Background()

	nw, err := network.New(ctx)
	if err != nil {
		log.Printf("create network: %s", err)
		return
	}
	defer func() {
		if err := nw.Remove(context.Background()); err != nil {
			log.Printf("remove network: %s", err)
		}
	}()

	ctr, err := awsemulator.Run(ctx, awsemulator.DefaultImage,
		network.WithNetwork([]string{"aws"}, nw),
		awsemulator.WithTLSNames("emulator.internal"),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			log.Printf("terminate container: %s", err)
		}
	}()
	if err != nil {
		log.Printf("run container: %s", err)
		return
	}

	// Copy this CA certificate into the other containers.
	_ = ctr.CACertPEM()
}
