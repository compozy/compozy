package desktoprelease

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleasePolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse the reserved stable channel", func(t *testing.T) {
		t.Parallel()

		assertErrorContains(t, ValidateDesktopChannel(ChannelStable), "stable channel is reserved")
	})

	t.Run("Should require a candidate strictly greater than the live feed", func(t *testing.T) {
		t.Parallel()

		if err := AssertStrictlyGreater("0.4.0-beta.10", "0.4.0-beta.9"); err != nil {
			t.Fatalf("AssertStrictlyGreater(valid) error = %v", err)
		}
		for _, test := range []struct {
			name      string
			candidate string
		}{
			{name: "Should reject an equal candidate", candidate: "0.4.0-beta.9"},
			{name: "Should reject an older candidate", candidate: "0.4.0-beta.8"},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				assertErrorContains(
					t,
					AssertStrictlyGreater(test.candidate, "0.4.0-beta.9"),
					"must be strictly greater",
				)
			})
		}
	})

	t.Run("Should reject updater comparator overrides", func(t *testing.T) {
		t.Parallel()

		assertErrorContains(
			t,
			AssertDefaultComparator("builder.version_comparator(compare)"),
			"custom updater comparator \"version_comparator\" is forbidden",
		)
		if err := AssertDefaultComparator("updater_builder().timeout(duration)"); err != nil {
			t.Fatalf("AssertDefaultComparator(default) error = %v", err)
		}
	})
}

func TestManifestValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject latest manifest missing any required platform", func(t *testing.T) {
		t.Parallel()

		manifest := validLatestManifest()
		delete(manifest.Platforms, platformWindowsX8664)
		manifest.Platforms["unsupported"] = UpdaterPlatform{}
		assertErrorContains(t, ValidateLatestManifest(manifest), "required platform windows-x86_64 is missing")
	})

	t.Run("Should reject signatures expressed as URLs", func(t *testing.T) {
		t.Parallel()

		manifest := validLatestManifest()
		entry := manifest.Platforms[platformLinuxX8664]
		entry.Signature = "https://releases.compozy.com/signature.sig"
		manifest.Platforms[platformLinuxX8664] = entry
		assertErrorContains(t, ValidateLatestManifest(manifest), "signature must be non-empty content, not a URL")
	})

	t.Run("Should reject runtime manifest without digest schema heads or SemVer", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name     string
			mutate   func(*RuntimeManifest)
			expected string
		}{
			{name: "Should reject a missing digest", expected: "runtime sha256 is malformed", mutate: func(manifest *RuntimeManifest) {
				entry := manifest.Platforms[platformDarwinAArch64]
				entry.SHA256 = ""
				manifest.Platforms[platformDarwinAArch64] = entry
			}},
			{name: "Should reject a missing schema head", expected: "runtime schema_heads.memory is required", mutate: func(manifest *RuntimeManifest) {
				delete(manifest.SchemaHeads, schemaStreamMemory)
			}},
			{name: "Should reject an invalid version", expected: "must be strict unprefixed SemVer", mutate: func(manifest *RuntimeManifest) {
				manifest.Version = "not-semver"
			}},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				manifest := validRuntimeManifest()
				test.mutate(&manifest)
				assertErrorContains(t, ValidateRuntimeManifest(manifest), test.expected)
			})
		}
	})
}

