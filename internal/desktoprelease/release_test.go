package desktoprelease

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReleasePolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse the reserved stable channel", func(t *testing.T) {
		t.Parallel()
		assertErrorContains(t, ValidateDesktopChannel(ChannelStable), "stable channel is reserved")
	})

	t.Run("Should require candidates to cross strict prerelease boundaries", func(t *testing.T) {
		t.Parallel()
		if err := AssertStrictlyGreater("0.4.0-beta.10", "0.4.0-beta.9"); err != nil {
			t.Fatalf("AssertStrictlyGreater(valid) error = %v", err)
		}
		for _, candidate := range []string{"0.4.0-beta.9", "0.4.0-beta.8", "0.3.99"} {
			assertErrorContains(t, AssertStrictlyGreater(candidate, "0.4.0-beta.9"), "strictly greater")
		}
	})

	t.Run("Should enforce the previous-generation compatibility ceiling", func(t *testing.T) {
		t.Parallel()
		if err := AssertCompatibleWithPrevious("0.4.0-beta.9", "0.4.0-beta.9"); err != nil {
			t.Fatalf("AssertCompatibleWithPrevious(equal) error = %v", err)
		}
		assertErrorContains(
			t,
			AssertCompatibleWithPrevious("0.4.0-beta.10", "0.4.0-beta.9"),
			"exceeds previous channel app version",
		)
	})
}

func TestDesktopArtifactInventory(t *testing.T) {
	t.Parallel()
	const version = "0.4.0-beta.2"
	want := []string{
		"CompozyOS-0.4.0-beta.2-linux-x64.AppImage",
		"CompozyOS-0.4.0-beta.2-linux-x64.deb",
		"CompozyOS-0.4.0-beta.2-mac-arm64.dmg",
		"CompozyOS-0.4.0-beta.2-mac-arm64.zip",
		"CompozyOS-0.4.0-beta.2-mac-x64.dmg",
		"CompozyOS-0.4.0-beta.2-mac-x64.zip",
	}
	got := DesktopArtifactNames(version)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("DesktopArtifactNames() = %v, want %v", got, want)
	}

	for _, test := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{name: "Should reject a missing package", mutate: func(t *testing.T, dir string) {
			t.Helper()
			removeTestFile(t, filepath.Join(dir, want[0]))
		}},
		{name: "Should reject an extra package", mutate: func(t *testing.T, dir string) {
			t.Helper()
			writeTestFile(t, filepath.Join(dir, "unexpected.zip"), []byte("extra"))
		}},
		{name: "Should reject an empty package", mutate: func(t *testing.T, dir string) {
			t.Helper()
			writeTestFile(t, filepath.Join(dir, want[0]), nil)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeDesktopArtifacts(t, dir, version)
			test.mutate(t, dir)
			if err := AssertExactDesktopInventory(t.Context(), dir, version); err == nil {
				t.Fatal("AssertExactDesktopInventory() error = nil")
			}
		})
	}
}

