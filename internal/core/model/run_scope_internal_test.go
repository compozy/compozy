package model

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

type failingEntropyReader struct {
	err error
}

func (r failingEntropyReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestBuildRunIDPreservesExplicitRunIDWithoutEntropy(t *testing.T) {
	const explicitRunID = " explicit-run "
	got, err := buildRunID(&RuntimeConfig{RunID: explicitRunID}, time.Time{}, nil)
	if err != nil {
		t.Fatalf("buildRunID() error = %v", err)
	}
	if got != explicitRunID {
		t.Fatalf("buildRunID() = %q, want %q", got, explicitRunID)
	}
}

func TestBuildRunIDGeneratesEntropyBackedLabeledIDs(t *testing.T) {
	now := time.Date(2026, 5, 13, 20, 4, 5, 123456789, time.UTC)
	entropy := bytes.NewReader(append(
		bytes.Repeat([]byte{0x11}, generatedRunIDEntropyBytes),
		bytes.Repeat([]byte{0x22}, generatedRunIDEntropyBytes)...,
	))
	cfg := &RuntimeConfig{Mode: ExecutionModeExec}

	first, err := buildRunID(cfg, now, entropy)
	if err != nil {
		t.Fatalf("buildRunID(first) error = %v", err)
	}
	second, err := buildRunID(cfg, now, entropy)
	if err != nil {
		t.Fatalf("buildRunID(second) error = %v", err)
	}
	if first == second {
		t.Fatalf("generated run IDs should differ for different entropy: %q", first)
	}
	if !strings.HasPrefix(first, "exec-20260513-200405-123456789-1111111111111111") {
		t.Fatalf("first generated run ID = %q, want exec temporal prefix with entropy", first)
	}
	if !strings.HasPrefix(second, "exec-20260513-200405-123456789-2222222222222222") {
		t.Fatalf("second generated run ID = %q, want exec temporal prefix with entropy", second)
	}
}

func TestBuildRunIDUsesModeSpecificLabels(t *testing.T) {
	now := time.Date(2026, 5, 13, 20, 4, 5, 0, time.UTC)
	testCases := []struct {
		name       string
		cfg        *RuntimeConfig
		wantPrefix string
	}{
		{
			name:       "exec",
			cfg:        &RuntimeConfig{Mode: ExecutionModeExec},
			wantPrefix: "exec-",
		},
		{
			name:       "tasks",
			cfg:        &RuntimeConfig{Mode: ExecutionModePRDTasks, Name: "feature/auth"},
			wantPrefix: "tasks-feature_auth-",
		},
		{
			name:       "reviews",
			cfg:        &RuntimeConfig{Mode: ExecutionModePRReview, Name: "user/auth", Round: 2},
			wantPrefix: "reviews-user_auth-",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildRunID(
				tc.cfg,
				now,
				bytes.NewReader(bytes.Repeat([]byte{0xaa}, generatedRunIDEntropyBytes)),
			)
			if err != nil {
				t.Fatalf("buildRunID() error = %v", err)
			}
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("buildRunID() = %q, want prefix %q", got, tc.wantPrefix)
			}
		})
	}
}

func TestBuildRunIDReturnsEntropyErrors(t *testing.T) {
	entropyErr := errors.New("entropy unavailable")
	_, err := buildRunID(
		&RuntimeConfig{Mode: ExecutionModeExec},
		time.Date(2026, 5, 13, 20, 4, 5, 0, time.UTC),
		failingEntropyReader{err: entropyErr},
	)
	if !errors.Is(err, entropyErr) {
		t.Fatalf("buildRunID() error = %v, want wrapped entropy error", err)
	}
	if !strings.Contains(err.Error(), "build run id: read entropy") {
		t.Fatalf("buildRunID() error = %q, want entropy context", err)
	}
}
