package example

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"
)

// TestIdentity checks that the emulator answers as account 000000000000.
func TestIdentity(t *testing.T) {
	client := sts.NewFromConfig(loadConfig(t))

	out, err := client.GetCallerIdentity(t.Context(), &sts.GetCallerIdentityInput{})
	require.NoError(t, err)
	require.Equal(t, "000000000000", aws.ToString(out.Account))
}
