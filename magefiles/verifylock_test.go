//go:build mage

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifyLockDisabled(t *testing.T) {
	// t.Setenv forbids t.Parallel (L-002); the whole test stays serial.
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "Should keep the lock enabled when the env var is unset", value: "", want: false},
		{name: "Should keep the lock enabled for unrecognized values", value: "on", want: false},
		{name: "Should disable the lock for off", value: "off", want: true},
		{name: "Should disable the lock for 0", value: "0", want: true},
		{name: "Should disable the lock for false", value: "false", want: true},
		{name: "Should disable the lock for mixed-case disabled", value: " Disabled ", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(verifyLockEnvVar, tc.value)
			if got := verifyLockDisabled(); got != tc.want {
				t.Fatalf("verifyLockDisabled() with %q = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestAcquireVerifyLockAt(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize a second acquirer until the first releases", func(t *testing.T) {
		t.Parallel()
		lockPath := filepath.Join(t.TempDir(), verifyLockFileName)
		firstRelease := acquireVerifyLockAt(lockPath)
		secondAcquired := make(chan func(), 1)
		go func() {
			secondAcquired <- acquireVerifyLockAt(lockPath)
		}()
		select {
		case release := <-secondAcquired:
			release()
			firstRelease()
			t.Fatal("second acquirer obtained the verify lock while the first still held it")
		case <-time.After(3 * verifyLockPollInterval):
		}
		firstRelease()
		select {
		case release := <-secondAcquired:
			release()
		case <-time.After(10 * time.Second):
			t.Fatal("second acquirer never obtained the verify lock after release")
		}
	})

	t.Run("Should record holder metadata while held and clear it on release", func(t *testing.T) {
		t.Parallel()
		lockPath := filepath.Join(t.TempDir(), verifyLockFileName)
		release := acquireVerifyLockAt(lockPath)
		data, err := os.ReadFile(lockPath)
		if err != nil {
			release()
			t.Fatalf("read verify lock holder: %v", err)
		}
		var holder verifyLockHolder
		if err := json.Unmarshal(data, &holder); err != nil {
			release()
			t.Fatalf("decode verify lock holder %q: %v", string(data), err)
		}
		if holder.PID != os.Getpid() {
			release()
			t.Fatalf("verify lock holder pid = %d, want %d", holder.PID, os.Getpid())
		}
		if holder.StartedAt == "" || holder.WorkingDir == "" {
			release()
			t.Fatalf("verify lock holder missing metadata: %+v", holder)
		}
		described := describeVerifyLockHolder(lockPath)
		if !strings.Contains(described, "pid ") {
			release()
			t.Fatalf("describeVerifyLockHolder() = %q, want pid description", described)
		}
		release()
		cleared, err := os.ReadFile(lockPath)
		if err != nil {
			t.Fatalf("read verify lock after release: %v", err)
		}
		if len(strings.TrimSpace(string(cleared))) != 0 {
			t.Fatalf("verify lock holder not cleared on release: %q", string(cleared))
		}
	})

	t.Run("Should describe an anonymous holder when the lock file is empty", func(t *testing.T) {
		t.Parallel()
		lockPath := filepath.Join(t.TempDir(), verifyLockFileName)
		if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
			t.Fatalf("seed empty lock file: %v", err)
		}
		if got := describeVerifyLockHolder(lockPath); got != "another make verify run" {
			t.Fatalf("describeVerifyLockHolder() = %q, want anonymous fallback", got)
		}
	})
}
