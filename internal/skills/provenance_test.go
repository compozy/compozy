package skills

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestComputeHashReturnsConsistentSHA256ForSameContent(t *testing.T) {
	t.Parallel()

	content := []byte(strings.Join([]string{
		"---",
		"name: marketplace-skill",
		"description: Example skill",
		"---",
		"Review carefully.",
	}, "\n"))

	first := ComputeHash(content)
	second := ComputeHash(content)

	if first != second {
		t.Fatalf("ComputeHash() first = %q, second = %q, want identical hashes", first, second)
	}
}

func TestComputeHashReturnsDifferentHashForDifferentContent(t *testing.T) {
	t.Parallel()

	first := ComputeHash([]byte("first content"))
	second := ComputeHash([]byte("second content"))

	if first == second {
		t.Fatalf("ComputeHash() hashes match for different content: %q", first)
	}
}

func TestComputeDirectoryHashReturnsDifferentHashWhenAuxiliaryFileChanges(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	writeSkillFile(t, skillDir, skillFileName, strings.Join([]string{
		"---",
		"name: directory-hash",
		"description: Example skill",
		"---",
		"body",
	}, "\n"))
	helperPath := filepath.Join(skillDir, "helper.sh")
	if err := os.WriteFile(helperPath, []byte("#!/bin/sh\nprintf 'first'\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", helperPath, err)
	}

	first, err := ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash() error = %v", err)
	}

	if err := os.WriteFile(helperPath, []byte("#!/bin/sh\nprintf 'second'\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", helperPath, err)
	}

	second, err := ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash() second error = %v", err)
	}

	if first == second {
		t.Fatalf("ComputeDirectoryHash() hashes match after helper mutation: %q", first)
	}
}

func TestComputeDirectoryHashRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectSymlinkEscape", func(t *testing.T) {
		t.Parallel()

		skillDir := t.TempDir()
		writeSkillFile(t, skillDir, skillFileName, strings.Join([]string{
			"---",
			"name: directory-hash",
			"description: Example skill",
			"---",
			"body",
		}, "\n"))
		outside := filepath.Join(t.TempDir(), "secret.txt")
		if err := os.WriteFile(outside, []byte("secret\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(outside) error = %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(skillDir, "secret.txt")); err != nil {
			t.Skipf("os.Symlink(secret) unavailable: %v", err)
		}

		_, err := ComputeDirectoryHash(skillDir)
		if !errors.Is(err, ErrSymlinkEscape) {
			t.Fatalf("ComputeDirectoryHash() error = %v, want ErrSymlinkEscape", err)
		}
	})
}

func TestWriteSidecarCreatesStableHumanReadableJSON(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	provenance := testProvenance()

	if err := WriteSidecar(skillDir, provenance); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	sidecarBytes, err := os.ReadFile(filepath.Join(skillDir, sidecarFileName))
	if err != nil {
		t.Fatalf("ReadFile(sidecar) error = %v", err)
	}

	want := strings.Join([]string{
		"{",
		`  "hash": "abc123",`,
		`  "registry": "clawhub",`,
		`  "slug": "@author/marketplace-skill",`,
		`  "version": "1.2.3",`,
		`  "installed_at": "2026-04-07T12:00:00Z"`,
		"}",
		"",
	}, "\n")

	if string(sidecarBytes) != want {
		t.Fatalf("sidecar JSON mismatch\nwant:\n%s\ngot:\n%s", want, string(sidecarBytes))
	}
}

func TestReadSidecarParsesValidJSONIntoProvenance(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	want := testProvenance()
	if err := WriteSidecar(skillDir, want); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	got, err := ReadSidecar(skillDir)
	if err != nil {
		t.Fatalf("ReadSidecar() error = %v", err)
	}

	if got == nil {
		t.Fatal("ReadSidecar() = nil, want provenance")
	}
	if *got != want {
		t.Fatalf("ReadSidecar() = %#v, want %#v", *got, want)
	}
}

