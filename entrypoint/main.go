// Command entrypoint starts the AWS Emulator in the container.
//
// By default it replaces itself with "aws-emulator -b 0.0.0.0 <args>". The
// process is then the same as with the old ENTRYPOINT.
//
// When one of these variables is set, the program starts the TLS front end:
//
//	EMULATOR_TLS_NAMES    More host names and IP addresses for the certificate,
//	                      separated by commas or spaces. A name can be a
//	                      wildcard, for example "*.example.test".
//	EMULATOR_TLS_CA_CERT  A CA certificate file (PEM) to sign the certificate.
//	EMULATOR_TLS_CA_KEY   The private key file (PEM) of that CA.
//	EMULATOR_HTTP_PORT    A port that also accepts plain HTTP.
//
// The emulator makes a certificate for localhost and 127.0.0.1 only, and it
// has no option to change it. In TLS front end mode the emulator listens on
// 127.0.0.1 in the container. This program listens on the same port on the
// other addresses of the container. It serves a certificate with all the
// names, signed by a CA that it keeps in $HOME/.aws/emulator-ca. It passes
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
// The program looks in PATH first, as the old ENTRYPOINT did.
var emulatorBinaries = []string{"/usr/bin/aws-emulator", "/opt/aws-emulator/bin/aws-emulator"}

