//go:build mage

package main

import (
	"strings"
	"testing"
)

func TestSourcePolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should require integration filenames to imply the integration tag", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(
			t,
			root,
			"internal/valid/client_integration_test.go",
			"//go:build integration && !windows\n\npackage valid\n",
		)
		writeTestFile(
			t,
			root,
			"cmd/invalid/client_integration_test.go",
			"//go:build !windows\n\npackage invalid\n",
		)

		violations, err := inspectSourcePolicies(root)
		if err != nil {
			t.Fatalf("inspectSourcePolicies() error = %v", err)
		}
		requireSourcePolicyViolation(t, violations, "cmd/invalid/client_integration_test.go", "must imply")
	})

	t.Run("Should reserve provider home key mutation for providerenv", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(
			t,
			root,
			"internal/session/provider.go",
			"package session\n\nimport \"os\"\n\nfunc mutate() error { return os.Setenv(\"HOME\", \"/tmp\") }\n",
		)
		writeTestFile(
			t,
			root,
			"internal/providerenv/env.go",
			"package providerenv\n\nimport \"os\"\n\nfunc mutate() error { return os.Setenv(\"HOME\", \"/tmp\") }\n",
		)

		violations, err := inspectSourcePolicies(root)
		if err != nil {
			t.Fatalf("inspectSourcePolicies() error = %v", err)
		}
		requireSourcePolicyViolation(t, violations, "internal/session/provider.go", "provider home key")
	})

	t.Run("Should reserve raw process control for procutil", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(
			t,
			root,
			"internal/session/process.go",
			"package session\n\nimport \"syscall\"\n\nfunc stop(pid int) error { return syscall.Kill(pid, 0) }\n",
		)
		writeTestFile(
			t,
			root,
			"internal/procutil/process.go",
			"package procutil\n\nimport \"syscall\"\n\nfunc stop(pid int) error { return syscall.Kill(pid, 0) }\n",
		)
		writeTestFile(
			t,
			root,
			"internal/subprocess/process.go",
			"package subprocess\n\nimport \"os\"\n\nfunc stop(process *os.Process) error { return process.Kill() }\n",
		)

		violations, err := inspectSourcePolicies(root)
		if err != nil {
			t.Fatalf("inspectSourcePolicies() error = %v", err)
		}
		requireSourcePolicyViolation(t, violations, "internal/session/process.go", "syscall.Kill")
		requireSourcePolicyViolation(t, violations, "internal/subprocess/process.go", "os.Process.Signal/Kill")
	})

	t.Run("Should reserve SysProcAttr configuration for procutil", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(
			t,
			root,
			"internal/hooks/process.go",
			"package hooks\n\nimport \"os/exec\"\n\nfunc configure(cmd *exec.Cmd) { cmd.SysProcAttr = nil }\n",
		)

		violations, err := inspectSourcePolicies(root)
		if err != nil {
			t.Fatalf("inspectSourcePolicies() error = %v", err)
		}
		requireSourcePolicyViolation(t, violations, "internal/hooks/process.go", "SysProcAttr")
	})

	t.Run("Should ignore unrelated fields named Process or SysProcAttr", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(
			t,
			root,
			"internal/session/worker.go",
			"package session\n\n"+
				"type killer interface { Kill() error }\n"+
				"type worker struct { Process killer; SysProcAttr any }\n\n"+
				"func stop(value *worker) error { value.SysProcAttr = nil; return value.Process.Kill() }\n",
		)

		violations, err := inspectSourcePolicies(root)
		if err != nil {
			t.Fatalf("inspectSourcePolicies() error = %v", err)
		}
		for _, violation := range violations {
			if violation.path == "internal/session/worker.go" {
				t.Fatalf("violations = %#v, want unrelated fields accepted", violations)
			}
		}
	})
}

func requireSourcePolicyViolation(
	t *testing.T,
	violations []sourcePolicyViolation,
	path string,
	messagePart string,
) {
	t.Helper()

	for _, violation := range violations {
		if violation.path == path && strings.Contains(violation.message, messagePart) {
			return
		}
	}
	t.Fatalf("violations = %#v, want %s containing %q", violations, path, messagePart)
}
