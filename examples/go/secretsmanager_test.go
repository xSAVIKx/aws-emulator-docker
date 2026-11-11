package example

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/require"
)

// DatabaseCredentials is the JSON value of the secret.
type DatabaseCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// TestSecrets creates a JSON secret, reads it back, and deletes it.
func TestSecrets(t *testing.T) {
	ctx := t.Context()
	client := secretsmanager.NewFromConfig(loadConfig(t))
	name := uniqueName(t)
	want := DatabaseCredentials{Username: "app", Password: "s3cr3t"}

	value, err := json.Marshal(want)
	require.NoError(t, err)
	_, err = client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretString: aws.String(string(value)),
	})
	require.NoError(t, err)
	// ForceDeleteWithoutRecovery deletes the secret now, without the default
	// recovery window of 30 days.
	deleteSecret := func() error {
		_, err := client.DeleteSecret(cleanupContext(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(name),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
		return err
	}
	t.Cleanup(func() { _ = deleteSecret() })

	got, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
	require.NoError(t, err)
	var creds DatabaseCredentials
	require.NoError(t, json.Unmarshal([]byte(aws.ToString(got.SecretString)), &creds))
	require.Equal(t, want, creds)

	require.NoError(t, deleteSecret())

	// After the delete, a read fails. The test does not check the error type:
	// real AWS returns ResourceNotFoundException, and the emulator returns
	// InvalidRequestException right after a forced delete.
	_, err = client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
	require.Error(t, err)
}
