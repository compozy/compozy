//go:build integration

package scripts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGateEvidenceBehavior(t *testing.T) {
	t.Parallel()

	t.Run("Should fail the lane when tee cannot capture its log", func(t *testing.T) {
		t.Parallel()

		repo := newGateTestRepo(t)
		fakeBin := t.TempDir()
		callsPath := filepath.Join(t.TempDir(), "make-calls")
		writeExecutable(t, fakeBin, "make", `#!/bin/sh
printf '%s\n' "$*" >> "$GATE_TEST_CALLS"
exit 0
`)
		writeExecutable(t, fakeBin, "tee", `#!/bin/sh
cat >/dev/null
exit 23
`)

		output, err := runGate(t, repo, []string{
			"GATE_TEST_CALLS=" + callsPath,
			"PATH=" + fakeBin + ":" + os.Getenv("PATH"),
		}, "full")
		if err == nil {
			t.Fatalf("expected tee failure, output:\n%s", output)
		}

		exitErr, exitErrMatched := errors.AsType[*exec.ExitError](err)
		if !exitErrMatched {
			t.Fatalf("expected gate exit error, got %T: %v", err, err)
		}
		if exitErr.ExitCode() != 23 {
			t.Fatalf("expected tee exit code 23, got %d; output:\n%s", exitErr.ExitCode(), output)
		}

		record := readFile(t, filepath.Join(repo, ".cache", "gate", "full.json"))
		if !strings.Contains(record, `"result": "fail"`) {
			t.Fatalf("expected failed evidence record, got:\n%s", record)
		}
		if strings.Contains(record, `"result": "pass"`) {
			t.Fatalf("tee failure must not produce passing evidence, got:\n%s", record)
		}
	})

	t.Run("Should fail safely when no merge base is usable", func(t *testing.T) {
		t.Parallel()

		repo := newGateTestRepo(t)
		fakeBin := t.TempDir()
		callsPath := filepath.Join(t.TempDir(), "make-calls")
		writeExecutable(t, fakeBin, "make", `#!/bin/sh
printf '%s\n' "$*" >> "$GATE_TEST_CALLS"
exit 0
`)

		output, err := runGate(t, repo, []string{
			"GATE_BASE=refs/heads/missing-gate-base",
			"GATE_TEST_CALLS=" + callsPath,
			"PATH=" + fakeBin + ":" + os.Getenv("PATH"),
		}, "auto")
		if err == nil {
			t.Fatalf("expected classification failure without a merge base; output:\n%s", output)
		}
		if !strings.Contains(output, "no usable merge base") {
			t.Fatalf("expected explicit merge-base reason, got:\n%s", output)
		}
		if !strings.Contains(output, "cannot classify the local gate") {
			t.Fatalf("expected actionable classification error, got:\n%s", output)
		}
		calls, readErr := os.ReadFile(callsPath)
		if readErr != nil && !os.IsNotExist(readErr) {
			t.Fatalf("read gate calls: %v", readErr)
		}
		if strings.Contains(string(calls), "verify") {
			t.Fatalf("missing merge base must not invoke local full verification; calls:\n%s", calls)
		}
	})

	t.Run("Should keep sensitive config changes in local scoped lanes", func(t *testing.T) {
		t.Parallel()

		repo := newGateTestRepo(t)
		configDir := filepath.Join(repo, "internal", "config")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("create config directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "config.go"), []byte("package config\n"), 0o644); err != nil {
			t.Fatalf("write config change: %v", err)
		}

		fakeBin := t.TempDir()
		callsPath := filepath.Join(t.TempDir(), "gate-calls")
		writeExecutable(t, fakeBin, "make", `#!/bin/sh
printf 'make %s\n' "$*" >> "$GATE_TEST_CALLS"
exit 0
`)
		writeExecutable(t, fakeBin, "go", `#!/bin/sh
printf 'go %s\n' "$*" >> "$GATE_TEST_CALLS"
exit 0
`)

		output, err := runGate(t, repo, []string{
			"GATE_TEST_CALLS=" + callsPath,
			"PATH=" + fakeBin + ":" + os.Getenv("PATH"),
		}, "auto")
		if err != nil {
			t.Fatalf("expected scoped gate to succeed: %v\n%s", err, output)
		}
		calls := readFile(t, callsPath)
		if strings.Contains(calls, "make verify") {
			t.Fatalf("sensitive change must not invoke the local full gate; calls:\n%s", calls)
		}
		if !strings.Contains(calls, "make go-lint") {
			t.Fatalf("expected scoped Go lint lane; calls:\n%s", calls)
		}
		if !strings.Contains(calls, "go test -race") || !strings.Contains(calls, "./internal/config/...") {
			t.Fatalf("expected scoped config race tests; calls:\n%s", calls)
		}
		if !strings.Contains(calls, "-timeout 45m") {
			t.Fatalf("expected contention-safe Go test timeout; calls:\n%s", calls)
		}
		if !strings.Contains(output, "PR CI") {
			t.Fatalf("expected delegated full-gate ownership in output; got:\n%s", output)
		}
	})

	t.Run("Should preserve the fingerprint after committing identical content", func(t *testing.T) {
		t.Parallel()

		repo := newGateTestRepo(t)
		if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("updated\n"), 0o644); err != nil {
			t.Fatalf("update repository seed: %v", err)
		}

		before, err := runGate(t, repo, nil, "fingerprint")
		if err != nil {
			t.Fatalf("fingerprint before commit: %v\n%s", err, before)
		}
		runCommand(t, repo, "git", "add", "seed.txt")
		runCommand(
			t,
			repo,
			"git",
			"-c",
			"user.name=Gate Test",
			"-c",
			"user.email=gate-test@example.com",
			"commit",
			"--quiet",
			"-m",
			"update seed",
		)
		after, err := runGate(t, repo, nil, "fingerprint")
		if err != nil {
			t.Fatalf("fingerprint after commit: %v\n%s", err, after)
		}
		if before != after {
			t.Fatalf("fingerprint changed after content-identical commit: before %q, after %q", before, after)
		}
	})

	t.Run("Should rerun a passing lane when its evidence log is missing", func(t *testing.T) {
		t.Parallel()

		repo := newGateTestRepo(t)
		fakeBin := t.TempDir()
		callsPath := filepath.Join(t.TempDir(), "make-calls")
		writeExecutable(t, fakeBin, "make", `#!/bin/sh
printf '%s\n' "$*" >> "$GATE_TEST_CALLS"
exit 0
`)
		env := []string{
			"GATE_TEST_CALLS=" + callsPath,
			"PATH=" + fakeBin + ":" + os.Getenv("PATH"),
		}

		if output, err := runGate(t, repo, env, "full"); err != nil {
			t.Fatalf("first full lane: %v\n%s", err, output)
		}
		record := readFile(t, filepath.Join(repo, ".cache", "gate", "full.json"))
		logPath := jsonRecordField(t, record, "log")
		if err := os.Remove(filepath.Join(repo, logPath)); err != nil {
			t.Fatalf("remove evidence log: %v", err)
		}
		if output, err := runGate(t, repo, env, "full"); err != nil {
			t.Fatalf("second full lane: %v\n%s", err, output)
		}
		calls := strings.Fields(readFile(t, callsPath))
		if len(calls) != 2 {
			t.Fatalf("expected missing log to force two make invocations, got %d: %v", len(calls), calls)
		}
	})

	t.Run("Should classify every sensitive surface without a local full gate", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			path string
			want string
		}{
			{path: "go.mod", want: "go scopes: ./..."},
			{path: "bun.lock", want: "js lane: all workspaces"},
			{path: "Makefile", want: "tooling lanes"},
			{path: "schema.sql", want: "codegen lane"},
			{path: "openapi/schema.yaml", want: "codegen lane"},
			{path: "packages/ui/src/tokens.css", want: "codegen lane"},
			{path: "config.toml", want: "go scopes: ./..."},
			{path: "extensions/spec-cycle/skills/example.md", want: "go scopes: ./extensions/..."},
			{path: "skills/content.go", want: "go scopes: ./skills/..."},
			{path: "desktop/schema/config.ts", want: "js filters: ./desktop"},
			{path: "main.go", want: "go scopes: ./..."},
			{path: "vitest.config.ts", want: "js lane: all workspaces"},
			{path: ".vscode/tasks.json", want: "no-lane"},
			{path: ".repoclone.rc", want: "no-lane"},
		}
		for _, tc := range cases {
			t.Run(tc.path, func(t *testing.T) {
				t.Parallel()
				repo := newGateTestRepo(t)
				path := filepath.Join(repo, filepath.FromSlash(tc.path))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("create parent: %v", err)
				}
				if err := os.WriteFile(path, []byte("changed\n"), 0o644); err != nil {
					t.Fatalf("write changed file: %v", err)
				}
				output, err := runGate(t, repo, nil, "plan")
				if err != nil {
					t.Fatalf("plan: %v\n%s", err, output)
				}
				if strings.Contains(output, "make verify") {
					t.Fatalf("sensitive path planned local full verification:\n%s", output)
				}
				if !strings.Contains(output, tc.want) {
					t.Fatalf("expected %q for %s, got:\n%s", tc.want, tc.path, output)
				}
			})
		}
	})

	t.Run("Should wait for a machine capacity slot before running affected lanes", func(t *testing.T) {
		t.Parallel()

		repo := newGateTestRepo(t)
		configDir := filepath.Join(repo, "internal", "config")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("create config directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "config.go"), []byte("package config\n"), 0o644); err != nil {
			t.Fatalf("write config change: %v", err)
		}
		fakeBin := t.TempDir()
		writeExecutable(t, fakeBin, "make", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, fakeBin, "go", "#!/bin/sh\nexit 0\n")
		slotRoot := t.TempDir()
		heldSlot := filepath.Join(slotRoot, "slot-1")
		if err := os.Mkdir(heldSlot, 0o755); err != nil {
			t.Fatalf("hold capacity slot: %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(heldSlot, "pid"),
			[]byte(strconv.Itoa(os.Getpid())+"\n"),
			0o644,
		); err != nil {
			t.Fatalf("write slot owner: %v", err)
		}
		released := make(chan error, 1)
		go func() {
			time.Sleep(1500 * time.Millisecond)
			if err := os.Remove(filepath.Join(heldSlot, "pid")); err != nil {
				released <- err
				return
			}
			released <- os.Remove(heldSlot)
		}()

		output, err := runGate(t, repo, []string{
			"COMPOZY_GATE_MAX_CONCURRENT=1",
			"COMPOZY_GATE_SLOT_DIR=" + slotRoot,
			"PATH=" + fakeBin + ":" + os.Getenv("PATH"),
		}, "auto")
		if releaseErr := <-released; releaseErr != nil {
			t.Fatalf("release held capacity slot: %v", releaseErr)
		}
		if err != nil {
			t.Fatalf("gate after capacity wait: %v\n%s", err, output)
		}
		if !strings.Contains(output, "waiting for one of 1 machine capacity slots") {
			t.Fatalf("expected visible capacity wait, got:\n%s", output)
		}
	})
}

func newGateTestRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "scripts"), 0o755); err != nil {
		t.Fatalf("create scripts directory: %v", err)
	}
	gateSource, err := os.ReadFile("gate.sh")
	if err != nil {
		t.Fatalf("read gate script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "scripts", "gate.sh"), gateSource, 0o755); err != nil {
		t.Fatalf("write gate script: %v", err)
	}
	runtimeSource, err := os.ReadFile("gate_runtime.sh")
	if err != nil {
		t.Fatalf("read gate runtime support: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "scripts", "gate_runtime.sh"), runtimeSource, 0o755); err != nil {
		t.Fatalf("write gate runtime support: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write repository seed: %v", err)
	}

	runCommand(t, repo, "git", "init", "--quiet")
	runCommand(t, repo, "git", "add", "scripts/gate.sh", "scripts/gate_runtime.sh", "seed.txt")
	runCommand(
		t,
		repo,
		"git",
		"-c",
		"user.name=Gate Test",
		"-c",
		"user.email=gate-test@example.com",
		"commit",
		"--quiet",
		"-m",
		"seed",
	)
	return repo
}

func writeExecutable(t *testing.T, directory, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

func runGate(t *testing.T, repo string, extraEnv []string, mode string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "bash", "scripts/gate.sh", mode)
	cmd.Dir = repo
	cmd.Env = replaceEnv(os.Environ(), append([]string{"GATE_BASE=HEAD"}, extraEnv...)...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func replaceEnv(base []string, replacements ...string) []string {
	values := make(map[string]string, len(base)+len(replacements))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		values[key] = entry
	}
	for _, entry := range replacements {
		key, _, _ := strings.Cut(entry, "=")
		values[key] = entry
	}
	result := make([]string, 0, len(values))
	for _, entry := range values {
		result = append(result, entry)
	}
	return result
}

func jsonRecordField(t *testing.T, record, field string) string {
	t.Helper()
	prefix := `"` + field + `": "`
	start := strings.Index(record, prefix)
	if start < 0 {
		t.Fatalf("record field %q missing:\n%s", field, record)
	}
	value := record[start+len(prefix):]
	end := strings.IndexByte(value, '"')
	if end < 0 {
		t.Fatalf("record field %q is unterminated:\n%s", field, record)
	}
	return value[:end]
}

func runCommand(t *testing.T, directory, name string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = directory
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run %s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}
