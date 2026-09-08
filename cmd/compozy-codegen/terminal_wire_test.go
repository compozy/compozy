package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalWire(t *testing.T) {
	t.Run("Should generate both terminal wire languages from one manifest", func(t *testing.T) {
		t.Parallel()
		directory := t.TempDir()
		paths := terminalWirePaths{
			goOutput:   filepath.Join(directory, "opcodes_generated.go"),
			tsOutput:   filepath.Join(directory, "terminal-wire.ts"),
			docsOutput: filepath.Join(directory, "terminal-wire.md"),
		}
		if err := writeTerminalWire(t.Context(), paths); err != nil {
			t.Fatalf("writeTerminalWire() error = %v", err)
		}
		if err := checkTerminalWire(t.Context(), paths); err != nil {
			t.Fatalf("checkTerminalWire() error = %v", err)
		}
		goContent, err := os.ReadFile(paths.goOutput)
		if err != nil {
			t.Fatalf("ReadFile(Go terminal wire) error = %v", err)
		}
		tsContent, err := os.ReadFile(paths.tsOutput)
		if err != nil {
			t.Fatalf("ReadFile(TypeScript terminal wire) error = %v", err)
		}
		docsContent, err := os.ReadFile(paths.docsOutput)
		if err != nil {
			t.Fatalf("ReadFile(terminal wire docs) error = %v", err)
		}
		for _, contract := range [][]byte{goContent, tsContent} {
			if !bytes.Contains(contract, []byte("compozy.terminal.v3")) ||
				!bytes.Contains(contract, []byte("RedactedInput")) &&
					!bytes.Contains(contract, []byte("redactedInput")) {
				t.Fatalf("generated terminal wire contract is incomplete: %s", contract)
			}
		}
		if !bytes.Contains(docsContent, []byte("Earlier versions are rejected")) ||
			bytes.Contains(docsContent, []byte("`RELEASE`")) ||
			bytes.Contains(docsContent, []byte("write lease")) {
			t.Fatalf("generated terminal wire docs describe an obsolete contract: %s", docsContent)
		}
		if err := os.WriteFile(paths.tsOutput, []byte("export const stale = true;\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(stale terminal wire) error = %v", err)
		}
		if err := checkTerminalWire(t.Context(), paths); !errors.Is(err, ErrStaleGeneratedFile) {
			t.Fatalf("checkTerminalWire(stale) error = %v, want ErrStaleGeneratedFile", err)
		}
	})

	t.Run("Should require a version bump for incompatible terminal wire edits", func(t *testing.T) {
		t.Parallel()

		manifestPath := filepath.Join("..", "..", defaultTerminalWireManifestPath)
		manifest, err := readTerminalWireManifest(manifestPath)
		if err != nil {
			t.Fatalf("readTerminalWireManifest() error = %v", err)
		}
		mutations := map[string]func(*terminalWireManifest){
			"rename": func(candidate *terminalWireManifest) {
				candidate.Server[0].GoName = "ServerOpRenamed"
			},
			"remove": func(candidate *terminalWireManifest) {
				candidate.Server = candidate.Server[:len(candidate.Server)-1]
			},
			"add": func(candidate *terminalWireManifest) {
				candidate.Server = append(candidate.Server, terminalWireOpcode{
					GoName: "ServerOpAdded", TSName: "added", Value: byte(len(candidate.Server) + 1),
				})
			},
		}
		for name, mutate := range mutations {
			t.Run("Should reject terminal wire "+name+" without a version bump", func(t *testing.T) {
				t.Parallel()

				encoded, marshalErr := json.Marshal(manifest)
				if marshalErr != nil {
					t.Fatalf("json.Marshal(manifest) error = %v", marshalErr)
				}
				var candidate terminalWireManifest
				if unmarshalErr := json.Unmarshal(encoded, &candidate); unmarshalErr != nil {
					t.Fatalf("json.Unmarshal(manifest) error = %v", unmarshalErr)
				}
				mutate(&candidate)
				assertErrorContains(
					t,
					validateTerminalWireManifest(candidate),
					"bump the subprotocol version",
				)
			})
		}
	})

	t.Run("Should accept every decimal terminal wire version at least two", func(t *testing.T) {
		t.Parallel()

		manifestPath := filepath.Join("..", "..", defaultTerminalWireManifestPath)
		manifest, err := readTerminalWireManifest(manifestPath)
		if err != nil {
			t.Fatalf("readTerminalWireManifest() error = %v", err)
		}
		for _, version := range []string{"v10", "v19", "v999999999999999999999999999999999999"} {
			t.Run("Should accept "+version, func(t *testing.T) {
				t.Parallel()

				candidate := manifest
				candidate.Subprotocol = "compozy.terminal." + version
				if validationErr := validateTerminalWireManifest(candidate); validationErr != nil {
					t.Fatalf("validateTerminalWireManifest(%s) error = %v", version, validationErr)
				}
			})
		}
	})

	t.Run("Should restore every terminal wire artifact after publication fails", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		paths := terminalWirePaths{
			manifest:   filepath.Join("..", "..", defaultTerminalWireManifestPath),
			goOutput:   filepath.Join(directory, "opcodes_generated.go"),
			tsOutput:   filepath.Join(directory, "terminal-wire.ts"),
			docsOutput: filepath.Join(directory, "terminal-wire.md"),
		}
		originals := map[string][]byte{
			paths.goOutput:   []byte("original go\n"),
			paths.tsOutput:   []byte("original ts\n"),
			paths.docsOutput: []byte("original docs\n"),
		}
		for path, content := range originals {
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v", path, err)
			}
		}
		publishErr := errors.New("injected terminal wire publication failure")
		artifacts := []generatedArtifact{
			{path: paths.goOutput, content: []byte("next go\n")},
			{path: paths.tsOutput, content: []byte("next ts\n")},
			{path: paths.docsOutput, content: []byte("next docs\n")},
		}
		err := publishGeneratedArtifactSet(artifacts, func(path string, content []byte) error {
			if path == paths.tsOutput {
				return publishErr
			}
			return publishGeneratedFile(path, content)
		})
		if !errors.Is(err, publishErr) {
			t.Fatalf("publishGeneratedArtifactSet() error = %v, want %v", err, publishErr)
		}
		for path, want := range originals {
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", path, readErr)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("artifact %q after rollback = %q, want %q", path, got, want)
			}
		}
	})
}

func assertErrorContains(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want text %q", fragment)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("error = %q, want text %q", err.Error(), fragment)
	}
}
