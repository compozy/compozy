package desktoprelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
		for _, candidate := range []struct {
			name    string
			version string
		}{
			{name: "Should reject an equal version", version: "0.4.0-beta.9"},
			{name: "Should reject an older prerelease", version: "0.4.0-beta.8"},
			{name: "Should reject an older release", version: "0.3.99"},
		} {
			t.Run(candidate.name, func(t *testing.T) {
				t.Parallel()
				assertErrorContains(t, AssertStrictlyGreater(candidate.version, "0.4.0-beta.9"), "strictly greater")
			})
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
		name        string
		wantMessage string
		mutate      func(*testing.T, string)
	}{
		{name: "Should reject a missing package", wantMessage: "artifact inventory", mutate: func(t *testing.T, dir string) {
			t.Helper()
			removeTestFile(t, filepath.Join(dir, want[0]))
		}},
		{name: "Should reject an extra package", wantMessage: "artifact inventory", mutate: func(t *testing.T, dir string) {
			t.Helper()
			writeTestFile(t, filepath.Join(dir, "unexpected.zip"), []byte("extra"))
		}},
		{name: "Should reject an empty package", wantMessage: "is not a non-empty regular file", mutate: func(t *testing.T, dir string) {
			t.Helper()
			writeTestFile(t, filepath.Join(dir, want[0]), nil)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeDesktopArtifacts(t, dir, version)
			test.mutate(t, dir)
			assertErrorContains(t, AssertExactDesktopInventory(t.Context(), dir, version), test.wantMessage)
		})
	}
}

