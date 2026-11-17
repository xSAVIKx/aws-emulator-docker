// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const startTimeout = 3 * time.Minute

type frontEnd struct {
	listeners []net.Listener
	addresses []string
	wg        sync.WaitGroup
}

func (f *frontEnd) close() {
	for _, l := range f.listeners {
		_ = l.Close()
	}
}

// startFrontEnd waits for the emulator, writes the trust files, and then
// starts the listeners. A client that can connect can therefore also read
// the new ca.pem.
func startFrontEnd(stop <-chan struct{}, runDir string, port, httpPort int, ca *authority, leaf tls.Certificate) (*frontEnd, error) {
	upstream := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	emulatorCert, protocol, err := waitForEmulator(stop, runDir, upstream)
	if err != nil {
		return nil, err
	}
	if err := trustFiles(runDir, ca, emulatorCert); err != nil {
		return nil, fmt.Errorf("cannot write the trust files in %s: %w", runDir, err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(emulatorCert)
	upstreamTLS := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	protocols := []string{"http/1.1"}
	if protocol == "h2" {
		protocols = []string{"h2", "http/1.1"}
	}
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{leaf},
		NextProtos:   protocols,
		MinVersion:   tls.VersionTLS12,
	}

	fe := &frontEnd{}
	for _, ip := range interfaceAddresses() {
		addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
		l, err := net.Listen("tcp", addr)
		if err != nil {
			logf("cannot listen on %s: %v", addr, err)
			continue
		}
		fe.add(l, "https://"+addr, func(c net.Conn) { serveTLS(c, serverTLS, upstream, upstreamTLS) })
	}
	if len(fe.listeners) == 0 {
		return nil, errors.New("the container has no network address for the TLS front end")
	}
	if httpPort != 0 {
		addr := net.JoinHostPort("", strconv.Itoa(httpPort))
		l, err := net.Listen("tcp", addr)
		if err != nil {
			fe.close()
			return nil, fmt.Errorf("cannot listen on %s for plain HTTP: %w", addr, err)
		}
		fe.add(l, "http://"+l.Addr().String(), func(c net.Conn) { servePlain(c, upstream, upstreamTLS) })
	}
	if err := writeFileAtomic(filepath.Join(runDir, readyName), ca.pem, 0o644); err != nil {
		fe.close()
		return nil, fmt.Errorf("cannot write %s: %w", readyName, err)
	}
	return fe, nil
}

func (f *frontEnd) add(l net.Listener, label string, handle func(net.Conn)) {
	f.listeners = append(f.listeners, l)
	f.addresses = append(f.addresses, label)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				logf("accept on %s: %v", label, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			go handle(c)
		}
	}()
}

// waitForEmulator waits until the emulator answers its health route with
// the certificate in ca.pem. A ca.pem from an earlier run does not match.
func waitForEmulator(stop <-chan struct{}, runDir, upstream string) (*x509.Certificate, string, error) {
	deadline := time.Now().Add(startTimeout)
	var last error
	for time.Now().Before(deadline) {
		select {
		case <-stop:
			return nil, "", errors.New("the emulator stopped")
		default:
		}
		cert, protocol, err := probe(runDir, upstream)
		if err == nil {
			return cert, protocol, nil
		}
		last = err
		time.Sleep(250 * time.Millisecond)
	}
	return nil, "", fmt.Errorf("the emulator did not start in %s: %v", startTimeout, last)
}

func probe(runDir, upstream string) (*x509.Certificate, string, error) {
	data, err := os.ReadFile(filepath.Join(runDir, "ca.pem"))
	if err != nil {
		return nil, "", err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, "", errors.New("ca.pem has no certificate yet")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, "", err
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	cfg := &tls.Config{RootCAs: roots, NextProtos: []string{"h2", "http/1.1"}, MinVersion: tls.VersionTLS12}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", upstream, cfg)
	if err != nil {
		return nil, "", err
	}
	protocol := conn.ConnectionState().NegotiatedProtocol
	_ = conn.Close()

	client := &http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
	resp, err := client.Get("https://" + upstream + "/__admin/health")
	if err != nil {
		return nil, "", err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("health route: HTTP %d", resp.StatusCode)
	}
	return cert, protocol, nil
}

func serveTLS(c net.Conn, serverTLS *tls.Config, upstream string, upstreamTLS *tls.Config) {
	defer c.Close()
	client := tls.Server(c, serverTLS)
	_ = c.SetDeadline(time.Now().Add(15 * time.Second))
	if err := client.Handshake(); err != nil {
		return
	}
	_ = c.SetDeadline(time.Time{})
	cfg := upstreamTLS.Clone()
	if p := client.ConnectionState().NegotiatedProtocol; p != "" {
		cfg.NextProtos = []string{p}
	} else {
		cfg.NextProtos = []string{"http/1.1"}
	}
	server, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", upstream, cfg)
	if err != nil {
		logf("cannot connect to the emulator: %v", err)
		return
	}
	defer server.Close()
	pipe(client, server)
}

func servePlain(c net.Conn, upstream string, upstreamTLS *tls.Config) {
	defer c.Close()
	cfg := upstreamTLS.Clone()
	cfg.NextProtos = []string{"http/1.1"}
	server, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", upstream, cfg)
	if err != nil {
		logf("cannot connect to the emulator: %v", err)
		return
	}
	defer server.Close()
	pipe(c, server)
}

type closeWriter interface{ CloseWrite() error }

// pipe copies the bytes in both directions. When one side stops sending,
// it closes the write half of the other side, so that a response can still
// arrive.
func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	cp := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(closeWriter); ok {
			_ = cw.CloseWrite()
		} else {
			_ = dst.Close()
		}
	}
	wg.Add(2)
	go cp(a, b)
	go cp(b, a)
	wg.Wait()
}

// interfaceAddresses returns the addresses of the container that are not
// loopback and not link-local.
func interfaceAddresses() []net.IP {
	var out []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			out = append(out, ip)
		}
	}
	return out
}