func TestReadSidecarReturnsDescriptiveErrorForMalformedJSON(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	sidecarPath := filepath.Join(skillDir, sidecarFileName)
	if err := os.WriteFile(sidecarPath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", sidecarPath, err)
	}

	_, err := ReadSidecar(skillDir)
	if err == nil {
		t.Fatal("ReadSidecar() error = nil, want malformed JSON error")
	}
	if !strings.Contains(err.Error(), "parse provenance sidecar") {
		t.Fatalf("ReadSidecar() error = %v, want parse provenance sidecar context", err)
	}
}

func TestReadSidecarRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		field     string
		mutate    func(*Provenance)
		wantField string
	}{
		{
			name:      "ShouldRejectMissingHash",
			field:     "hash",
			wantField: "missing hash",
			mutate: func(p *Provenance) {
				p.Hash = ""
			},
		},
		{
			name:      "ShouldRejectMissingRegistry",
			field:     "registry",
			wantField: "missing registry",
			mutate: func(p *Provenance) {
				p.Registry = ""
			},
		},
		{
			name:      "ShouldRejectMissingSlug",
			field:     "slug",
			wantField: "missing slug",
			mutate: func(p *Provenance) {
				p.Slug = ""
			},
		},
		{
			name:      "ShouldRejectMissingVersion",
			field:     "version",
			wantField: "missing version",
			mutate: func(p *Provenance) {
				p.Version = ""
			},
		},
		{
			name:      "ShouldRejectMissingInstalledAt",
			field:     "installed_at",
			wantField: "missing installed_at",
			mutate: func(p *Provenance) {
				p.InstalledAt = time.Time{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			skillDir := t.TempDir()
			provenance := testProvenance()
			tt.mutate(&provenance)

			payload, err := json.Marshal(provenance)
			if err != nil {
				t.Fatalf("json.Marshal(provenance) error = %v", err)
			}
			sidecarPath := filepath.Join(skillDir, sidecarFileName)
			if err := os.WriteFile(sidecarPath, append(payload, '\n'), 0o644); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v", sidecarPath, err)
			}

			_, err = ReadSidecar(skillDir)
			if err == nil {
				t.Fatalf("ReadSidecar() error = nil, want %s validation failure", tt.field)
			}
			if !strings.Contains(err.Error(), tt.wantField) {
				t.Fatalf("ReadSidecar() error = %v, want %q", err, tt.wantField)
			}
		})
	}
}

func TestReadSidecarReturnsNotExistForMissingSidecar(t *testing.T) {
	t.Parallel()

	_, err := ReadSidecar(t.TempDir())
	if err == nil {
		t.Fatal("ReadSidecar() error = nil, want fs.ErrNotExist")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadSidecar() error = %v, want fs.ErrNotExist", err)
	}
}

func TestVerifyHashReturnsNilWhenHashMatches(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	content := strings.Join([]string{
		"---",
		"name: verified-skill",
		"description: Verified skill",
		"---",
		"Everything is intact.",
	}, "\n")
	writeSkillFile(t, skillDir, skillFileName, content)
	hash, err := ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash() error = %v", err)
	}

	err = VerifyHash(skillDir, &Provenance{
		Hash: hash,
	})
	if err != nil {
		t.Fatalf("VerifyHash() error = %v, want nil", err)
	}
}