func TestGitHubBackendSafety(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a plaintext GitHub API URL", func(t *testing.T) {
		t.Parallel()
		_, err := NewGitHubBackend(httpDoerFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("request should not be sent")
		}), "http://api.github.test", "compozy/compozy", "secret")
		assertErrorContains(t, err, "must be an HTTPS origin")
	})

	t.Run("Should preserve the binary accept header for asset verification", func(t *testing.T) {
		t.Parallel()
		contents := []byte("desktop-payload")
		digest := sha256.Sum256(contents)
		backend := &GitHubBackend{
			client: httpDoerFunc(func(request *http.Request) (*http.Response, error) {
				if got := request.Header.Get("Accept"); got != "application/octet-stream" {
					t.Fatalf("Accept = %q, want application/octet-stream", got)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(string(contents))),
				}, nil
			}),
			apiURL: "https://api.github.test", repository: "compozy/compozy", token: "secret",
		}
		err := backend.verifyDownloadedAsset(t.Context(), 42, Artifact{
			Name: "CompozyOS.zip", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(contents)),
		})
		if err != nil {
			t.Fatalf("verifyDownloadedAsset() error = %v", err)
		}
	})

	t.Run("Should scan every GitHub audit page", func(t *testing.T) {
		t.Parallel()
		pages := 0
		backend := &GitHubBackend{
			client: httpDoerFunc(func(request *http.Request) (*http.Response, error) {
				pages++
				page := request.URL.Query().Get("page")
				count := githubCommitPageSize
				if page == "2" {
					count = 1
				}
				commits := make([]githubAuditCommit, count)
				for index := range commits {
					commits[index].SHA = fmt.Sprintf("page-%s-commit-%d", page, index)
				}
				body, err := json.Marshal(commits)
				if err != nil {
					t.Fatalf("json.Marshal(commits) error = %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(string(body))),
				}, nil
			}),
			apiURL: "https://api.github.test", repository: "compozy/compozy", token: "secret",
		}
		visited := 0
		err := backend.walkAuditCommits(t.Context(), "channel-beta", func(githubAuditCommit) (bool, error) {
			visited++
			return false, nil
		})
		if err != nil {
			t.Fatalf("walkAuditCommits() error = %v", err)
		}
		if pages != 2 || visited != githubCommitPageSize+1 {
			t.Fatalf("pages = %d, visited = %d, want 2 and %d", pages, visited, githubCommitPageSize+1)
		}
	})
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
		assertErrorContains(t, err, "does not match generation")
	})

	t.Run("Should refuse manifest integrity that does not match the payload", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		authority := mustAuthority(t, backend)
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
		manifest := string(linuxManifestFixtureForAssets(t, assetDir, "1.0.0-beta.2"))
		manifest = strings.Replace(manifest, "sha512: ", "sha512: tampered", 1)
		writeTestFile(t, filepath.Join(channelDir, ManifestLinux), []byte(manifest))

		_, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-wrong-integrity", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if CodeOf(err) != ErrorVerificationFailed {
			t.Fatalf("CodeOf(Publish error) = %q, want %q; error = %v", CodeOf(err), ErrorVerificationFailed, err)
		}
		assertErrorContains(t, err, "integrity does not match")
	})

	t.Run("Should refuse manifest assets from another repository", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		authority := mustAuthority(t, backend)
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
		manifest := string(linuxManifestFixtureForAssets(t, assetDir, "1.0.0-beta.2"))
		manifest = strings.Replace(manifest, "github.com/compozy/compozy", "github.com/attacker/repository", 1)
		writeTestFile(t, filepath.Join(channelDir, ManifestLinux), []byte(manifest))

		_, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-wrong-repository", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if CodeOf(err) != ErrorVerificationFailed {
			t.Fatalf("CodeOf(Publish error) = %q, want %q; error = %v", CodeOf(err), ErrorVerificationFailed, err)
		}
		assertErrorContains(t, err, "not an immutable GitHub release URL")
	})

	t.Run("Should refuse an empty desktop payload", func(t *testing.T) {
		t.Parallel()
		backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
		authority := mustAuthority(t, backend)
		assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
		emptyName := DesktopArtifactNames("1.0.0-beta.2")[0]
		writeTestFile(t, filepath.Join(assetDir, emptyName), nil)

		_, err := authority.Publish(t.Context(), PublishRequest{
			OperationID: "publish-empty-payload", Channel: ChannelBeta, Version: "1.0.0-beta.2",
			AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
		})
		if CodeOf(err) != ErrorInventoryIncomplete {
			t.Fatalf("CodeOf(Publish error) = %q, want %q; error = %v", CodeOf(err), ErrorInventoryIncomplete, err)
		}
		assertErrorContains(t, err, "is empty")
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
			t.Fatalf(
				"CodeOf(interrupted Publish error) = %q, want %q; error = %v",
				CodeOf(err),
				ErrorVerificationFailed,
				err,
			)
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

	for _, mismatch := range []struct {
		name      string
		operation string
		version   string
	}{
		{name: "Should reject an operation id reused for another version", operation: operationPublish, version: "1.0.0-beta.3"},
		{name: "Should reject an operation id reused for another action", operation: operationRepair, version: "1.0.0-beta.2"},
	} {
		t.Run(mismatch.name, func(t *testing.T) {
			t.Parallel()
			backend := newFakeAuthorityBackend(t, "1.0.0-beta.1")
			authority := mustAuthority(t, backend)
			assetDir, channelDir := writeReleaseFixture(t, "1.0.0-beta.2", "1.0.0-beta.1")
			operationID := "publish-reused"
			_, err := authority.Publish(t.Context(), PublishRequest{
				OperationID: operationID, Channel: ChannelBeta, Version: "1.0.0-beta.2",
				AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
			})
			if err != nil {
				t.Fatalf("initial Publish() error = %v", err)
			}
			if mismatch.operation == operationPublish {
				assetDir, channelDir = writeReleaseFixture(t, mismatch.version, "1.0.0-beta.2")
				_, err = authority.Publish(t.Context(), PublishRequest{
					OperationID: operationID, Channel: ChannelBeta, Version: mismatch.version,
					AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
				})
			} else {
				_, err = authority.Repair(t.Context(), RepairRequest{
					OperationID: operationID, Channel: ChannelBeta, Version: mismatch.version, PublishedAt: testTime(),
				})
			}
			if CodeOf(err) != ErrorVerificationFailed {
				t.Fatalf(
					"CodeOf(reused operation error) = %q, want %q; error = %v",
					CodeOf(err),
					ErrorVerificationFailed,
					err,
				)
			}
			assertErrorContains(t, err, "already belongs to publish 1.0.0-beta.2")
		})
	}

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
		name    string
		version string
		minimum string
	}{
		{name: "Should publish the first compatible bump", version: "1.0.0-beta.2", minimum: "1.0.0-beta.1"},
		{name: "Should publish the next compatible bump", version: "1.0.0-beta.3", minimum: "1.0.0-beta.2"},
	} {
		t.Run(release.name, func(t *testing.T) {
			assetDir, channelDir := writeReleaseFixture(t, release.version, release.minimum)
			_, err := authority.Publish(t.Context(), PublishRequest{
				OperationID: "publish-" + release.version, Channel: ChannelBeta, Version: release.version,
				AssetDir: assetDir, ChannelDir: channelDir, PublishedAt: testTime(),
			})
			if err != nil {
				t.Fatalf("Publish(%s) error = %v", release.version, err)
			}
		})
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

func (b *fakeAuthorityBackend) Repository() string {
	return "compozy/compozy"
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
		OperationID: "seed-" + version, Operation: operationPublish, Version: version,
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
			commitCopy := commit
			return &commitCopy, nil
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
		http.NoBody,
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
		if _, err := fmt.Fprintf(&catalog, "%s  %s\n", artifact.SHA256, artifact.Name); err != nil {
			t.Fatalf("fmt.Fprintf(checksums catalog) error = %v", err)
		}
	}
	writeTestFile(t, filepath.Join(assetDir, ChecksumsFile), []byte(catalog.String()))

	channelDir := t.TempDir()
	writeTestFile(t, filepath.Join(channelDir, ManifestMac), macManifestFixtureForAssets(t, assetDir, version))
	writeTestFile(t, filepath.Join(channelDir, ManifestLinux), linuxManifestFixtureForAssets(t, assetDir, version))
	return assetDir, channelDir
}

func writeDesktopArtifacts(t *testing.T, dir, version string) {
	t.Helper()
	for _, name := range DesktopArtifactNames(version) {
		writeTestFile(t, filepath.Join(dir, name), []byte("artifact:"+name))
	}
}

func macManifestFixture(version string) []byte {
	return fmt.Appendf(
		nil,
		"version: %s\nfiles:\n  - url: https://github.com/compozy/compozy/releases/download/v%s/CompozyOS-%s-mac-arm64.zip\n    sha512: arm64\n    size: 10\n  - url: https://github.com/compozy/compozy/releases/download/v%s/CompozyOS-%s-mac-x64.zip\n    sha512: x64\n    size: 10\nreleaseDate: '2026-08-16T12:00:00Z'\n",
		version,
		version,
		version,
		version,
		version,
	)
}

func macManifestFixtureForAssets(t *testing.T, assetDir, version string) []byte {
	t.Helper()
	arm64Name := "CompozyOS-" + version + "-mac-arm64.zip"
	x64Name := "CompozyOS-" + version + "-mac-x64.zip"
	arm64Digest, arm64Size := manifestIntegrityFixture(t, assetDir, arm64Name)
	x64Digest, x64Size := manifestIntegrityFixture(t, assetDir, x64Name)
	return fmt.Appendf(
		nil,
		"version: %s\nfiles:\n  - url: https://github.com/compozy/compozy/releases/download/v%s/%s\n    sha512: %s\n    size: %d\n  - url: https://github.com/compozy/compozy/releases/download/v%s/%s\n    sha512: %s\n    size: %d\nreleaseDate: '2026-08-16T12:00:00Z'\n",
		version,
		version,
		arm64Name,
		arm64Digest,
		arm64Size,
		version,
		x64Name,
		x64Digest,
		x64Size,
	)
}

func linuxManifestFixture(version string) []byte {
	return fmt.Appendf(
		nil,
		"version: %s\nfiles:\n  - url: https://github.com/compozy/compozy/releases/download/v%s/CompozyOS-%s-linux-x64.AppImage\n    sha512: linux\n    size: 10\nreleaseDate: '2026-08-16T12:00:00Z'\n",
		version,
		version,
		version,
	)
}

func linuxManifestFixtureForAssets(t *testing.T, assetDir, version string) []byte {
	t.Helper()
	name := "CompozyOS-" + version + "-linux-x64.AppImage"
	digest, size := manifestIntegrityFixture(t, assetDir, name)
	return fmt.Appendf(
		nil,
		"version: %s\nfiles:\n  - url: https://github.com/compozy/compozy/releases/download/v%s/%s\n    sha512: %s\n    size: %d\nreleaseDate: '2026-08-16T12:00:00Z'\n",
		version,
		version,
		name,
		digest,
		size,
	)
}

func manifestIntegrityFixture(t *testing.T, assetDir, name string) (string, int64) {
	t.Helper()
	digest, size, err := inspectManifestIntegrity(filepath.Join(assetDir, name))
	if err != nil {
		t.Fatalf("inspectManifestIntegrity(%s) error = %v", name, err)
	}
	return digest, size
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

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (do httpDoerFunc) Do(request *http.Request) (*http.Response, error) {
	return do(request)
}

func assertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("error = %v, want message containing %q", err, expected)
	}
}
