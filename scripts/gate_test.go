package scripts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
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

	t.Run("Should escalate to the full gate when no merge base is usable", func(t *testing.T) {
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
		if err != nil {
			t.Fatalf("expected gate escalation to succeed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "no usable merge base") {
			t.Fatalf("expected explicit merge-base escalation reason, got:\n%s", output)
		}
		if got := strings.TrimSpace(readFile(t, callsPath)); got != "verify" {
			t.Fatalf("expected make verify escalation, got %q", got)
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
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write repository seed: %v", err)
	}

	runCommand(t, repo, "git", "init", "--quiet")
	runCommand(t, repo, "git", "add", "scripts/gate.sh", "seed.txt")
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
	cmd.Env = append(os.Environ(), extraEnv...)
	output, err := cmd.CombinedOutput()
	return string(output), err
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