func TestChannelAuthority(t *testing.T) {
	t.Parallel()

	t.Run("Should bootstrap an empty channel", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "")
		authority := mustAuthority(t, backend)
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.1", "1.0.0-beta.1")
		result, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-bootstrap", Channel: ChannelBeta, Version: "1.0.0-beta.1",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		if result.ChannelRefBefore != "" || result.ChannelRefAfter == "" {
			t.Fatalf("Publish() refs = %q -> %q, want empty -> commit", result.ChannelRefBefore, result.ChannelRefAfter)
		}
	})

	t.Run("Should verify every payload before the channel ref CAS", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		authority := mustAuthority(t, backend)
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
		result, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-002", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		if result.Outcome != "published" || result.AuditCommit != result.ChannelRefAfter {
			t.Fatalf("Publish() result = %#v", result)
		}
		backend.assertAllVerificationPrecedesCAS(t)
	})

	t.Run("Should refuse manifests from a different generation", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		authority := mustAuthority(t, backend)
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
		writeTestFile(t, filepath.Join(channelDir, ManifestLinux), linuxManifestFixture("1.0.0-beta.1"))
		before := backend.head
		_, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-wrong-manifest", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if CodeOf(err) != ErrorVerificationFailed || backend.head != before {
			t.Fatalf("Publish() error = %v, head = %s, want verification failure at %s", err, backend.head, before)
		}
	})

	t.Run("Should keep the provider on a complete generation across interruption and repair", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		authority := mustAuthority(t, backend)
		provider := httptest.NewServer(http.HandlerFunc(backend.serveChannelFile))
		t.Cleanup(provider.Close)

		backend.failVerification = true
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
		_, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-interrupted", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if CodeOf(err) != ErrorVerificationFailed {
			t.Fatalf("CodeOf(interrupted Publish error) = %q, want %q; error = %v", CodeOf(err), ErrorVerificationFailed, err)
		}
		assertProviderGeneration(t, provider.URL, "1.0.0-beta.1")

		backend.failVerification = false
		_, err = authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-complete", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		assertProviderGeneration(t, provider.URL, "1.0.0-beta.2")

		result, err := authority.Repair(t.Context(), RepairRequest{
			OperationID: "repair-provider", Channel: ChannelBeta, Version: "1.0.0-beta.1", PublishedAt: testTime(),
		})
		if err != nil {
			t.Fatalf("Repair() error = %v", err)
		}
		if result.AuditCommit == "" || result.ChannelRefAfter != result.AuditCommit {
			t.Fatalf("Repair() result = %#v, want an audited channel commit", result)
		}
		assertProviderGeneration(t, provider.URL, "1.0.0-beta.1")
	})

	t.Run("Should return channel_cas_conflict after a lost race", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		backend.casConflict = true
		authority := mustAuthority(t, backend)
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
		_, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-race", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if CodeOf(err) != ErrorChannelCASConflict {
			t.Fatalf("CodeOf(Publish error) = %q, want %q; error = %v", CodeOf(err), ErrorChannelCASConflict, err)
		}
	})

	t.Run("Should converge when the same operation is rerun", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		authority := mustAuthority(t, backend)
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
		request := PublishRequest{
			OperationID: "publish-idempotent", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		}
		if _, err := authority.Publish(t.Context(), request); err != nil {
			t.Fatalf("first Publish() error = %v", err)
		}
		commitsBefore := backend.commitCount()
		result, err := authority.Publish(t.Context(), request)
		if err != nil {
			t.Fatalf("second Publish() error = %v", err)
		}
		if result.Outcome != "already_completed" || backend.commitCount() != commitsBefore {
			t.Fatalf("idempotent result = %#v, commits = %d, want %d", result, backend.commitCount(), commitsBefore)
		}
	})

	t.Run("Should repair from a known-good generation and refuse missing assets", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		authority := mustAuthority(t, backend)
		knownGood := backend.head
		backend.seedGeneration(t, "1.0.0-beta.2")
		result, err := authority.Repair(t.Context(), RepairRequest{
			OperationID: "repair-001", Channel: ChannelBeta, Version: "1.0.0-beta.1", PublishedAt: testTime(),
		})
		if err != nil {
			t.Fatalf("Repair() error = %v", err)
		}
		if result.Outcome != "repaired" || backend.commits[result.AuditCommit].Generation.SourceCommit != knownGood {
			t.Fatalf("Repair() result = %#v", result)
		}

		backend = newFakeAuthorityBackend(t, "1.0.0-beta.1")
		delete(backend.releases["1.0.0-beta.1"], DesktopArtifactNames("1.0.0-beta.1")[0])
		authority = mustAuthority(t, backend)
		_, err = authority.Repair(t.Context(), RepairRequest{
			OperationID: "repair-missing", Channel: ChannelBeta, Version: "1.0.0-beta.1", PublishedAt: testTime(),
		})
		if CodeOf(err) != ErrorInventoryIncomplete {
			t.Fatalf("CodeOf(Repair error) = %q, want %q; error = %v", CodeOf(err), ErrorInventoryIncomplete, err)
		}
	})
}

func TestCompatibilityBumpProtocol(t *testing.T) {
	t.Parallel()
	backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
	authority := mustAuthority(t, backend)

	badAssets, badChannel := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.2")
	_, err := authority.Publish(t.Context(), PublishRequest{
		OperationID: "bad-bump", Channel: ChannelBeta, Version: "1.0.0-beta.2",
		AssetDir: badAssets, ChannelDir: badChannel, PublishedAt: testTime(),
	})
	if CodeOf(err) != ErrorVerificationFailed {
		t.Fatalf("incompatible Publish() code = %q, want %q; error = %v", CodeOf(err), ErrorVerificationFailed, err)
	}

	for _, release := range []struct {
		version string
		minimum string
	}{
		{version: "1.0.0-beta.2", minimum: "1.0.0-beta.1"},
		{version: "1.0.0-beta.3", minimum: "1.0.0-beta.2"},
	} {
		assetDir, channelDir := writeReleaseFixture(t, release.version, release.minimum)
		_, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-" + release.version, Channel: ChannelBeta, Version: release.version,
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if err != nil {
			t.Fatalf("Publish(%s) error = %v", release.version, err)
		}
	}
}

type fakeAuthorityBackend struct {
	mu               sync.Mutex
	head             string
	commits          map[string]ChannelCommit
	releases         map[string]map[string]Artifact
	events           []string
	nextCommit       int
	casConflict      bool
	failVerification bool
}