// tlsVariables turn on the TLS front end. The health checks use the same list.
var tlsVariables = []string{"EMULATOR_TLS_NAMES", "EMULATOR_TLS_CA_CERT", "EMULATOR_TLS_CA_KEY", "EMULATOR_HTTP_PORT"}

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
	parsed, err := parseArgs(args)
	if err != nil && cfg.enabled() {
		logf("%v", err)
		return 2
	}
	if !cfg.enabled() || parsed.info {
		argv := append([]string{"aws-emulator", "-b", "0.0.0.0"}, args...)
		err := syscall.Exec(bin, argv, os.Environ())
		logf("cannot start %s: %v", bin, err)
		return 127
	}
	port, err := parsed.frontEndPort(os.Getenv("EMULATOR_DATA_PORT"))
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
	if p, err := exec.LookPath("aws-emulator"); err == nil {
		return p, nil
	}
	for _, p := range emulatorBinaries {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
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

// loadConfig reads the variables. A variable that is set to a value with
// only spaces or commas is an error: the health checks treat any non-empty
// value as "front end on", so the entrypoint must not silently ignore it.
func loadConfig(getenv func(string) string) (config, error) {
	var c config
	for _, v := range tlsVariables {
		if raw := getenv(v); raw != "" && strings.Trim(raw, " ,\t\n") == "" {
			return c, fmt.Errorf("%s has no value: remove it, or give it a value", v)
		}
	}
	c.names = splitNames(getenv("EMULATOR_TLS_NAMES"))
	c.caCert = strings.TrimSpace(getenv("EMULATOR_TLS_CA_CERT"))
	c.caKey = strings.TrimSpace(getenv("EMULATOR_TLS_CA_KEY"))
	if (c.caCert == "") != (c.caKey == "") {
		return c, errors.New("set both EMULATOR_TLS_CA_CERT and EMULATOR_TLS_CA_KEY, or neither")
	}
	if v := strings.TrimSpace(getenv("EMULATOR_HTTP_PORT")); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 || strings.HasPrefix(v, "+") {
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

// args is what the entrypoint needs to know from the emulator arguments.
type args struct {
	info    bool   // -h, --help, -v or --version
	port    string // the last port value, "" when not given
	hasPort bool
	hasBind bool
}

// valueFlags are the emulator options that take a value (see "aws-emulator --help").
var valueFlags = map[string]bool{
	"-p": true, "--port": true, "-b": true, "--bind": true,
	"-t": true, "--telemetry": true, "-a": true, "--auth": true, "--idle-timeout": true,
}

// parseArgs reads the emulator arguments in the forms "-p 1", "-p1", "-p=1",
// "--port 1" and "--port=1". It stops at "--".
func parseArgs(list []string) (args, error) {
	var a args
	for i := 0; i < len(list); i++ {
		arg := list[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		name, value, inline := arg, "", false
		if strings.HasPrefix(arg, "--") {
			if k, v, ok := strings.Cut(arg, "="); ok {
				name, value, inline = k, v, true
			}
		} else if len(arg) > 2 {
			name, value, inline = arg[:2], strings.TrimPrefix(arg[2:], "="), true
		}
		switch name {
		case "-h", "--help", "-v", "--version":
			if !inline {
				a.info = true
			}
			continue
		}
		if !valueFlags[name] {
			continue
		}
		if !inline {
			if i+1 >= len(list) {
				return a, fmt.Errorf("%s needs a value", name)
			}
			i++
			value = list[i]
		}
		switch name {
		case "-p", "--port":
			a.port, a.hasPort = value, true
		case "-b", "--bind":
			a.hasBind = true
		}
	}
	return a, nil
}

// frontEndPort returns the emulator port. It refuses a bind address and
// port 0, because the front end needs both to be fixed.
func (a args) frontEndPort(envPort string) (int, error) {
	if a.hasBind {
		return 0, errors.New("do not set the bind address with the TLS front end: the emulator listens on 127.0.0.1 and the front end on the other addresses")
	}
	value := strings.TrimSpace(envPort)
	if a.hasPort {
		value = a.port
		if value == "" {
			return 0, errors.New("the port option has no value")
		}
	}
	if value == "" {
		return defaultPort, nil
	}
	p, err := strconv.Atoi(value)
	if err != nil || p < 0 || p > 65535 || strings.HasPrefix(value, "+") {
		return 0, fmt.Errorf("the port %q is not a number from 1 to 65535", value)
	}
	if p == 0 {
		return 0, errors.New("port 0 does not work with the TLS front end: set a fixed port")
	}
	return p, nil
}

func runFrontEnd(bin string, emulatorArgs []string, cfg config, port int, home string) int {
	runDir := filepath.Join(home, ".aws", "emulator", strconv.Itoa(port))
	readyFile := filepath.Join(runDir, readyName)
	if err := os.Remove(readyFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		logf("cannot remove %s: %v", readyFile, err)
		return 1
	}

	ca, err := loadOrCreateCA(cfg, filepath.Join(home, ".aws", "emulator-ca"))
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

	// Catch the signals before the start, so that none is lost.
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGCHLD)

	cmd := exec.Command(bin, append([]string{"-b", "127.0.0.1"}, emulatorArgs...)...)
	cmd.Args[0] = "aws-emulator"
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// A process group of its own: Ctrl+C on a terminal then reaches the
	// emulator one time only, through this program.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		logf("cannot start %s: %v", bin, err)
		return 127
	}
	child := cmd.Process.Pid

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

	// A safety net: a SIGCHLD can merge with others or get lost.
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	var fe *frontEnd
	finish := func(status syscall.WaitStatus) int {
		close(stop)
		if fe != nil {
			fe.close()
		}
		_ = os.Remove(readyFile)
		return exitCode(status)
	}
	for {
		select {
		case s := <-signals:
			if s == syscall.SIGCHLD {
				// This program can be PID 1: reap all children, also
				// orphans that the emulator leaves.
				if status, done := reap(child); done {
					return finish(status)
				}
				continue
			}
			_ = syscall.Kill(child, s.(syscall.Signal))
		case <-tick.C:
			if status, done := reap(child); done {
				return finish(status)
			}
		case fe = <-ready:
			logf("TLS front end ready on port %d for %s", port, strings.Join(names, ", "))
			for _, a := range fe.addresses {
				logf("listening on %s", a)
			}
		case err := <-failed:
			logf("%v", err)
			_ = syscall.Kill(child, syscall.SIGTERM)
			deadline := time.After(10 * time.Second)
			for {
				select {
				case s := <-signals:
					if s == syscall.SIGCHLD {
						if _, done := reap(child); done {
							_ = os.Remove(readyFile)
							return 1
						}
					}
				case <-deadline:
					_ = syscall.Kill(child, syscall.SIGKILL)
					deadline = nil
				}
			}
		}
	}
}

// reap collects every child that has exited. It reports the status of the
// emulator when the emulator is one of them.
func reap(child int) (syscall.WaitStatus, bool) {
	var result syscall.WaitStatus
	found := false
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if err == syscall.EINTR {
			continue
		}
		if pid <= 0 || err != nil {
			return result, found
		}
		if pid == child {
			result, found = status, true
		}
	}
}

func exitCode(status syscall.WaitStatus) int {
	if status.Signaled() {
		return 128 + int(status.Signal())
	}
	return status.ExitStatus()
}
