package sshsetup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

type target struct {
	Alias, Host, User string
	Port              int
}

func (t target) address() string {
	return t.User + "@" + net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
}

func (t target) knownHost() string {
	if t.Port == 22 {
		return t.Host
	}
	return "[" + t.Host + "]:" + strconv.Itoa(t.Port)
}

func (t target) route() store.ComputerRoute {
	return store.ComputerRoute{Kind: remoteproto.RouteSSH, Address: t.address(),
		SSH: &store.ComputerSSHTarget{User: t.User, Port: t.Port, Alias: t.Alias}}
}

func targetFromAddress(alias, address string) (target, error) {
	user, socket, ok := strings.Cut(address, "@")
	host, raw, err := net.SplitHostPort(socket)
	port, portErr := strconv.Atoi(raw)
	t := target{Alias: alias, Host: host, User: user, Port: port}
	if !ok || err != nil || portErr != nil || !t.valid() {
		return target{}, invalidSSHRequest()
	}
	return t, nil
}

func (t target) valid() bool {
	return safeSSHName(t.Alias) && safeSSHName(t.Host) && safeSSHUser(t.User) && t.Port > 0 && t.Port <= 65535
}

func safeSSHName(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-") && !strings.ContainsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune("._-:", r))
	})
}

func safeSSHUser(s string) bool {
	return safeSSHName(s) && !strings.ContainsAny(s, ":%")
}

func invalidSSHRequest() *Failure {
	return failure(
		http.StatusBadRequest,
		"invalid_computer_request",
		"Check the SSH host, user and port and try again.",
	)
}

type sshRunner interface {
	resolve(context.Context, Request) (target, error)
	run(context.Context, []string, io.Reader, []string) (commandResult, error)
}

type commandResult struct {
	stdout, stderr []byte
	code           int
}

type openSSH struct{ configPath string }

func (s openSSH) resolve(ctx context.Context, req Request) (target, error) {
	if !safeSSHName(req.Host) || (req.User != "" && !safeSSHUser(req.User)) || req.Port < 0 || req.Port > 65535 {
		return target{}, invalidSSHRequest()
	}
	ctx, cancel := context.WithTimeout(ctx, connectBudget)
	defer cancel()
	args := []string{"-G", "-o", "CanonicalizeHostname=no"}
	if req.User != "" {
		args = append(args, "-l", req.User)
	}
	if req.Port != 0 {
		args = append(args, "-p", strconv.Itoa(req.Port))
	}
	// OpenSSH runs Match exec even with -G. Keep target discovery local;
	// the operator's route commands still apply to real connections.
	result, err := s.run(ctx, append(args, "--", req.Host), nil, []string{"SHELL=/usr/bin/false"})
	if err != nil {
		return target{}, invalidSSHRequest()
	}
	t := target{Alias: req.Host}
	for line := range strings.SplitSeq(string(result.stdout), "\n") {
		key, value, _ := strings.Cut(line, " ")
		value = strings.TrimSpace(value)
		switch key {
		case "hostname":
			t.Host = strings.ToLower(value)
		case "user":
			t.User = value
		case "port":
			port, parseErr := strconv.Atoi(value)
			if parseErr != nil {
				return target{}, invalidSSHRequest()
			}
			t.Port = port
		}
	}
	if !t.valid() {
		return target{}, invalidSSHRequest()
	}
	return t, nil
}

func (s openSSH) command(ctx context.Context, args []string, stdin io.Reader, env []string) *exec.Cmd {
	if s.configPath != "" {
		args = append([]string{"-F", s.configPath}, args...)
	}
	// #nosec G204 -- fixed OpenSSH executable, validated target and quoted remote arguments.
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = stdin
	cmd.Env = append(os.Environ(), env...)
	cmd.WaitDelay = time.Second
	procutil.ConfigureCommandProcessGroup(cmd)
	cmd.Cancel = func() error { return procutil.KillCommandProcessGroupAndWait(cmd, time.Second) }
	return cmd
}

func (s openSSH) run(ctx context.Context, args []string, stdin io.Reader, env []string) (commandResult, error) {
	cmd := s.command(ctx, args, stdin, env)
	var stdout, stderr sshOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := runProcess(cmd)
	code := 0
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		code = exit.ExitCode()
	}
	return commandResult{stdout.Bytes(), stderr.Bytes(), code}, err
}

func runProcess(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		return errors.Join(err, cmd.Cancel(), cmd.Wait())
	}
	return cmd.Wait()
}

type sshOutput struct{ buffer bytes.Buffer }

func (b *sshOutput) Bytes() []byte { return b.buffer.Bytes() }

func (b *sshOutput) Write(p []byte) (int, error) {
	const limit = 64 << 10
	n := len(p)
	if n > limit {
		p = p[n-limit:]
	}
	if extra := b.buffer.Len() + len(p) - limit; extra > 0 {
		b.buffer.Next(extra)
	}
	_, err := b.buffer.Write(p)
	return n, err
}

func sshArgs(t target, pins string) []string {
	return append(sshOptions(t, pins), "-o", "StrictHostKeyChecking=yes")
}

func sshOptions(t target, pins string) []string {
	return []string{
		"-o", "RemoteCommand=none", "-o", "RequestTTY=no", "-o", "ConnectTimeout=10",
		"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ControlPersist=no",
		"-o", "GlobalKnownHostsFile=none", "-o", "KnownHostsCommand=none", "-o", "CheckHostIP=no",
		"-o", "VerifyHostKeyDNS=no", "-o", "UpdateHostKeys=no", "-o", "PermitLocalCommand=no",
		"-o", "ForwardAgent=no", "-o", "ForwardX11=no", "-o", "Tunnel=no",
		"-o", "ForkAfterAuthentication=no", "-o", "StdinNull=no",
		"-o", "HostKeyAlias=" + t.knownHost(), "-o", "UserKnownHostsFile=" + sshConfigQuote(filepath.ToSlash(pins)),
		"-o", "HostName=" + t.Host, "-l", t.User, "-p", strconv.Itoa(t.Port),
	}
}

func commandArgs(t target, pins, command string) []string {
	args := append(sshArgs(t, pins),
		"-o", "ClearAllForwardings=yes", "-o", "NumberOfPasswordPrompts=3", "-o", "BatchMode=no",
		"-o", "SessionType=default", "--", t.Alias)
	return append(args, command)
}

func sshConfigQuote(path string) string {
	if !strings.ContainsAny(path, " \t\"\\") {
		return path
	}
	return strconv.Quote(path)
}

func shellQuote(word string) string {
	return "'" + strings.ReplaceAll(word, "'", "'\\''") + "'"
}

func sshFailure(t target, result commandResult, err error) error {
	if err == nil {
		return nil
	}
	stderr := string(result.stderr)
	switch {
	case strings.Contains(stderr, "REMOTE HOST IDENTIFICATION HAS CHANGED"),
		strings.Contains(stderr, "Host key verification failed"):
		return failure(http.StatusUnprocessableEntity, "ssh_host_key_changed",
			fmt.Sprintf("%s's key changed since you trusted it.", t.Alias))
	case strings.Contains(stderr, "Permission denied"), strings.Contains(stderr, "Too many authentication failures"):
		return failure(http.StatusUnprocessableEntity, "ssh_auth_failed",
			fmt.Sprintf("SSH authentication failed for %s@%s.", t.User, t.Alias))
	default:
		return errInterrupted
	}
}