func TestVerifyHashReturnsExpectedAndActualHashWhenTampered(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	original := strings.Join([]string{
		"---",
		"name: tampered-skill",
		"description: Original content",
		"---",
		"Original body.",
	}, "\n")
	tampered := strings.Join([]string{
		"---",
		"name: tampered-skill",
		"description: Tampered content",
		"---",
		"Tampered body.",
	}, "\n")
	writeSkillFile(t, skillDir, skillFileName, original)
	originalHash, err := ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash() error = %v", err)
	}
	writeSkillFile(t, skillDir, skillFileName, tampered)
	actualHash, err := ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash() after tamper error = %v", err)
	}

	err = VerifyHash(skillDir, &Provenance{
		Hash: originalHash,
	})
	if err == nil {
		t.Fatal("VerifyHash() error = nil, want hash mismatch")
	}

	var mismatch *HashMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("VerifyHash() error = %v, want HashMismatchError", err)
	}
	if mismatch.ExpectedHash != originalHash {
		t.Fatalf("HashMismatchError.ExpectedHash = %q, want %q", mismatch.ExpectedHash, originalHash)
	}
	if mismatch.ActualHash != actualHash {
		t.Fatalf("HashMismatchError.ActualHash = %q, want %q", mismatch.ActualHash, actualHash)
	}
	if !strings.Contains(err.Error(), mismatch.ExpectedHash) || !strings.Contains(err.Error(), mismatch.ActualHash) {
		t.Fatalf("VerifyHash() error = %q, want expected and actual hashes in message", err.Error())
	}
}

func TestVerifyHashDetectsTamperingOutsideSkillMarkdown(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	content := strings.Join([]string{
		"---",
		"name: helper-sensitive",
		"description: Original content",
		"---",
		"Original body.",
	}, "\n")
	writeSkillFile(t, skillDir, skillFileName, content)
	helperPath := filepath.Join(skillDir, "helper.sh")
	if err := os.WriteFile(helperPath, []byte("#!/bin/sh\nprintf 'original'\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", helperPath, err)
	}

	hash, err := ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash() error = %v", err)
	}

	if err := os.WriteFile(helperPath, []byte("#!/bin/sh\nprintf 'tampered'\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", helperPath, err)
	}

	err = VerifyHash(skillDir, &Provenance{Hash: hash})
	if err == nil {
		t.Fatal("VerifyHash() error = nil, want hash mismatch after helper tamper")
	}

	var mismatch *HashMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("VerifyHash() error = %v, want HashMismatchError", err)
	}
	if mismatch.ExpectedHash != hash {
		t.Fatalf("HashMismatchError.ExpectedHash = %q, want %q", mismatch.ExpectedHash, hash)
	}
}

func TestHasSidecarReturnsTrueWhenSidecarExists(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	if err := WriteSidecar(skillDir, testProvenance()); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	hasSidecar, err := HasSidecar(skillDir)
	if err != nil {
		t.Fatalf("HasSidecar() error = %v", err)
	}
	if !hasSidecar {
		t.Fatal("HasSidecar() = false, want true")
	}
}

func TestHasSidecarReturnsFalseWhenSidecarMissing(t *testing.T) {
	t.Parallel()

	hasSidecar, err := HasSidecar(t.TempDir())
	if err != nil {
		t.Fatalf("HasSidecar() error = %v", err)
	}
	if hasSidecar {
		t.Fatal("HasSidecar() = true, want false")
	}
}

func TestSidecarRoundTripProducesIdenticalProvenance(t *testing.T) {
	t.Parallel()

	skillDir := t.TempDir()
	want := testProvenance()

	if err := WriteSidecar(skillDir, want); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	got, err := ReadSidecar(skillDir)
	if err != nil {
		t.Fatalf("ReadSidecar() error = %v", err)
	}

	marshaledGot, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(got) error = %v", err)
	}
	marshaledWant, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal(want) error = %v", err)
	}

	if !bytes.Equal(marshaledGot, marshaledWant) {
		t.Fatalf("round-trip mismatch\ngot:  %s\nwant: %s", marshaledGot, marshaledWant)
	}
}

func testProvenance() Provenance {
	return Provenance{
		Hash:        "abc123",
		Registry:    "clawhub",
		Slug:        "@author/marketplace-skill",
		Version:     "1.2.3",
		InstalledAt: time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC),
	}
}
