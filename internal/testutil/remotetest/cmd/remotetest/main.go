//go:build integration

package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := run(ctx, os.Args[1:], os.Stdout)
	stop()
	os.Exit(status)
}

func run(ctx context.Context, args []string, output io.Writer) (status int) {
	if used, err := sshsetup.RunAskpass(ctx, args, output); used {
		return exitStatus(err)
	}
	if len(args) == 1 && args[0] == "serve-daemon" {
		return exitStatus(runDaemon(ctx))
	}
	if len(args) > 0 && args[0] == "tunnel" {
		return exitStatus(runFixtureConnector(ctx, args))
	}
	flags := flag.NewFlagSet("remotetest", flag.ContinueOnError)
	scenario := flags.String(
		"scenario",
		"",
		"fixture to prepare (ssh-add, account-add, revoke-device, browser-purchase, hosted-purchase, hosted-browser, free-upgrade)",
	)
	repo := flags.String("repo", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return exitStatus(err)
	}
	f := &fixture{ctx: ctx}
	defer func() {
		if problem := recover(); problem != nil {
			if _, aborted := problem.(fixtureAbort); !aborted {
				panic(problem)
			}
		}
		if f.Failed() {
			status = 1
		}
	}()
	defer func() {
		for _, cleanup := range slices.Backward(f.cleanups) {
			cleanup()
		}
	}()
	lab := prepareScenario(f, *repo, *scenario)
	if err := json.MarshalWrite(output, lab); err != nil {
		f.Fatal(err)
	}
	if _, err := fmt.Fprintln(output); err != nil {
		f.Fatal(err)
	}
	<-ctx.Done()
	return 0
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

type fixture struct {
	ctx      context.Context
	cleanups []func()
	failed   bool
}

var _ remotetest.Fixture = (*fixture)(nil)

type fixtureAbort struct{}

func (f *fixture) Context() context.Context { return f.ctx }
func (f *fixture) Helper()                  {}
func (f *fixture) Failed() bool             { return f.failed }
func (f *fixture) Cleanup(cleanup func())   { f.cleanups = append(f.cleanups, cleanup) }

func (f *fixture) TempDir() string {
	dir, err := os.MkdirTemp("", "rc-e2e-")
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			f.Error(err)
		}
	})
	return dir
}

func (f *fixture) Logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (f *fixture) Error(err error) {
	f.Errorf("%v", err)
}

func (f *fixture) Errorf(format string, args ...any) {
	f.failed = true
	f.Logf(format, args...)
}

func (f *fixture) Fatal(args ...any) {
	f.Fatalf("%s", fmt.Sprint(args...))
}

func (f *fixture) Fatalf(format string, args ...any) {
	f.Errorf(format, args...)
	panic(fixtureAbort{})
}