func newFakeAuthorityBackend(t *testing.T, version string) *fakeAuthorityBackend {
	t.Helper()
	backend := &fakeAuthorityBackend{commits: map[string]ChannelCommit{}, releases: map[string]map[string]Artifact{}}
	if version != "" {
		backend.seedGeneration(t, version)
	}
	return backend
}

func (b *fakeAuthorityBackend) seedGeneration(t *testing.T, version string) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextCommit++
	sha := fmt.Sprintf("commit-%d", b.nextCommit)
	generation := Generation{
		OperationID: "seed-" + version, Operation: "publish", Version: version,
		MinAppVersion: version, PublishedAt: testTime(),
	}
	files := map[string][]byte{
		filepath.Join(ChannelDirectory, ManifestMac):   macManifestFixture(version),
		filepath.Join(ChannelDirectory, ManifestLinux): linuxManifestFixture(version),
	}
	bytes, err := canonicalJSON(generation)
	if err != nil {
		t.Fatalf("canonicalJSON(generation) error = %v", err)
	}
	files[filepath.Join(ChannelDirectory, GenerationFile)] = bytes
	b.commits[sha] = ChannelCommit{SHA: sha, Generation: generation, Files: files}
	b.head = sha
	assets := map[string]Artifact{}
	for _, name := range append(DesktopArtifactNames(version), CompatibilityFile) {
		assets[name] = Artifact{Name: name, SHA256: strings.Repeat("a", 64), Size: 1}
	}
	b.releases[version] = assets
}

func (b *fakeAuthorityBackend) ChannelRef(context.Context, string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.head == "" {
		return "", ErrChannelRefNotFound
	}
	return b.head, nil
}

func (b *fakeAuthorityBackend) FindOperation(_ context.Context, _ string, operationID string) (*ChannelCommit, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for sha := b.head; sha != ""; {
		commit := b.commits[sha]
		if commit.Generation.OperationID == operationID {
			copy := commit
			return &copy, nil
		}
		sha = parentOf(commit)
	}
	return nil, nil
}

func (b *fakeAuthorityBackend) Commit(_ context.Context, request CommitRequest) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, "commit")
	b.nextCommit++
	sha := fmt.Sprintf("commit-%d", b.nextCommit)
	files := cloneFiles(request.Files)
	files[".parent"] = []byte(request.ParentSHA)
	b.commits[sha] = ChannelCommit{SHA: sha, Generation: request.Generation, Files: files}
	return sha, nil
}

func (b *fakeAuthorityBackend) CompareAndSwapRef(_ context.Context, _ string, expectedSHA, nextSHA string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, "cas")
	if b.casConflict || b.head != expectedSHA {
		return ErrCompareAndSwap
	}
	b.head = nextSHA
	return nil
}

func (b *fakeAuthorityBackend) ReleaseInventory(_ context.Context, version string) ([]Artifact, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	assets := b.releases[version]
	result := make([]Artifact, 0, len(assets))
	for _, artifact := range assets {
		result = append(result, artifact)
	}
	return result, nil
}

func (b *fakeAuthorityBackend) UploadAsset(_ context.Context, version, _ string, artifact Artifact) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, "upload:"+artifact.Name)
	if b.releases[version] == nil {
		b.releases[version] = map[string]Artifact{}
	}
	b.releases[version][artifact.Name] = artifact
	return nil
}

func (b *fakeAuthorityBackend) VerifyAsset(_ context.Context, version string, artifact Artifact) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, "verify:"+artifact.Name)
	if b.failVerification {
		return errors.New("injected remote verification failure")
	}
	remote, ok := b.releases[version][artifact.Name]
	if !ok || remote.Size != artifact.Size || remote.SHA256 != artifact.SHA256 {
		return fmt.Errorf("remote artifact %s does not match", artifact.Name)
	}
	return nil
}

func (b *fakeAuthorityBackend) serveChannelFile(writer http.ResponseWriter, request *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	name := strings.TrimPrefix(request.URL.Path, "/"+ChannelBranchPrefix+ChannelBeta+"/")
	commit, ok := b.commits[b.head]
	if !ok {
		http.NotFound(writer, request)
		return
	}
	contents, ok := commit.Files[name]
	if !ok {
		http.NotFound(writer, request)
		return
	}
	writer.WriteHeader(http.StatusOK)
	if _, err := writer.Write(contents); err != nil {
		panic(fmt.Sprintf("write provider response: %v", err))
	}
}

