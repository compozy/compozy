package sshsetup

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const (
	promptEndpointEnv = "COMPOZY_SSH_ASKPASS_ENDPOINT"
	maxAnswer         = 4096
)

type askpassPrompt func(context.Context, string, func(string) error) error

func serveAskpass(ctx context.Context, ask askpassPrompt) ([]string, func(), error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	listener, err := new(net.ListenConfig).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	token := rand.Text()
	done := make(chan struct{})
	// #nosec G118 -- closing a relay cancels the outstanding prompt and joins the accept loop.
	lifetime, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(lifetime, func() { closeAskpass(listener) })
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			relayAnswer(lifetime, conn, token, ask)
		}
	}()
	env := []string{
		"SSH_ASKPASS=" + executable, "SSH_ASKPASS_REQUIRE=force", "DISPLAY=compozy",
		promptEndpointEnv + "=" + listener.Addr().String() + "/" + token,
	}
	return env, func() {
		cancel()
		<-done
		stop()
	}, nil
}

func closeAskpass(closer io.Closer) {
	if err := closer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return
	}
}

func relayAnswer(ctx context.Context, conn net.Conn, token string, ask askpassPrompt) {
	defer closeAskpass(conn)
	stop := context.AfterFunc(ctx, func() { closeAskpass(conn) })
	defer stop()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return
	}
	line, err := bufio.NewReader(io.LimitReader(conn, 128)).ReadString('\n')
	input, kind, ok := strings.Cut(strings.TrimSpace(line), " ")
	if err != nil || !ok || subtle.ConstantTimeCompare([]byte(input), []byte(token)) != 1 {
		return
	}
	if kind != "password" && kind != "otp" && kind != "passphrase" {
		return
	}
	err = ask(ctx, kind, func(value string) error {
		if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			return err
		}
		_, err := fmt.Fprintln(conn, value)
		return err
	})
	if err != nil {
		return
	}
}

// RunAskpass handles the single prompt argument OpenSSH passes to its helper.
func RunAskpass(ctx context.Context, args []string, out io.Writer) (bool, error) {
	endpoint := os.Getenv(promptEndpointEnv)
	if endpoint == "" || len(args) != 1 {
		return false, nil
	}
	address, token, ok := strings.Cut(endpoint, "/")
	if !ok || !strings.HasPrefix(address, "127.0.0.1:") || os.Getenv("SSH_ASKPASS_PROMPT") == "confirm" {
		return true, errors.New("ssh askpass refused the request")
	}
	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp4", address)
	if err != nil {
		return true, err
	}
	defer closeAskpass(conn)
	stop := context.AfterFunc(ctx, func() { closeAskpass(conn) })
	defer stop()
	kind := classifyPrompt(args[0])
	if _, err := fmt.Fprintf(conn, "%s %s\n", token, kind); err != nil {
		return true, err
	}
	if _, err := io.CopyN(out, conn, maxAnswer+2); err != nil && !errors.Is(err, io.EOF) {
		return true, err
	}
	return true, nil
}

func classifyPrompt(prompt string) string {
	prompt = strings.ToLower(prompt)
	switch {
	case strings.Contains(prompt, "passphrase"):
		return "passphrase"
	case strings.Contains(prompt, "code"), strings.Contains(prompt, "otp"), strings.Contains(prompt, "verification"):
		return "otp"
	default:
		return "password"
	}
}
