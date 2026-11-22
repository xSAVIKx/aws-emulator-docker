// SPDX-License-Identifier: Apache-2.0

package awsemulator

import (
	"fmt"
	"strings"

	"github.com/testcontainers/testcontainers-go"
)

// options holds the settings that the module options change.
type options struct {
	tlsNames  []string
	telemetry bool
}

func defaultOptions() options {
	return options{}
}

// Compile-time check: an Option is also a testcontainers.ContainerCustomizer.
var _ testcontainers.ContainerCustomizer = (Option)(nil)

// Option is an option for the AWS Emulator container. Pass it to [Run]
// together with the generic testcontainers options.
type Option func(*options) error

// Customize does nothing. It makes Option a testcontainers.ContainerCustomizer.
// [Run] applies the Option values before it builds the request.
func (o Option) Customize(*testcontainers.GenericContainerRequest) error {
	return nil
}

// WithTLSNames adds host names or IP addresses to the TLS certificate of the
// container. Use it when a client reaches the container through a name that
// the module does not know, for example a DNS name of a remote Docker host.
//
// The module always adds the Testcontainers host and all network aliases of
// the container. An empty name or a name with a comma or a space is an error.
func WithTLSNames(names ...string) Option {
	return func(o *options) error {
		for _, n := range names {
			if n == "" || strings.ContainsAny(n, ", \t\r\n") {
				return fmt.Errorf("awsemulator: invalid TLS name %q", n)
			}
			o.tlsNames = append(o.tlsNames, n)
		}
		return nil
	}
}

// WithTelemetry turns the telemetry of the emulator on or off. The default
// is off: the module sets EMULATOR_TELEMETRY=off. With on, the module does not
// set the variable, and the emulator uses its own default.
func WithTelemetry(on bool) Option {
	return func(o *options) error {
		o.telemetry = on
		return nil
	}
}
