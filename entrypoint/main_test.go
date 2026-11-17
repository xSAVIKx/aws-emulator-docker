// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfig(t *testing.T) {
	c, err := loadConfig(env(nil))
	if err != nil || c.enabled() {
		t.Fatalf("empty env: enabled=%v err=%v", c.enabled(), err)
	}
	c, err = loadConfig(env(map[string]string{"EMULATOR_TLS_NAMES": "aws, *.Localstack.cloud  10.0.0.5"}))
	if err != nil || !c.enabled() {
		t.Fatalf("names: enabled=%v err=%v", c.enabled(), err)
	}
	want := []string{"aws", "*.localstack.cloud", "10.0.0.5"}
	if len(c.names) != len(want) {
		t.Fatalf("names = %v, want %v", c.names, want)
	}
	for i := range want {
		if c.names[i] != want[i] {
			t.Fatalf("names = %v, want %v", c.names, want)
		}
	}
	for _, bad := range []map[string]string{
		{"EMULATOR_TLS_CA_CERT": "/x"},
		{"EMULATOR_HTTP_PORT": "0"},
		{"EMULATOR_HTTP_PORT": "http"},
		{"EMULATOR_TLS_NAMES": "a*b"},
		{"EMULATOR_TLS_NAMES": "-bad"},
		{"EMULATOR_TLS_NAMES": "a..b"},
	} {
		if _, err := loadConfig(env(bad)); err == nil {
			t.Errorf("loadConfig(%v): no error", bad)
		}
	}
	c, err = loadConfig(env(map[string]string{"EMULATOR_HTTP_PORT": "4566"}))
	if err != nil || c.httpPort != 4566 || !c.enabled() {
		t.Fatalf("http port: %+v %v", c, err)
	}
}

func TestEmulatorPort(t *testing.T) {
	cases := []struct {
		args []string
		env  string
		want int
		ok   bool
	}{
		{nil, "", 9297, true},
		{nil, "1234", 1234, true},
		{[]string{"-p", "4566", "-t", "off"}, "1234", 4566, true},
		{[]string{"--port=4567"}, "", 4567, true},
		{[]string{"--port", "4568"}, "", 4568, true},
		{[]string{"-p", "0"}, "", 0, false},
		{[]string{"-p"}, "", 0, false},
		{[]string{"-p", "x"}, "", 0, false},
		{[]string{"-b", "0.0.0.0"}, "", 0, false},
		{[]string{"--bind=127.0.0.1"}, "", 0, false},
	}
	for _, c := range cases {
		got, err := emulatorPort(c.args, c.env)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("emulatorPort(%v, %q) = %d, %v", c.args, c.env, got, err)
		}
	}
}

func TestInfoOnly(t *testing.T) {
	if !infoOnly([]string{"--version"}) || !infoOnly([]string{"-h"}) || infoOnly([]string{"-p", "1"}) {
		t.Fatal("infoOnly is wrong")
	}
}

func TestCAAndLeaf(t *testing.T) {
	dir := t.TempDir()
	ca, err := loadOrCreateCA(config{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dir, "ca-key.pem")); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file: %v %v", st, err)
	}
	again, err := loadOrCreateCA(config{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !again.cert.Equal(ca.cert) {
		t.Fatal("the saved CA was not used again")
	}
	explicit, err := loadOrCreateCA(config{caCert: filepath.Join(dir, "ca.pem"), caKey: filepath.Join(dir, "ca-key.pem")}, t.TempDir())
	if err != nil || !explicit.cert.Equal(ca.cert) {
		t.Fatalf("explicit CA: %v", err)
	}

	names := certificateNames([]string{"aws", "*.example.test", "10.1.2.3"})
	leaf, err := issueLeaf(ca, names)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.cert)
	for _, host := range []string{"localhost", "000000000000.localhost", "127.0.0.1", "::1", "aws", "s3.example.test", "10.1.2.3"} {
		if _, err := leaf.Leaf.Verify(x509.VerifyOptions{DNSName: host, Roots: roots}); err != nil {
			t.Errorf("verify %s: %v", host, err)
		}
	}
	if _, err := leaf.Leaf.Verify(x509.VerifyOptions{DNSName: "other", Roots: roots}); err == nil {
		t.Error("verify other: no error")
	}
	if _, err := tls.X509KeyPair(ca.pem, ca.pem); err == nil {
		t.Error("X509KeyPair accepted a certificate as a key")
	}
}

func TestTrustFiles(t *testing.T) {
	dir := t.TempDir()
	ca, err := loadOrCreateCA(config{}, filepath.Join(dir, "tls"))
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := newCA()
	if err != nil {
		t.Fatal(err)
	}
	if err := trustFiles(dir, ca, other.cert); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		t.Fatal("ca.pem has no certificates")
	}
	store, err := os.ReadFile(filepath.Join(dir, "truststore.p12"))
	if err != nil {
		t.Fatal(err)
	}
	certs, err := pkcs12.DecodeTrustStore(store, "")
	if err != nil || len(certs) != 2 || !certs[0].Equal(ca.cert) {
		t.Fatalf("truststore: %d certificates, %v", len(certs), err)
	}
}
