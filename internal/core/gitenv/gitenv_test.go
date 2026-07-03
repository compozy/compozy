package gitenv

import (
	"context"
	"reflect"
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
			want bool
		}{
			{name: "GIT_DIR", want: true},
			{name: " GIT_WORK_TREE ", want: true},
			{name: "GIT_INDEX_FILE", want: true},
			{name: "GIT_COMMON_DIR", want: true},
			{name: "GIT_OBJECT_DIRECTORY", want: true},
			{name: "GIT_ALTERNATE_OBJECT_DIRECTORIES", want: true},
			{name: "GIT_NAMESPACE", want: true},
			{name: "GIT_PREFIX", want: true},
			{name: "GIT_SSH_COMMAND", want: false},
			{name: "HOME", want: false},
			{name: "", want: false},
		}
		for _, tt := range tests {
			if got := IsRepositoryEnvName(tt.name); got != tt.want {
				t.Fatalf("IsRepositoryEnvName(%q) = %t, want %t", tt.name, got, tt.want)
			}
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
