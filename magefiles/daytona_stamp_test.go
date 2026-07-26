//go:build mage

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoListDepLine(t *testing.T) {
	t.Parallel()

	t.Run("Should parse dir, module path, go files, and embeds", func(t *testing.T) {
		t.Parallel()
		dir, modPath, files, ok := parseGoListDepLine(
			"/repo/internal/pkg|github.com/compozy/agh|a.go,b.go|assets/data.json",
		)
		if !ok {
			t.Fatal("parseGoListDepLine() ok = false, want true")
		}
		if dir != "/repo/internal/pkg" || modPath != "github.com/compozy/agh" {
			t.Fatalf("parseGoListDepLine() dir=%q mod=%q", dir, modPath)
		}
		want := []string{"a.go", "b.go", "assets/data.json"}
		if len(files) != len(want) {
			t.Fatalf("parseGoListDepLine() files = %v, want %v", files, want)
		}
		for i := range want {
			if files[i] != want[i] {
				t.Fatalf("parseGoListDepLine() files[%d] = %q, want %q", i, files[i], want[i])
			}
		}
	})

	t.Run("Should reject blank and malformed lines", func(t *testing.T) {
		t.Parallel()
		for _, line := range []string{"", "   ", "only|three|fields", "|github.com/compozy/agh|a.go|"} {
			if _, _, _, ok := parseGoListDepLine(line); ok {
				t.Fatalf("parseGoListDepLine(%q) ok = true, want false", line)
			}
		}
	})
}

func TestHashFilesInto(t *testing.T) {
	t.Parallel()

	digestOf := func(t *testing.T, paths []string) string {
		t.Helper()
		hash := sha256.New()
		if err := hashFilesInto(hash, paths); err != nil {
			t.Fatalf("hashFilesInto() error = %v", err)
		}
		return hex.EncodeToString(hash.Sum(nil))
	}

	t.Run("Should be deterministic and content-sensitive", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "input.go")
		if err := os.WriteFile(file, []byte("package a\n"), 0o644); err != nil {
			t.Fatalf("write input: %v", err)
		}
		first := digestOf(t, []string{file})
		second := digestOf(t, []string{file})
		if first != second {
			t.Fatalf("digest not deterministic: %q vs %q", first, second)
		}
		if err := os.WriteFile(file, []byte("package b\n"), 0o644); err != nil {
			t.Fatalf("rewrite input: %v", err)
		}
		if changed := digestOf(t, []string{file}); changed == first {
			t.Fatal("digest unchanged after content change")
		}
	})

	t.Run("Should fail when an input file is missing", func(t *testing.T) {
		t.Parallel()
		hash := sha256.New()
		if err := hashFilesInto(hash, []string{filepath.Join(t.TempDir(), "missing.go")}); err == nil {
			t.Fatal("hashFilesInto() error = nil, want read error")
		}
	})
}