func assertProviderGeneration(t *testing.T, providerURL, wantVersion string) {
	t.Helper()
	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		providerURL+"/"+ChannelBranchPrefix+ChannelBeta+"/"+ChannelDirectory+"/"+GenerationFile,
		nil,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("provider request error = %v", err)
	}
	contents, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		t.Fatalf("io.ReadAll(provider) error = %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("provider response close error = %v", closeErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("provider status = %d, body = %s", response.StatusCode, contents)
	}
	generation, err := DecodeGeneration(contents)
	if err != nil {
		t.Fatalf("DecodeGeneration(provider) error = %v", err)
	}
	if generation.Version != wantVersion {
		t.Fatalf("provider generation = %s, want %s", generation.Version, wantVersion)
	}
}

func (b *fakeAuthorityBackend) ReadCommit(_ context.Context, sha string) (ChannelCommit, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	commit, ok := b.commits[sha]
	if !ok {
		return ChannelCommit{}, errors.New("commit not found")
	}
	return commit, nil
}

func (b *fakeAuthorityBackend) KnownGood(_ context.Context, _ string, version string) (ChannelCommit, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, commit := range b.commits {
		if commit.Generation.Version == version {
			return commit, nil
		}
	}
	return ChannelCommit{}, errors.New("known-good generation not found")
}

func (b *fakeAuthorityBackend) commitCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.commits)
}

func (b *fakeAuthorityBackend) assertAllVerificationPrecedesCAS(t *testing.T) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	commit := slices.Index(b.events, "commit")
	cas := slices.Index(b.events, "cas")
	if commit == -1 || cas == -1 || commit >= cas {
		t.Fatalf("authority events = %v, want commit before CAS", b.events)
	}
	for index, event := range b.events {
		if strings.HasPrefix(event, "verify:") && index > commit {
			t.Fatalf("authority events = %v, verification occurred after commit", b.events)
		}
	}
}

func parentOf(commit ChannelCommit) string {
	return string(commit.Files[".parent"])
}

func mustAuthority(t *testing.T, backend AuthorityBackend) *Authority {
	t.Helper()
	authority, err := NewAuthority(backend)
	if err != nil {
		t.Fatalf("NewAuthority() error = %v", err)
	}
	return authority
}

func writeReleaseFixture(t *testing.T, version, minAppVersion string) (string, string) {
	t.Helper()
	assetDir := t.TempDir()
	writeDesktopArtifacts(t, assetDir, version)
	if err := WriteCompatibility(filepath.Join(assetDir, CompatibilityFile), Compatibility{
		RuntimeVersion: version, MinAppVersion: minAppVersion,
	}); err != nil {
		t.Fatalf("WriteCompatibility() error = %v", err)
	}
	artifacts, err := InspectDesktopInventory(t.Context(), assetDir, version)
	if err != nil {
		t.Fatalf("InspectDesktopInventory() error = %v", err)
	}
	compatibility, err := inspectArtifact(filepath.Join(assetDir, CompatibilityFile))
	if err != nil {
		t.Fatalf("inspectArtifact(compat.json) error = %v", err)
	}
	artifacts = append(artifacts, compatibility)
	var catalog strings.Builder
	for _, artifact := range artifacts {
		fmt.Fprintf(&catalog, "%s  %s\n", artifact.SHA256, artifact.Name)
	}
	writeTestFile(t, filepath.Join(assetDir, ChecksumsFile), []byte(catalog.String()))

	channelDir := t.TempDir()
	writeTestFile(t, filepath.Join(channelDir, ManifestMac), macManifestFixture(version))
	writeTestFile(t, filepath.Join(channelDir, ManifestLinux), linuxManifestFixture(version))
	return assetDir, channelDir
}

func writeDesktopArtifacts(t *testing.T, dir, version string) {
	t.Helper()
	for _, name := range DesktopArtifactNames(version) {
		writeTestFile(t, filepath.Join(dir, name), []byte("artifact:"+name))
	}
}

func macManifestFixture(version string) []byte {
	return []byte(fmt.Sprintf(
		"version: %s\nfiles:\n  - url: https://github.com/compozy/compozy/releases/download/v%s/CompozyOS-%s-mac-arm64.zip\n    sha512: arm64\n    size: 10\n  - url: https://github.com/compozy/compozy/releases/download/v%s/CompozyOS-%s-mac-x64.zip\n    sha512: x64\n    size: 10\nreleaseDate: '2026-08-16T12:00:00Z'\n",
		version, version, version, version, version,
	))
}

func linuxManifestFixture(version string) []byte {
	return []byte(fmt.Sprintf(
		"version: %s\nfiles:\n  - url: https://github.com/compozy/compozy/releases/download/v%s/CompozyOS-%s-linux-x64.AppImage\n    sha512: linux\n    size: 10\nreleaseDate: '2026-08-16T12:00:00Z'\n",
		version, version, version,
	))
}

func writeTestFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
}

func removeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove(%s) error = %v", path, err)
	}
}

func testTime() time.Time {
	return time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
}

func assertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("error = %v, want message containing %q", err, expected)
	}
}
