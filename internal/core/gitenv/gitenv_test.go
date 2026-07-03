package gitenv

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestSanitizedEnv(t *testing.T) {
	t.Run("Should remove repository env vars and preserve transport env vars", func(t *testing.T) {
		t.Setenv("GIT_DIR", "/wrong/.git")
		t.Setenv("GIT_WORK_TREE", "/wrong")
		t.Setenv("GIT_INDEX_FILE", "/wrong/.git/index")
		t.Setenv("GIT_SSH_COMMAND", "ssh -i key")

		env := SanitizedEnv()
		for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
			if hasEnvName(env, name) {
				t.Fatalf("SanitizedEnv() retained %s", name)
			}
		}
		if !hasEnvName(env, "GIT_SSH_COMMAND") {
			t.Fatal("SanitizedEnv() removed GIT_SSH_COMMAND")
		}
	})
}

func TestIsRepositoryEnvName(t *testing.T) {
	t.Run("Should identify repository-scoped git env vars", func(t *testing.T) {
		tests := []struct {
			name string
			env  string
			want bool
		}{
			{name: "Should identify GIT_DIR as repository scoped", env: "GIT_DIR", want: true},
			{name: "Should trim and identify GIT_WORK_TREE as repository scoped", env: " GIT_WORK_TREE ", want: true},
			{name: "Should identify GIT_INDEX_FILE as repository scoped", env: "GIT_INDEX_FILE", want: true},
			{name: "Should identify GIT_COMMON_DIR as repository scoped", env: "GIT_COMMON_DIR", want: true},
			{
				name: "Should identify GIT_OBJECT_DIRECTORY as repository scoped",
				env:  "GIT_OBJECT_DIRECTORY",
				want: true,
			},
			{
				name: "Should identify GIT_ALTERNATE_OBJECT_DIRECTORIES as repository scoped",
				env:  "GIT_ALTERNATE_OBJECT_DIRECTORIES",
				want: true,
			},
			{name: "Should identify GIT_NAMESPACE as repository scoped", env: "GIT_NAMESPACE", want: true},
			{name: "Should identify GIT_PREFIX as repository scoped", env: "GIT_PREFIX", want: true},
			{name: "Should preserve GIT_SSH_COMMAND as transport scoped", env: "GIT_SSH_COMMAND", want: false},
			{name: "Should ignore HOME as unrelated", env: "HOME", want: false},
			{name: "Should ignore an empty env name", env: "", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				if got := IsRepositoryEnvName(tt.env); got != tt.want {
					t.Fatalf("IsRepositoryEnvName(%q) = %t, want %t", tt.env, got, tt.want)
				}
			})
		}
	})
}

func TestCommand(t *testing.T) {
	t.Run("Should pin git to the provided directory with sanitized env", func(t *testing.T) {
		t.Setenv("GIT_DIR", "/wrong/.git")

		cmd := Command(context.Background(), " /repo ", "status", "--short")
		wantArgs := []string{"git", "status", "--short"}
		if !reflect.DeepEqual(cmd.Args, wantArgs) {
			t.Fatalf("Command() args = %#v, want %#v", cmd.Args, wantArgs)
		}
		if cmd.Dir != "/repo" {
			t.Fatalf("Command() dir = %q, want /repo", cmd.Dir)
		}
		if hasEnvName(cmd.Env, "GIT_DIR") {
			t.Fatal("Command() retained GIT_DIR")
		}
	})
}

func TestRun(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	t.Run("Should return trimmed stdout on success", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		if _, err := Run(context.Background(), dir, "init", "-q"); err != nil {
			t.Fatalf("Run(init) error = %v", err)
		}

		out, err := Run(context.Background(), dir, "rev-parse", "--show-toplevel")
		if err != nil {
			t.Fatalf("Run(rev-parse) error = %v", err)
		}
		if out == "" {
			t.Fatal("Run(rev-parse) returned empty output")
		}
		if strings.TrimSpace(out) != out {
			t.Fatalf("Run(rev-parse) output = %q, want trimmed output", out)
		}
	})

	t.Run("Should wrap error with stderr message on failure", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		if _, err := Run(context.Background(), dir, "init", "-q"); err != nil {
			t.Fatalf("Run(init) error = %v", err)
		}

		_, err := Run(context.Background(), dir, "rev-parse", "--verify", "nope")
		if err == nil {
			t.Fatal("Run(rev-parse) error = nil, want error")
		}
		got := err.Error()
		for _, want := range []string{"git rev-parse --verify nope", dir, "nope"} {
			if !strings.Contains(got, want) {
				t.Fatalf("Run(rev-parse) error = %q, want it to contain %q", got, want)
			}
		}
	})

	t.Run("Should wrap empty output errors with command context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		dir := t.TempDir()
		_, err := Run(ctx, dir, "status", "--short")
		if err == nil {
			t.Fatal("Run(status) error = nil, want error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run(status) error = %v, want context.Canceled", err)
		}
		got := err.Error()
		for _, want := range []string{"git status --short", dir} {
			if !strings.Contains(got, want) {
				t.Fatalf("Run(status) error = %q, want it to contain %q", got, want)
			}
		}
	})
}

func hasEnvName(env []string, name string) bool {
	for _, entry := range env {
		if envEntryName(entry) == name {
			return true
		}
	}
	return false
}

func envEntryName(entry string) string {
	for index, char := range entry {
		if char == '=' {
			return entry[:index]
		}
	}
	return entry
}
