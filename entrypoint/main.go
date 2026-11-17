// Command entrypoint starts the AWS Emulator in the container.
//
// By default it replaces itself with "aws-emulator -b 0.0.0.0 <args>". The
// process is then the same as with the old ENTRYPOINT.
//
// When one of these variables is set, the program starts the TLS front end:
//
//	EMULATOR_TLS_NAMES    More host names and IP addresses for the certificate,
//	                      separated by commas or spaces. A name can be a
//	                      wildcard, for example "*.localhost".
//	EMULATOR_TLS_CA_CERT  A CA certificate file (PEM) to sign the certificate.
//	EMULATOR_TLS_CA_KEY   The private key file (PEM) of that CA.
//	EMULATOR_HTTP_PORT    A port that also accepts plain HTTP.
//
// The emulator makes a certificate for localhost and 127.0.0.1 only, and it
// has no option to change it. In TLS front end mode the emulator listens on
// 127.0.0.1 in the container. This program listens on the same port on the
// other addresses of the container. It serves a certificate with all the
// names, signed by a CA that it keeps in $HOME/.aws/emulator/tls. It passes
// the bytes of each connection to the emulator without changes.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultPort = 9297

// emulatorBinaries are the places where the packages install the emulator.
var emulatorBinaries = []string{"/usr/bin/aws-emulator", "/opt/aws-emulator/bin/aws-emulator"}

func main() {
	os.Exit(run(os.Args[1:]))
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "aws-emulator-entrypoint: "+format+"\n", args...)
}

func run(args []string) int {
	bin, err := findEmulator()
	if err != nil {
		logf("%v", err)
		return 127
	}
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		logf("%v", err)
		return 2
	}
	if !cfg.enabled() || infoOnly(args) {
		argv := append([]string{"aws-emulator", "-b", "0.0.0.0"}, args...)
		err := syscall.Exec(bin, argv, os.Environ())
		logf("cannot start %s: %v", bin, err)
		return 127
	}
	port, err := emulatorPort(args, os.Getenv("EMULATOR_DATA_PORT"))
	if err != nil {
		logf("%v", err)
		return 2
	}
	if cfg.httpPort == port {
		logf("EMULATOR_HTTP_PORT must not be the emulator port %d", port)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		logf("cannot find the home folder: %v", err)
		return 2
	}
	return runFrontEnd(bin, args, cfg, port, home)
}

func findEmulator() (string, error) {
	if p := os.Getenv("EMULATOR_BINARY"); p != "" {
		return p, nil
	}
	for _, p := range emulatorBinaries {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("aws-emulator"); err == nil {
		return p, nil
	}
	return "", errors.New("cannot find the aws-emulator program")
}

// config holds the TLS front end settings.
type config struct {
	names    []string
	caCert   string
	caKey    string
	httpPort int
}

func (c config) enabled() bool {
	return len(c.names) > 0 || c.caCert != "" || c.httpPort != 0
}

func loadConfig(getenv func(string) string) (config, error) {
	var c config
	c.names = splitNames(getenv("EMULATOR_TLS_NAMES"))
	c.caCert = strings.TrimSpace(getenv("EMULATOR_TLS_CA_CERT"))
	c.caKey = strings.TrimSpace(getenv("EMULATOR_TLS_CA_KEY"))
	if (c.caCert == "") != (c.caKey == "") {
		return c, errors.New("set both EMULATOR_TLS_CA_CERT and EMULATOR_TLS_CA_KEY, or neither")
	}
	if v := strings.TrimSpace(getenv("EMULATOR_HTTP_PORT")); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			return c, fmt.Errorf("EMULATOR_HTTP_PORT %q is not a port from 1 to 65535", v)
		}
		c.httpPort = p
	}
	for _, n := range c.names {
		if err := checkName(n); err != nil {
			return c, err
		}
	}
	return c, nil
}

func splitNames(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	out := make([]string, 0, len(f))
	for _, n := range f {
		out = append(out, strings.ToLower(n))
	}
	return out
}

// infoOnly is true when the arguments only ask for help or the version.
func infoOnly(args []string) bool {
	for _, a := range args {
		switch a {
		case "-h", "--help", "-v", "--version":
			return true
		}
	}
	return false
}

// emulatorPort finds the port in the emulator arguments. It refuses a bind
// address and port 0, because the front end needs both to be fixed.
func emulatorPort(args []string, envPort string) (int, error) {
	value := strings.TrimSpace(envPort)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-p" || a == "--port":
			if i+1 >= len(args) {
				return 0, fmt.Errorf("%s needs a value", a)
			}
			value = args[i+1]
			i++
		case strings.HasPrefix(a, "--port="):
			value = strings.TrimPrefix(a, "--port=")
		case a == "-b" || a == "--bind" || strings.HasPrefix(a, "--bind="):
			return 0, errors.New("do not set the bind address with the TLS front end: the emulator listens on 127.0.0.1 and the front end on the other addresses")
		}
	}
	if value == "" {
		return defaultPort, nil
	}
	p, err := strconv.Atoi(value)
	if err != nil || p < 0 || p > 65535 {
		return 0, fmt.Errorf("the port %q is not a number from 1 to 65535", value)
	}
	if p == 0 {
		return 0, errors.New("port 0 does not work with the TLS front end: set a fixed port")
	}
	return p, nil
}

func runFrontEnd(bin string, args []string, cfg config, port int, home string) int {
	runDir := filepath.Join(home, ".aws", "emulator", strconv.Itoa(port))
	readyFile := filepath.Join(runDir, readyName)
	if err := os.Remove(readyFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		logf("cannot remove %s: %v", readyFile, err)
		return 1
	}

	ca, err := loadOrCreateCA(cfg, filepath.Join(home, ".aws", "emulator", "tls"))
	if err != nil {
		logf("%v", err)
		return 1
	}
	names := certificateNames(cfg.names)
	leaf, err := issueLeaf(ca, names)
	if err != nil {
		logf("cannot make the server certificate: %v", err)
		return 1
	}

	cmd := exec.Command(bin, append([]string{"-b", "127.0.0.1"}, args...)...)
	cmd.Args[0] = "aws-emulator"
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		logf("cannot start %s: %v", bin, err)
		return 127
	}
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	ready := make(chan *frontEnd, 1)
	failed := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		fe, err := startFrontEnd(stop, runDir, port, cfg.httpPort, ca, leaf)
		if err != nil {
			failed <- err
			return
		}
		ready <- fe
	}()

	var fe *frontEnd
	for {
		select {
		case s := <-signals:
			_ = cmd.Process.Signal(s)
		case fe = <-ready:
			logf("TLS front end ready on port %d for %s", port, strings.Join(names, ", "))
			for _, a := range fe.addresses {
				logf("listening on %s", a)
			}
		case err := <-failed:
			logf("%v", err)
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-exited:
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
				<-exited
			}
			return 1
		case err := <-exited:
			close(stop)
			if fe != nil {
				fe.close()
			}
			_ = os.Remove(readyFile)
			return exitCode(err)
		}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return ee.ExitCode()
	}
	logf("%v", err)
	return 1
}
