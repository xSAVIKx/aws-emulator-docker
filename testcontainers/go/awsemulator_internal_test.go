// SPDX-License-Identifier: Apache-2.0

package awsemulator

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func TestParseCredentials(t *testing.T) {
	id, secret, err := parseCredentials([]byte("[other]\naws_access_key_id = x\n\n[default]\naws_access_key_id = AKIDEXAMPLE\naws_secret_access_key=secret/with=equals\n"))
	require.NoError(t, err)
	require.Equal(t, "AKIDEXAMPLE", id)
	require.Equal(t, "secret/with=equals", secret)

	_, _, err = parseCredentials([]byte("[default]\naws_access_key_id = AKIDEXAMPLE\n"))
	require.Error(t, err)
}

func TestFinalize(t *testing.T) {
	for _, tc := range []struct {
		name      string
		settings  options
		req       testcontainers.GenericContainerRequest
		wantNames string // without the daemon host
		telemetry bool   // EMULATOR_TELEMETRY is absent
	}{
		{
			name: "aliases and extra names",
			settings: options{
				tlsNames: []string{"emulator.test", "aws"},
			},
			req: testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
				Env:            map[string]string{"EMULATOR_TELEMETRY": "off"},
				Networks:       []string{"n1", "n2"},
				NetworkAliases: map[string][]string{"n1": {"aws"}, "n2": {"s3", "sqs"}},
			}},
			wantNames: "aws,s3,sqs,emulator.test",
		},
		{
			name:      "telemetry on",
			settings:  options{telemetry: true},
			req:       testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Env: map[string]string{"EMULATOR_TELEMETRY": "off"}}},
			telemetry: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, finalize(t.Context(), tc.settings)(&tc.req))
			names := tc.req.Env["EMULATOR_TLS_NAMES"]
			require.NotEmpty(t, names)
			host := daemonHost(t.Context(), tc.req.ProviderType)
			want := tc.wantNames
			switch {
			case host != "" && want != "":
				want = host + "," + want
			case host != "":
				want = host
			case want == "":
				want = "localhost"
			}
			require.Equal(t, want, names)
			_, set := tc.req.Env["EMULATOR_TELEMETRY"]
			require.Equal(t, !tc.telemetry, set)
		})
	}
}

func TestWithTLSNames(t *testing.T) {
	var o options
	require.NoError(t, WithTLSNames("a.test", "10.0.0.1")(&o))
	require.Equal(t, []string{"a.test", "10.0.0.1"}, o.tlsNames)
	require.Error(t, WithTLSNames("")(&o))
	require.Error(t, WithTLSNames("a,b")(&o))
}

func TestUniqueNames(t *testing.T) {
	require.Equal(t, []string{"a", "b"}, uniqueNames([]string{"a", "", " b ", "a"}))
	require.Empty(t, uniqueNames(nil))
}
