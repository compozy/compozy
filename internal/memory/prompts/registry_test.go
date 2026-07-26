package prompts

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should load every declared asset version", func(t *testing.T) {
		t.Parallel()

		for _, declared := range allAssetVersions() {
			t.Run(fmt.Sprintf("Should load %s %s", declared.name, declared.version), func(t *testing.T) {
				t.Parallel()

				asset, err := Load(declared.name, declared.version)
				if err != nil {
					t.Fatalf("load %s %s: %v", declared.name, declared.version, err)
				}
				if asset.Name != declared.name {
					t.Fatalf("asset name = %q, want %q", asset.Name, declared.name)
				}
				if asset.Version != declared.version {
					t.Fatalf("asset version = %q, want %q", asset.Version, declared.version)
				}
				if asset.Filename != declared.filename {
					t.Fatalf("asset filename = %q, want %q", asset.Filename, declared.filename)
				}
				if strings.TrimSpace(asset.Content) == "" {
					t.Fatalf("asset %s %s content is empty", declared.name, declared.version)
				}
			})
		}
	})

	t.Run("Should select latest version deterministically", func(t *testing.T) {
		t.Parallel()

		registry := DefaultRegistry()
		for _, name := range allAssetNames() {
			t.Run(fmt.Sprintf("Should select latest %s", name), func(t *testing.T) {
				t.Parallel()

				latestVersion, ok := registry.latest[name]
				if !ok {
					t.Fatalf("latest version for %s is not declared", name)
				}
				versions, ok := registry.assets[name]
				if !ok {
					t.Fatalf("asset %s has no declared versions", name)
				}
				if _, ok := versions[latestVersion]; !ok {
					t.Fatalf("latest version %s for %s is not in declared versions", latestVersion, name)
				}

				explicit, err := registry.Load(name, latestVersion)
				if err != nil {
					t.Fatalf("load explicit %s %s: %v", name, latestVersion, err)
				}
				latest, err := registry.LoadLatest(name)
				if err != nil {
					t.Fatalf("load latest %s: %v", name, err)
				}
				if latest != explicit {
					t.Fatalf("latest asset = %#v, want %#v", latest, explicit)
				}
			})
		}
	})

	t.Run("Should render template assets with representative data", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name      Name
			data      any
			fragments []string
		}{
			{
				name: NameCheckpointSummary,
				data: map[string]any{
					"WorkspaceID":     "workspace-1",
					"SessionID":       "session-2",
					"EndedAt":         "2026-07-15T12:00:00Z",
					"PreviousSummary": "Prior checkpoint fact.",
					"Transcript":      "New checkpoint fact.",
				},
				fragments: []string{
					"Prior checkpoint fact.",
					"New checkpoint fact.",
					"## Historical Task Snapshot",
				},
			},
			{
				name: NameDecide,
				data: map[string]any{
					"Candidate": map[string]any{
						"Entity":    "AGH",
						"Attribute": "memory",
						"Scope":     "workspace",
						"AgentName": "codex",
						"AgentTier": "workspace",
						"Content":   "Prefer durable memory facts.",
					},
					"RuleTrace": []map[string]any{
						{
							"Name":   "duplicate_check",
							"Passed": true,
							"Reason": "unique candidate",
							"Target": "target-1",
						},
					},
					"Targets": []map[string]any{
						{
							"ID":             "target-1",
							"TargetFilename": "memory.md",
							"Entity":         "AGH",
							"Attribute":      "memory",
							"Content":        "Existing memory fact.",
							"LastUpdatedAt":  "2026-05-17T00:00:00Z",
						},
					},
				},
				fragments: []string{
					"candidate.frontmatter.entity",
					"AGH",
					"duplicate_check",
					"target.target_filename",
				},
			},
			{
				name: NameDream,
				data: map[string]any{
					"RunID":       "dream-run-1",
					"Scope":       "workspace",
					"WorkspaceID": "workspace-1",
					"Candidates": []map[string]any{
						{
							"ChunkID":     "chunk-1",
							"Type":        "project",
							"Scope":       "workspace",
							"Score":       0.91,
							"RecallCount": 3,
							"Content":     "Agents need deterministic memory synthesis.",
						},
					},
				},
				fragments: []string{
					"dream-run-1",
					"chunk-1",
					"deterministic DLQ replay",
				},
			},
			{
				name: NameExtract,
				data: map[string]any{
					"WhatNotToSave": "Do not save secrets.",
					"Turn": map[string]any{
						"SessionID":       "session-1",
						"RootSessionID":   "root-1",
						"ParentSessionID": "parent-1",
						"AgentID":         "agent-1",
						"WorkspaceID":     "workspace-1",
						"SinceMessageSeq": 1,
						"UntilMessageSeq": 7,
					},
					"Transcript": "User described a durable project preference.",
				},
				fragments: []string{
					"Do not save secrets.",
					"session-1",
					"User described a durable project preference.",
				},
			},
		}

		for _, tc := range cases {
			t.Run(fmt.Sprintf("Should render %s v1", tc.name), func(t *testing.T) {
				t.Parallel()

				parsed, err := ParseTemplate(tc.name, VersionV1)
				if err != nil {
					t.Fatalf("parse %s: %v", tc.name, err)
				}
				var rendered bytes.Buffer
				if err := parsed.Execute(&rendered, tc.data); err != nil {
					t.Fatalf("execute %s: %v", tc.name, err)
				}
				output := rendered.String()
				for _, fragment := range tc.fragments {
					if !strings.Contains(output, fragment) {
						t.Fatalf("rendered %s missing fragment %q: %q", tc.name, fragment, output)
					}
				}
			})
		}
	})

	t.Run("Should parse template assets with missing keys rejected", func(t *testing.T) {
		t.Parallel()

		for _, declared := range allAssetVersions() {
			if !strings.HasSuffix(declared.filename, ".tmpl") {
				continue
			}
			t.Run(
				fmt.Sprintf("Should reject missing keys for %s %s", declared.name, declared.version),
				func(t *testing.T) {
					t.Parallel()

					parsed, err := ParseTemplate(declared.name, declared.version)
					if err != nil {
						t.Fatalf("parse %s %s: %v", declared.name, declared.version, err)
					}
					var rendered bytes.Buffer
					err = parsed.Execute(&rendered, map[string]any{})
					if err == nil {
						t.Fatalf("execute %s %s with missing keys succeeded", declared.name, declared.version)
					}
				},
			)
		}
	})

	t.Run("Should load latest for each declared prompt from the manifest", func(t *testing.T) {
		t.Parallel()

		registry := DefaultRegistry()
		for _, name := range allAssetNames() {
			t.Run(fmt.Sprintf("Should load latest %s from manifest", name), func(t *testing.T) {
				t.Parallel()

				latest, err := registry.LoadLatest(name)
				if err != nil {
					t.Fatalf("load latest %s: %v", name, err)
				}
				if latest.Version != registry.latest[name] {
					t.Fatalf("latest version = %q, want %q", latest.Version, registry.latest[name])
				}
				if _, ok := registry.assets[name][latest.Version]; !ok {
					t.Fatalf("latest version %s for %s is not declared", latest.Version, name)
				}
			})
		}
	})

	t.Run("Should fail clearly for unknown asset names and versions", func(t *testing.T) {
		t.Parallel()

		_, err := Load(Name("missing"), VersionV1)
		if !errors.Is(err, ErrAssetNotFound) {
			t.Fatalf("missing asset error = %v, want ErrAssetNotFound", err)
		}
		_, err = Load(NameDecide, "v2")
		if !errors.Is(err, ErrVersionNotFound) {
			t.Fatalf("missing version error = %v, want ErrVersionNotFound", err)
		}
	})

	t.Run("Should fail clearly for invalid templates and missing files", func(t *testing.T) {
		t.Parallel()

		invalidRegistry := NewRegistry(fstest.MapFS{
			"decide.v1.tmpl": {Data: []byte("{{")},
		})
		_, err := invalidRegistry.ParseTemplate(NameDecide, VersionV1)
		if err == nil {
			t.Fatalf("invalid template parse succeeded")
		}
		if !strings.Contains(err.Error(), "parse decide v1") {
			t.Fatalf("invalid template error = %q, want parse context", err.Error())
		}

		missingFileRegistry := NewRegistry(fstest.MapFS{})
		_, err = missingFileRegistry.Load(NameDecide, VersionV1)
		if err == nil {
			t.Fatalf("missing file load succeeded")
		}
		if !strings.Contains(err.Error(), "read decide v1") {
			t.Fatalf("missing file error = %q, want read context", err.Error())
		}
	})

	t.Run("Should expose controller extractor dreaming and policy fields", func(t *testing.T) {
		t.Parallel()

		assertAssetContains(t, NameDecide,
			"candidate.frontmatter.entity",
			"target.target_filename",
			"Rule trace",
		)
		assertAssetContains(t, NameExtract,
			"WHAT_NOT_TO_SAVE policy",
			"session_id",
			"Transcript snapshot",
		)
		assertAssetContains(t, NameDream,
			"_system/dreaming/",
			"deterministic DLQ replay",
		)
		assertAssetContains(t, NameWhatNotToSave,
			"Raw transcript dumps",
			"Secrets, credentials, tokens",
			"Anything already documented in AGENTS.md",
		)
	})
}

type declaredAssetVersion struct {
	name     Name
	version  string
	filename string
}

func allAssetVersions() []declaredAssetVersion {
	assetIndex := defaultAssetIndex()
	versions := make([]declaredAssetVersion, 0, len(assetIndex))
	for name, versionIndex := range assetIndex {
		for version, filename := range versionIndex {
			versions = append(versions, declaredAssetVersion{
				name:     name,
				version:  version,
				filename: filename,
			})
		}
	}
	slices.SortFunc(versions, func(a declaredAssetVersion, b declaredAssetVersion) int {
		if a.name != b.name {
			return cmp.Compare(a.name, b.name)
		}
		return cmp.Compare(a.version, b.version)
	})
	return versions
}

func allAssetNames() []Name {
	assetIndex := defaultAssetIndex()
	names := make([]Name, 0, len(assetIndex))
	for name := range assetIndex {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func assertAssetContains(t *testing.T, name Name, fragments ...string) {
	t.Helper()

	asset, err := Load(name, VersionV1)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	for _, fragment := range fragments {
		if !strings.Contains(asset.Content, fragment) {
			t.Fatalf("asset %s missing fragment %q", name, fragment)
		}
	}
}