func TestBuildFeeds(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a distribution base that is not an origin", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		desktopDir := filepath.Join(root, "desktop-artifacts")
		runtimeDir := filepath.Join(root, "runtime-artifacts")
		writeDesktopArtifacts(t, desktopDir, "0.4.0-beta.2")
		writeRuntimeArtifacts(t, runtimeDir)
		writeMigrationStreams(t, root)

		for _, test := range []struct {
			name     string
			baseURL  string
			expected string
		}{
			{name: "Should reject a path", baseURL: DistributionOrigin + "/desktop", expected: "origin must not include a path"},
			{name: "Should reject a query", baseURL: DistributionOrigin + "?channel=beta", expected: "distribution URL must use"},
			{name: "Should reject a fragment", baseURL: DistributionOrigin + "#beta", expected: "distribution URL must use"},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				err := BuildFeeds(t.Context(), BuildRequest{
					Version:     "0.4.0-beta.2",
					PublishedAt: time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC),
					DesktopDir:  desktopDir,
					RuntimeDir:  runtimeDir,
					RepoRoot:    root,
					OutputDir:   filepath.Join(root, "feed-"+strings.ReplaceAll(test.name, " ", "-")),
					BaseURL:     test.baseURL,
				})
				assertErrorContains(t, err, test.expected)
			})
		}
	})

	t.Run("Should generate canonical feeds with exact artifacts and schema heads", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		desktopDir := filepath.Join(root, "desktop-artifacts")
		runtimeDir := filepath.Join(root, "runtime-artifacts")
		outputDir := filepath.Join(root, "feed")
		writeDesktopArtifacts(t, desktopDir, "0.4.0-beta.2")
		writeRuntimeArtifacts(t, runtimeDir)
		writeMigrationStreams(t, root)

		err := BuildFeeds(t.Context(), BuildRequest{
			Version:     "0.4.0-beta.2",
			PublishedAt: time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC),
			DesktopDir:  desktopDir,
			RuntimeDir:  runtimeDir,
			RepoRoot:    root,
			OutputDir:   outputDir,
			BaseURL:     DistributionOrigin,
		})
		if err != nil {
			t.Fatalf("BuildFeeds() error = %v", err)
		}

		latestBytes := readTestFile(t, filepath.Join(outputDir, "latest.json"))
		if strings.HasSuffix(string(latestBytes), "\n") {
			t.Fatal("latest.json has a trailing newline, want canonical JSON bytes")
		}
		var latest LatestManifest
		if err := json.Unmarshal(latestBytes, &latest); err != nil {
			t.Fatalf("json.Unmarshal(latest.json) error = %v", err)
		}
		if err := ValidateLatestManifest(latest); err != nil {
			t.Fatalf("ValidateLatestManifest(generated) error = %v", err)
		}
		if got, want := latest.Platforms[platformDarwinAArch64].URL, latest.Platforms[platformDarwinX8664].URL; got != want {
			t.Fatalf("universal Darwin URLs differ: arm64 = %q, x86_64 = %q", got, want)
		}

		var runtime RuntimeManifest
		if err := json.Unmarshal(readTestFile(t, filepath.Join(outputDir, "runtime.json")), &runtime); err != nil {
			t.Fatalf("json.Unmarshal(runtime.json) error = %v", err)
		}
		if err := ValidateRuntimeManifest(runtime); err != nil {
			t.Fatalf("ValidateRuntimeManifest(generated) error = %v", err)
		}
		wantHeads := map[string]uint64{
			schemaStreamGlobal:    57,
			schemaStreamMemory:    1,
			schemaStreamSession:   6,
			schemaStreamWorkspace: 1,
		}
		for stream, want := range wantHeads {
			if got := runtime.SchemaHeads[stream]; got != want {
				t.Fatalf("runtime schema_heads.%s = %d, want %d", stream, got, want)
			}
		}
	})

	t.Run("Should reject a desktop artifact missing its signature", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeDesktopArtifacts(t, dir, "0.4.0-beta.2")
		if err := os.Remove(filepath.Join(dir, "CompozyOS_0.4.0-beta.2_amd64.AppImage.sig")); err != nil {
			t.Fatalf("os.Remove(signature) error = %v", err)
		}
		assertErrorContains(
			t,
			AssertExactDesktopInventory(t.Context(), dir, "0.4.0-beta.2"),
			"CompozyOS_0.4.0-beta.2_amd64.AppImage.sig",
		)
	})

	t.Run("Should reject an unexpected runtime artifact", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeRuntimeArtifacts(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte("unexpected"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(extra runtime artifact) error = %v", err)
		}
		assertErrorContains(t, AssertRuntimeInventory(t.Context(), dir), "checksums.txt")
	})
}

