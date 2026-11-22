// Command healthcheck calls the AWS Emulator health route on 127.0.0.1.
//
// The distroless image has no shell and no curl, so the Docker HEALTHCHECK
// uses this small static program instead of healthcheck.sh. It reads the
// port from EMULATOR_HEALTHCHECK_PORT (default 9297) and exits with 0 when
// the route returns HTTP 200.
//
// The emulator creates a new self-signed certificate at each start, so the
// program skips certificate verification. It calls only the loopback address.
// With the TLS front end, it also waits until the front end is ready.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	port := os.Getenv("EMULATOR_HEALTHCHECK_PORT")
	if port == "" {
		port = "9297"
	}
	if os.Getenv("EMULATOR_TLS_NAMES")+os.Getenv("EMULATOR_TLS_CA_CERT")+os.Getenv("EMULATOR_HTTP_PORT") != "" {
		home, _ := os.UserHomeDir()
		if st, err := os.Stat(filepath.Join(home, ".aws", "emulator", port, "tls-ca.pem")); err != nil || st.Size() == 0 {
			fmt.Fprintln(os.Stderr, "unhealthy: the TLS front end is not ready")
			os.Exit(1)
		}
	}
	client := &http.Client{
		Timeout: 4 * time.Second,
		Transport: &http.Transport{
			// #nosec G402 -- loopback health check of a self-signed endpoint.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get("https://127.0.0.1:" + port + "/__admin/health")
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy: HTTP", resp.StatusCode)
		os.Exit(1)
	}
}
