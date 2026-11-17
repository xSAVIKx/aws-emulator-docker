// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// readyName is the file that the front end writes when it accepts
// connections. It holds the CA certificate. The health check looks for it.
const readyName = "tls-ca.pem"

type authority struct {
	cert *x509.Certificate
	key  crypto.Signer
	pem  []byte
}

// loadOrCreateCA loads the CA from the configured files. Without them, it
// loads the CA from dir, or makes a new CA and saves it in dir. A saved CA
// stays the same when dir is on a volume, so a client can trust it once.
// A lock file stops two containers that share dir from writing two CAs.
func loadOrCreateCA(cfg config, dir string) (*authority, error) {
	if cfg.caCert != "" {
		return loadCA(cfg.caCert, cfg.caKey)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		logf("cannot make %s, so the CA is only in memory and changes at each start: %v", dir, err)
		ca, _, err := newCA()
		return ca, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot open the CA lock file: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return nil, fmt.Errorf("cannot lock the CA folder: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck

	certFile, keyFile := filepath.Join(dir, "ca.pem"), filepath.Join(dir, "ca-key.pem")
	_, err = os.Stat(certFile)
	switch {
	case err == nil:
		return loadCA(certFile, keyFile)
	case !errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("cannot read the CA in %s: %w", dir, err)
	}
	ca, keyPEM, err := newCA()
	if err != nil {
		return nil, fmt.Errorf("cannot make a CA: %w", err)
	}
	// The key first: a certificate file without its key never exists.
	if err := writeFileAtomic(keyFile, keyPEM, 0o600); err != nil {
		logf("cannot save the CA key in %s, so the CA is only in memory: %v", dir, err)
		return ca, nil
	}
	if err := writeFileAtomic(certFile, ca.pem, 0o644); err != nil {
		_ = os.Remove(keyFile)
		logf("cannot save the CA in %s, so the CA is only in memory: %v", dir, err)
	}
	return ca, nil
}

func newCA() (*authority, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "AWS Emulator local CA", Organization: []string{"aws-emulator-docker"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return &authority{cert: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}, keyPEM, nil
}

func loadCA(certFile, keyFile string) (*authority, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read the CA certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read the CA key: %w", err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("the CA files %s and %s are not a valid pair: %w", certFile, keyFile, err)
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("cannot parse the CA certificate: %w", err)
	}
	if !cert.IsCA {
		return nil, fmt.Errorf("the certificate in %s is not a CA certificate", certFile)
	}
	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, errors.New("the CA key cannot sign")
	}
	return &authority{cert: cert, key: signer, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})}, nil
}

// certificateNames returns the default names, the container host name and
// addresses, and the names from EMULATOR_TLS_NAMES, without duplicates.
func certificateNames(extra []string) []string {
	// OpenSSL and curl do not accept a wildcard directly under a top-level
	// name such as "*.localhost". The SDKs put the account ID in front of
	// the host for S3 Control, so the list also has that exact name.
	names := []string{"localhost", "*.localhost", "000000000000.localhost", "127.0.0.1", "::1"}
	if h, err := os.Hostname(); err == nil && h != "" {
		names = append(names, strings.ToLower(h))
	}
	for _, ip := range interfaceAddresses() {
		names = append(names, ip.String())
	}
	names = append(names, extra...)
	seen := map[string]bool{}
	out := names[:0]
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// checkName accepts an IP address, a host name, or a wildcard host name
// with "*." at the start.
func checkName(n string) error {
	if net.ParseIP(n) != nil {
		return nil
	}
	host := strings.TrimPrefix(n, "*.")
	if host == "" || len(host) > 253 || strings.Contains(host, "*") {
		return fmt.Errorf("EMULATOR_TLS_NAMES: %q is not a host name or an IP address", n)
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("EMULATOR_TLS_NAMES: %q is not a host name or an IP address", n)
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return fmt.Errorf("EMULATOR_TLS_NAMES: %q is not a host name or an IP address", n)
			}
		}
	}
	return nil
}

func issueLeaf(ca *authority, names []string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	notAfter := now.AddDate(1, 0, 0)
	if ca.cert.NotAfter.Before(notAfter) {
		notAfter = ca.cert.NotAfter
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: "aws-emulator"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, key.Public(), ca.key)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der, ca.cert.Raw}, PrivateKey: key, Leaf: leaf}, nil
}

// trustFiles rewrites ca.pem and truststore.p12 in the run folder. Both then
// hold the front end CA first and the emulator certificate second. Clients
// that connect through the front end need the CA. Clients in the same
// network namespace that use 127.0.0.1 reach the emulator directly and need
// its certificate.
func trustFiles(runDir string, ca *authority, emulatorCert *x509.Certificate) error {
	bundle := append(append([]byte{}, ca.pem...), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: emulatorCert.Raw})...)
	if err := writeFileAtomic(filepath.Join(runDir, "ca.pem"), bundle, 0o600); err != nil {
		return err
	}
	store, err := pkcs12.Passwordless.EncodeTrustStoreEntries([]pkcs12.TrustStoreEntry{
		{Cert: ca.cert, FriendlyName: "aws-emulator-ca"},
		{Cert: emulatorCert, FriendlyName: "aws-emulator"},
	}, "")
	if err != nil {
		return fmt.Errorf("cannot make the Java truststore: %w", err)
	}
	return writeFileAtomic(filepath.Join(runDir, "truststore.p12"), store, 0o600)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		panic(err)
	}
	return n
}