func TestChannelConfig(t *testing.T) {
	t.Parallel()
	const minisignPublicKey = "untrusted comment: minisign public key E7620F1842B4E81F\n" +
		"RWQf6LRCGA9i53mlYecO4IzT51TGPpvWucNSCh1CBM0QTaLn73Y7GFO3\n"

	t.Run("Should embed beta endpoint public key and object-form Windows signer", func(t *testing.T) {
		t.Parallel()

		publicKey := base64.StdEncoding.EncodeToString([]byte(minisignPublicKey))
		output := filepath.Join(t.TempDir(), "tauri.channel.conf.json")
		err := WriteChannelConfig(ChannelConfigRequest{
			Version:       "0.4.0-beta.2",
			Channel:       ChannelBeta,
			PublicKey:     publicKey,
			AzureEndpoint: "https://eus.codesigning.azure.net/",
			OutputPath:    output,
		})
		if err != nil {
			t.Fatalf("WriteChannelConfig() error = %v", err)
		}
		contents := string(readTestFile(t, output))
		for _, want := range []string{
			`"cmd":"artifact-signing-cli"`,
			`"installMode":"passive"`,
			`https://releases.compozy.com/desktop/beta/latest.json`,
			publicKey,
		} {
			if !strings.Contains(contents, want) {
				t.Fatalf("channel config missing %q: %s", want, contents)
			}
		}
	})

	t.Run("Should reject updater public keys without a valid Ed25519 Minisign packet", func(t *testing.T) {
		t.Parallel()

		for _, decoded := range []string{
			"untrusted comment: minisign public key\nRWQexample\n",
			"untrusted comment: not a minisign public key\nRWQf6LRCGA9i53mlYecO4IzT51TGPpvWucNSCh1CBM0QTaLn73Y7GFO3\n",
			"untrusted comment: minisign public key E7620F1842B4E81F\nWXgf6LRCGA9i53mlYecO4IzT51TGPpvWucNSCh1CBM0QTaLn73Y7GFO3\n",
		} {
			err := WriteChannelConfig(ChannelConfigRequest{
				Version:    "0.4.0-beta.2",
				Channel:    ChannelBeta,
				PublicKey:  base64.StdEncoding.EncodeToString([]byte(decoded)),
				OutputPath: filepath.Join(t.TempDir(), "tauri.channel.conf.json"),
			})
			assertErrorContains(t, err, "valid Ed25519 Minisign public key")
		}
	})
}

func assertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want message containing %q", expected)
	}
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("error = %q, want message containing %q", err, expected)
	}
}

func validLatestManifest() LatestManifest {
	platforms := make(map[string]UpdaterPlatform, len(platformKeys))
	for _, platform := range platformKeys {
		platforms[platform] = UpdaterPlatform{
			Signature: "RWQsignature",
			URL:       DistributionOrigin + "/desktop/v/0.4.0-beta.2/artifact",
		}
	}
	return LatestManifest{
		Version:   "0.4.0-beta.2",
		Notes:     "CompozyOS 0.4.0-beta.2",
		PubDate:   "2026-08-10T12:00:00Z",
		Platforms: platforms,
	}
}

func validRuntimeManifest() RuntimeManifest {
	platforms := make(map[string]RuntimePlatform, len(platformKeys))
	for _, platform := range platformKeys {
		platforms[platform] = RuntimePlatform{
			URL:    DistributionOrigin + "/desktop/v/runtime/0.4.0-beta.2/artifact",
			SHA256: strings.Repeat("a", 64),
			Size:   1,
		}
	}
	return RuntimeManifest{
		SchemaVersion: 1,
		Version:       "0.4.0-beta.2",
		Platforms:     platforms,
		SchemaHeads: map[string]uint64{
			schemaStreamGlobal:    57,
			schemaStreamMemory:    1,
			schemaStreamSession:   6,
			schemaStreamWorkspace: 1,
		},
	}
}

func writeDesktopArtifacts(t *testing.T, dir, version string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(desktop artifacts) error = %v", err)
	}
	for _, name := range DesktopArtifactNames(version) {
		contents := []byte("artifact")
		if strings.HasSuffix(name, ".sig") {
			contents = []byte("RWQsignature")
		}
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0o600); err != nil {
			t.Fatalf("os.WriteFile(%s) error = %v", name, err)
		}
	}
}

func writeRuntimeArtifacts(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(runtime artifacts) error = %v", err)
	}
	for _, name := range RuntimeArtifactNames() {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%s) error = %v", name, err)
		}
	}
}

func writeMigrationStreams(t *testing.T, root string) {
	t.Helper()
	versions := map[string]string{
		schemaStreamGlobal:    "00057_schema.sql",
		schemaStreamMemory:    "00001_baseline.sql",
		schemaStreamSession:   "00006_schema.sql",
		schemaStreamWorkspace: "00001_baseline.sql",
	}
	for _, stream := range schemaStreamSpecs {
		dir := filepath.Join(root, stream.dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%s migrations) error = %v", stream.name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, versions[stream.name]), []byte("-- migration"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%s migration) error = %v", stream.name, err)
		}
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", path, err)
	}
	return contents
}
