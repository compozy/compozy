package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWritesStructuredLogsToFile(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "logs", "agh.log")

	log, closeFn, err := New(WithLevel("debug"), WithFile(logFile), WithMirrorToStderr(false))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	log.Info("hello", "component", "test")

	if err := closeFn(); err != nil {
		t.Fatalf("closeFn() error = %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"msg":"hello"`) {
		t.Fatalf("log file = %q, want hello message", string(data))
	}
}

func TestNewRedactsMessageAndContentAttributesWithoutChangingCorrelation(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "logs", "agh.log")
	log, closeFn, err := New(WithFile(logFile), WithMirrorToStderr(false))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := closeFn(); err != nil {
			t.Errorf("closeFn() error = %v", err)
		}
	})

	secret := "sk-ant-api03-abcdefghijklmnopqrstuv"
	wantEnvelope := map[string]string{
		"claim_token_hash": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"session_id":       "550e8400-e29b-41d4-a716-446655440000",
		"run_id":           "62f82910-18ca-4f2e-aa4a-54dcde9fe761",
	}
	log.Info(
		"provider returned "+secret,
		"detail", "tool output "+secret,
		"payload", map[string]any{
			"content":          "nested tool output " + secret,
			"claim_token_hash": wantEnvelope["claim_token_hash"],
		},
		"claim_token_hash", wantEnvelope["claim_token_hash"],
		"session_id", wantEnvelope["session_id"],
		"run_id", wantEnvelope["run_id"],
	)
	if err := closeFn(); err != nil {
		t.Fatalf("closeFn() error = %v", err)
	}
	closeFn = func() error { return nil }

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v log=%s", err, data)
	}
	for _, key := range []string{"msg", "detail"} {
		value, ok := record[key].(string)
		if !ok {
			t.Fatalf("log[%q] = %#v, want string", key, record[key])
		}
		if strings.Contains(value, secret) || !strings.Contains(value, "[REDACTED]") {
			t.Fatalf("log[%q] = %q, want secret redacted", key, value)
		}
	}
	for key, want := range wantEnvelope {
		if got, ok := record[key].(string); !ok || got != want {
			t.Fatalf("log[%q] = %#v, want intact %q", key, record[key], want)
		}
	}
	payload, ok := record["payload"].(map[string]any)
	if !ok {
		t.Fatalf("log[payload] = %#v, want object", record["payload"])
	}
	content, ok := payload["content"].(string)
	if !ok || strings.Contains(content, secret) || !strings.Contains(content, "[REDACTED]") {
		t.Fatalf("log[payload][content] = %#v, want secret redacted", payload["content"])
	}
	if got, ok := payload["claim_token_hash"].(string); !ok || got != wantEnvelope["claim_token_hash"] {
		t.Fatalf(
			"log[payload][claim_token_hash] = %#v, want intact %q",
			payload["claim_token_hash"],
			wantEnvelope["claim_token_hash"],
		)
	}
}

func TestNewWithFileRotation(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRotateStructuredLogFileWhenSizeCapIsReached", func(t *testing.T) {
		t.Parallel()

		logFile := filepath.Join(t.TempDir(), "logs", "agh.log")
		log, closeFn, err := New(
			WithLevel("info"),
			WithFile(logFile),
			WithFileRotation(FileRotationConfig{MaxSizeMB: 1, MaxBackups: 2, MaxAgeDays: 1}),
			WithMirrorToStderr(false),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		payload := strings.Repeat("x", 2048)
		for i := range 700 {
			log.Info("rotation-check", "index", i, "payload", payload)
		}
		if err := closeFn(); err != nil {
			t.Fatalf("closeFn() error = %v", err)
		}
		entries, err := os.ReadDir(filepath.Dir(logFile))
		if err != nil {
			t.Fatalf("ReadDir() error = %v", err)
		}
		rotated := false
		for _, entry := range entries {
			name := entry.Name()
			if name != filepath.Base(logFile) && strings.HasPrefix(name, "agh-") && strings.HasSuffix(name, ".log") {
				rotated = true
			}
		}
		if !rotated {
			t.Fatalf("rotated log file not found in %v", entries)
		}
	})
}

func TestParseLevelRejectsUnsupportedValue(t *testing.T) {
	t.Parallel()

	if _, err := ParseLevel("trace"); err == nil {
		t.Fatal("ParseLevel() error = nil, want non-nil")
	}
}

func TestParseLevelAcceptsConfiguredValues(t *testing.T) {
	t.Parallel()

	tests := []string{"", "debug", "info", "warn", "error"}
	for _, tt := range tests {
		if _, err := ParseLevel(tt); err != nil {
			t.Fatalf("ParseLevel(%q) error = %v", tt, err)
		}
	}
}

func TestNewWithoutFileStillBuildsLogger(t *testing.T) {
	t.Parallel()

	log, closeFn, err := New(WithMirrorToStderr(false))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if log == nil {
		t.Fatal("New() logger = nil")
	}
	if err := closeFn(); err != nil {
		t.Fatalf("closeFn() error = %v", err)
	}
}

func TestMirrorToStderrHelpers(t *testing.T) {
	t.Parallel()

	t.Run("ShouldDisableMirrorWhenEnvExplicitlyFalse", func(t *testing.T) {
		t.Parallel()

		getenv := func(string) string {
			return "false"
		}
		if MirrorToStderrEnabled(getenv) {
			t.Fatal("MirrorToStderrEnabled(false) = true, want false")
		}
	})

	t.Run("ShouldDefaultMirrorWhenEnvUnset", func(t *testing.T) {
		t.Parallel()

		getenv := func(string) string {
			return ""
		}
		if !MirrorToStderrEnabled(getenv) {
			t.Fatal("MirrorToStderrEnabled(unset) = false, want true")
		}
	})

	t.Run("ShouldInjectDetachedEnvOverride", func(t *testing.T) {
		t.Parallel()

		sandbox := WithMirrorToStderrEnv([]string{"PATH=/usr/bin"}, false)
		joined := strings.Join(sandbox, "\n")
		if !strings.Contains(joined, "AGH_INTERNAL_LOG_MIRROR_STDERR=0") {
			t.Fatalf("WithMirrorToStderrEnv(false) = %q, want AGH_INTERNAL_LOG_MIRROR_STDERR=0", joined)
		}
	})
}
