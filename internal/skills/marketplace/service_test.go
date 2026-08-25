package marketplace

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	registrypkg "github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func timeNowForTest() time.Time {
	return time.Date(2026, time.May, 17, 12, 0, 0, 0, time.UTC)
}

type fakeInstallRegistry struct {
	archive        []byte
	detail         *registrypkg.Detail
	updateInfo     *registrypkg.UpdateInfo
	checkUpdateErr error
	checkUpdateFn  func(string, string) (*registrypkg.UpdateInfo, error)
}

type opaqueRegistryLookupError struct {
	cause error
}

func (e opaqueRegistryLookupError) Error() string {
	return "registry lookup failed with opaque backend code"
}

func (e opaqueRegistryLookupError) Unwrap() error {
	return e.cause
}

type fakeSkillResolver struct {
	skills map[string]*skills.Skill
}

type recordingSearchSource struct {
	query string
	opts  registrypkg.SearchOpts
}

func (s *recordingSearchSource) Name() string {
	return "recording-search"
}

func (s *recordingSearchSource) Capabilities() registrypkg.SourceCaps {
	return registrypkg.SourceCaps{Search: true}
}

func (s *recordingSearchSource) Search(
	_ context.Context,
	query string,
	opts registrypkg.SearchOpts,
) ([]registrypkg.Listing, error) {
	s.query = query
	s.opts = opts
	return []registrypkg.Listing{}, nil
}

func (s *recordingSearchSource) Info(
	context.Context,
	string,
) (*registrypkg.Detail, error) {
	return nil, registrypkg.ErrNotSupported
}

func (s *recordingSearchSource) Download(
	context.Context,
	string,
	registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	return nil, registrypkg.ErrNotSupported
}

func (s *recordingSearchSource) Close() error {
	return nil
}

func (r fakeSkillResolver) Get(name string) (*skills.Skill, bool) {
	skill, ok := r.skills[name]
	return skill, ok
}

func (r fakeInstallRegistry) Download(
	context.Context,
	string,
	registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	return &registrypkg.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader(r.archive)),
		Slug:        r.detail.Slug,
		Version:     r.detail.Version,
		ContentType: "application/gzip",
	}, nil
}

func (r fakeInstallRegistry) Info(context.Context, string) (*registrypkg.Detail, error) {
	return r.detail, nil
}

func (r fakeInstallRegistry) CheckUpdate(
	_ context.Context,
	slug string,
	currentVersion string,
) (*registrypkg.UpdateInfo, error) {
	if r.checkUpdateFn != nil {
		return r.checkUpdateFn(slug, currentVersion)
	}
	if r.checkUpdateErr != nil {
		return nil, r.checkUpdateErr
	}
	return r.updateInfo, nil
}

func TestPathInsideRoot(t *testing.T) {
	t.Parallel()

	t.Run("Should reject targets that escape the root through symlinks", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		outside := t.TempDir()
		linkPath := filepath.Join(root, "escape")
		if err := os.Symlink(outside, linkPath); err != nil {
			t.Fatalf("os.Symlink() error = %v", err)
		}
		outsideSkill := filepath.Join(outside, "SKILL.md")
		if err := os.WriteFile(outsideSkill, []byte("outside"), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		_, err := PathInsideRoot(root, filepath.Join(linkPath, "SKILL.md"))
		if !errors.Is(err, registrypkg.ErrPathOutsideRoot) {
			t.Fatalf("PathInsideRoot() error = %v, want ErrPathOutsideRoot", err)
		}
	})

	t.Run("Should preserve lexical targets that stay within the root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		targetDir := filepath.Join(root, "review")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll() error = %v", err)
		}
		targetPath := filepath.Join(targetDir, "SKILL.md")
		if err := os.WriteFile(targetPath, []byte("inside"), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		resolved, err := PathInsideRoot(root, targetPath)
		if err != nil {
			t.Fatalf("PathInsideRoot() error = %v", err)
		}
		want, err := filepath.EvalSymlinks(targetPath)
		if err != nil {
			t.Fatalf("EvalSymlinks(target) error = %v", err)
		}
		if got := resolved; got != want {
			t.Fatalf("PathInsideRoot() = %q, want resolved %q", got, want)
		}
	})

	t.Run("Should allow missing targets beneath the resolved root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		targetPath := filepath.Join(root, "review", "SKILL.md")

		resolved, err := PathInsideRoot(root, targetPath)
		if err != nil {
			t.Fatalf("PathInsideRoot() error = %v", err)
		}
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatalf("EvalSymlinks(root) error = %v", err)
		}
		want := filepath.Join(resolvedRoot, "review", "SKILL.md")
		if got := resolved; got != want {
			t.Fatalf("PathInsideRoot() = %q, want resolved %q", got, want)
		}
	})

	t.Run("Should return the validated canonical target for an internal symlink", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		realDir := filepath.Join(root, "real")
		if err := os.MkdirAll(realDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(real) error = %v", err)
		}
		target := filepath.Join(realDir, SkillMarkdownFileName)
		if err := os.WriteFile(target, []byte("inside"), 0o644); err != nil {
			t.Fatalf("WriteFile(target) error = %v", err)
		}
		alias := filepath.Join(root, "alias")
		if err := os.Symlink(realDir, alias); err != nil {
			t.Fatalf("Symlink(alias) error = %v", err)
		}

		resolved, err := PathInsideRoot(root, filepath.Join(alias, SkillMarkdownFileName))
		if err != nil {
			t.Fatalf("PathInsideRoot() error = %v", err)
		}
		want, err := filepath.EvalSymlinks(target)
		if err != nil {
			t.Fatalf("EvalSymlinks(target) error = %v", err)
		}
		if resolved != want {
			t.Fatalf("PathInsideRoot() = %q, want canonical %q", resolved, want)
		}
	})

	t.Run("Should reject blank roots", func(t *testing.T) {
		t.Parallel()

		_, err := PathInsideRoot("   ", "review/SKILL.md")
		if !errors.Is(err, registrypkg.ErrPathRootRequired) {
			t.Fatalf("PathInsideRoot() error = %v, want ErrPathRootRequired", err)
		}
	})

	t.Run("Should reject percent-encoded traversal paths", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()

		_, err := PathInsideRoot(root, filepath.Join(root, "%2e%2e", "escape", "SKILL.md"))
		if !errors.Is(err, registrypkg.ErrPathOutsideRoot) {
			t.Fatalf("PathInsideRoot() error = %v, want ErrPathOutsideRoot", err)
		}
	})
}

func TestNormalizeSkillSlug(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		input           string
		want            string
		wantErr         error
		wantErrContains string
	}{
		{
			name:  "Should accept scoped marketplace slugs",
			input: "@acme/review",
			want:  "@acme/review",
		},
		{
			name:  "Should accept raw registry skill slugs",
			input: "review",
			want:  "review",
		},
		{
			name:  "Should trim accepted registry slugs",
			input: "  agentvibes-openclaw-skill  ",
			want:  "agentvibes-openclaw-skill",
		},
		{
			name:            "Should reject empty slugs",
			input:           "   ",
			wantErr:         ErrValidation,
			wantErrContains: "skill slug is required",
		},
		{
			name:            "Should reject raw slugs with path separators",
			input:           "acme/review",
			wantErr:         ErrValidation,
			wantErrContains: "skill slug must not include path separators",
		},
		{
			name:            "Should reject raw relative path segments",
			input:           "..",
			wantErr:         ErrValidation,
			wantErrContains: "skill slug must not be a relative path segment",
		},
		{
			name:            "Should reject raw slugs with unsupported characters",
			input:           "bad slug",
			wantErr:         ErrValidation,
			wantErrContains: `skill slug must match "@author/name"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeSkillSlug(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NormalizeSkillSlug(%q) error = %v, want %v", tc.input, err, tc.wantErr)
				}
				if tc.wantErrContains != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrContains)) {
					t.Fatalf(
						"NormalizeSkillSlug(%q) error = %v, want message containing %q",
						tc.input,
						err,
						tc.wantErrContains,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeSkillSlug(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeSkillSlug(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestServiceSearch(t *testing.T) {
	t.Parallel()

	t.Run("Should forward continuation pagination to the registry source", func(t *testing.T) {
		t.Parallel()

		source := &recordingSearchSource{}
		service := NewService(
			compozyconfig.HomePaths{},
			compozyconfig.SkillsConfig{},
			WithSourceLoader(func(compozyconfig.MarketplaceConfig) ([]registrypkg.Source, error) {
				return []registrypkg.Source{source}, nil
			}),
		)

		listings, err := service.Search(context.Background(), "review", 41, 7)
		if err != nil {
			t.Fatalf("Service.Search() error = %v", err)
		}
		if len(listings) != 0 {
			t.Fatalf("Service.Search() listings = %d, want 0", len(listings))
		}
		if source.query != "review" {
			t.Fatalf("registry query = %q, want %q", source.query, "review")
		}
		wantOpts := registrypkg.SearchOpts{
			Limit:  7,
			Offset: 41,
			Type:   registrypkg.PackageTypeSkill,
		}
		if source.opts != wantOpts {
			t.Fatalf("registry opts = %+v, want %+v", source.opts, wantOpts)
		}
	})
}

func TestInstallWithRegistryRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve manual skill directories on marketplace name collision", func(t *testing.T) {
		t.Parallel()

		skillsDir := t.TempDir()
		manualSkill := filepath.Join(skillsDir, "review")
		manualBody := "---\nname: review\ndescription: Manual skill\n---\nmanual body\n"
		if err := os.MkdirAll(manualSkill, 0o755); err != nil {
			t.Fatalf("MkdirAll(manual skill) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(manualSkill, SkillMarkdownFileName),
			[]byte(manualBody),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile(manual skill) error = %v", err)
		}

		registry := fakeInstallRegistry{
			archive: marketplaceSkillArchive(t, "review", "Marketplace skill", "marketplace body"),
			detail: &registrypkg.Detail{
				Listing: registrypkg.Listing{
					Slug:    "@acme/review",
					Name:    "review",
					Version: "2.0.0",
					Source:  "test-registry",
				},
			},
		}

		_, err := InstallWithRegistry(
			context.Background(),
			skillsDir,
			registry,
			"@acme/review",
			"",
			"",
			timeNowForTest,
		)
		if !errors.Is(err, ErrNotMarketplace) {
			t.Fatalf("InstallWithRegistry() error = %v, want ErrNotMarketplace", err)
		}
		body, readErr := os.ReadFile(filepath.Join(manualSkill, SkillMarkdownFileName))
		if readErr != nil {
			t.Fatalf("ReadFile(manual skill) error = %v", readErr)
		}
		if got := string(body); got != manualBody {
			t.Fatalf("manual skill body = %q, want %q", got, manualBody)
		}
	})

	t.Run("Should reject downloaded manifest names that cannot be managed locally", func(t *testing.T) {
		t.Parallel()

		registry := fakeInstallRegistry{
			archive: marketplaceSkillArchive(t, "parent/review", "Bad skill", "bad body"),
			detail: &registrypkg.Detail{
				Listing: registrypkg.Listing{
					Slug:    "@acme/review",
					Name:    "review",
					Version: "1.0.0",
					Source:  "test-registry",
				},
			},
		}

		_, err := InstallWithRegistry(
			context.Background(),
			t.TempDir(),
			registry,
			"@acme/review",
			"",
			"",
			timeNowForTest,
		)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("InstallWithRegistry() error = %v, want ErrValidation", err)
		}
	})
}

func TestUpdateSkillClassifiesRegistryLookupFailures(t *testing.T) {
	t.Parallel()

	typedCause := opaqueRegistryLookupError{cause: registrypkg.NewPackageNotFoundError("@acme/review")}
	joinedCause := opaqueRegistryLookupError{cause: errors.Join(
		errors.New("registry response was incomplete"),
		registrypkg.ErrPackageNotFound,
	)}
	textOnlyNotFound := errors.New("remote package not found")
	domainValidation := classifiedf(ErrValidation, "marketplace request is invalid")
	domainNotFound := classifiedf(ErrNotFound, "marketplace skill is absent")
	domainNotMarketplace := classifiedf(ErrNotMarketplace, "marketplace provenance is absent")
	domainNotConfigured := classifiedf(ErrNotConfigured, "marketplace source is unavailable")
	domainUnavailable := &Error{Kind: ErrUnavailable, Cause: registrypkg.ErrPackageNotFound}

	tests := []struct {
		name                    string
		registryErr             error
		wantMarketplaceNotFound bool
		wantRegistryNotFound    bool
		wantUnavailable         bool
		wantPassthrough         bool
	}{
		{
			name:                    "Should classify typed remote absence despite unrelated wording",
			registryErr:             typedCause,
			wantMarketplaceNotFound: true,
			wantRegistryNotFound:    true,
		},
		{
			name:                    "Should classify joined typed remote absence",
			registryErr:             joinedCause,
			wantMarketplaceNotFound: true,
			wantRegistryNotFound:    true,
		},
		{
			name:            "Should leave text-only remote absence unclassified",
			registryErr:     textOnlyNotFound,
			wantPassthrough: true,
		},
		{
			name:            "Should preserve marketplace validation failures",
			registryErr:     domainValidation,
			wantPassthrough: true,
		},
		{
			name:                    "Should preserve marketplace absence failures",
			registryErr:             domainNotFound,
			wantMarketplaceNotFound: true,
			wantPassthrough:         true,
		},
		{
			name:            "Should preserve non-marketplace failures",
			registryErr:     domainNotMarketplace,
			wantPassthrough: true,
		},
		{
			name:                 "Should preserve unavailable marketplace failures with registry absence causes",
			registryErr:          domainUnavailable,
			wantRegistryNotFound: true,
			wantUnavailable:      true,
			wantPassthrough:      true,
		},
		{
			name:            "Should preserve marketplace configuration failures",
			registryErr:     domainNotConfigured,
			wantPassthrough: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			installed := InstalledSkill{
				Name: "review",
				Dir:  t.TempDir(),
				Provenance: skills.Provenance{
					Registry:    "test-registry",
					Slug:        "@acme/review",
					Version:     "1.0.0",
					InstalledAt: timeNowForTest(),
				},
			}
			registry := fakeInstallRegistry{checkUpdateErr: test.registryErr}

			_, err := UpdateSkill(
				context.Background(),
				t.TempDir(),
				registry,
				installed,
				true,
				timeNowForTest,
			)
			if err == nil {
				t.Fatal("UpdateSkill() error = nil, want registry lookup failure")
			}
			if test.wantPassthrough && err != test.registryErr {
				t.Fatalf("UpdateSkill() error = %v, want original error %v", err, test.registryErr)
			}
			if got := errors.Is(err, ErrNotFound); got != test.wantMarketplaceNotFound {
				t.Fatalf("errors.Is(UpdateSkill() error, ErrNotFound) = %t, want %t", got, test.wantMarketplaceNotFound)
			}
			if got := errors.Is(err, registrypkg.ErrPackageNotFound); got != test.wantRegistryNotFound {
				t.Fatalf(
					"errors.Is(UpdateSkill() error, registry.ErrPackageNotFound) = %t, want %t",
					got,
					test.wantRegistryNotFound,
				)
			}
			if got := errors.Is(err, ErrUnavailable); got != test.wantUnavailable {
				t.Fatalf("errors.Is(UpdateSkill() error, ErrUnavailable) = %t, want %t", got, test.wantUnavailable)
			}
			if !errors.Is(err, test.registryErr) {
				t.Fatalf("UpdateSkill() error = %v, want preserved cause %v", err, test.registryErr)
			}
			if !test.wantMarketplaceNotFound || test.wantPassthrough {
				return
			}

			classified, classifiedMatched := errors.AsType[*Error](err)
			if !classifiedMatched {
				t.Fatalf("errors.As(UpdateSkill() error, *Error) = false, want classified marketplace error")
			}
			if classified.Cause != test.registryErr {
				t.Fatalf(
					"classified marketplace cause = %v, want original registry error %v",
					classified.Cause,
					test.registryErr,
				)
			}
			if got := err.Error(); got != test.registryErr.Error() {
				t.Fatalf("UpdateSkill() error text = %q, want preserved cause text %q", got, test.registryErr.Error())
			}
		})
	}
}

func TestUpdateWithRegistryPreservesSuccessfulBatchResults(t *testing.T) {
	t.Parallel()

	t.Run("Should return applied results alongside named aggregate failures", func(t *testing.T) {
		t.Parallel()

		skillsDir := t.TempDir()
		writeMarketplaceInstalledSkill(t, skillsDir, "alpha", "@acme/alpha", "1.0.0")
		writeMarketplaceInstalledSkill(t, skillsDir, "bravo", "@acme/bravo", "1.0.0")
		registry := fakeInstallRegistry{
			checkUpdateFn: func(slug string, _ string) (*registrypkg.UpdateInfo, error) {
				if slug == "@acme/bravo" {
					return nil, errors.New("registry offline")
				}
				return &registrypkg.UpdateInfo{CurrentVersion: "1.0.0", LatestVersion: "1.0.0"}, nil
			},
		}

		items, err := UpdateWithRegistry(
			context.Background(),
			skillsDir,
			registry,
			UpdateRequest{All: true, CheckOnly: true},
			timeNowForTest,
		)
		if err == nil || !strings.Contains(err.Error(), `update marketplace skill "bravo"`) ||
			!strings.Contains(err.Error(), "registry offline") {
			t.Fatalf("UpdateWithRegistry() error = %v, want named registry failure", err)
		}
		if len(items) != 1 || items[0].Name != "alpha" || items[0].Status != UpdateStatusCurrent {
			t.Fatalf("UpdateWithRegistry() items = %#v, want successful alpha result", items)
		}
	})
}

func TestVerifyInstallVisible(t *testing.T) {
	t.Parallel()

	result := InstallResult{
		Name:     "review",
		Slug:     "@compozy/review",
		Version:  "1.2.0",
		Registry: "clawhub",
		Path:     "/tmp/compozy/skills/review",
		Hash:     "sha256:abc",
		Status:   marketplaceInstallStatusInstalled,
	}
	visibleSkill := func() *skills.Skill {
		return &skills.Skill{
			Meta:     skills.SkillMeta{Name: "review"},
			Source:   skills.SourceMarketplace,
			Dir:      result.Path,
			FilePath: filepath.Join(result.Path, SkillMarkdownFileName),
			Enabled:  true,
			Provenance: &skills.Provenance{
				Hash:     "sha256:abc",
				Slug:     "@compozy/review",
				Registry: "clawhub",
				Version:  "1.2.0",
			},
		}
	}

	t.Run("Should accept visible marketplace skill with matching provenance", func(t *testing.T) {
		t.Parallel()

		err := VerifyInstallVisible(fakeSkillResolver{skills: map[string]*skills.Skill{
			"review": visibleSkill(),
		}}, result)
		if err != nil {
			t.Fatalf("VerifyInstallVisible() error = %v", err)
		}
	})

	cases := []struct {
		name     string
		resolver SkillResolver
		wantText string
	}{
		{
			name:     "Should classify missing discovery as unavailable",
			resolver: fakeSkillResolver{skills: map[string]*skills.Skill{}},
			wantText: "not visible after skill discovery",
		},
		{
			name: "Should classify shadowing source as unavailable",
			resolver: fakeSkillResolver{skills: map[string]*skills.Skill{
				"review": {
					Meta:     skills.SkillMeta{Name: "review"},
					Source:   skills.SourceUser,
					FilePath: "/tmp/compozy/skills/review/SKILL.md",
					Enabled:  true,
				},
			}},
			wantText: "resolved as user from /tmp/compozy/skills/review/SKILL.md",
		},
		{
			name: "Should classify missing provenance as unavailable",
			resolver: fakeSkillResolver{skills: map[string]*skills.Skill{
				"review": func() *skills.Skill {
					skill := visibleSkill()
					skill.Provenance = nil
					return skill
				}(),
			}},
			wantText: "missing provenance after skill discovery",
		},
		{
			name: "Should classify a discovered path mismatch as unavailable",
			resolver: fakeSkillResolver{skills: map[string]*skills.Skill{
				"review": func() *skills.Skill {
					skill := visibleSkill()
					skill.Dir = "/tmp/other/review"
					skill.FilePath = "/tmp/other/review/SKILL.md"
					return skill
				}(),
			}},
			wantText: "resolved from /tmp/other/review",
		},
		{
			name: "Should classify slug mismatch as unavailable",
			resolver: fakeSkillResolver{skills: map[string]*skills.Skill{
				"review": func() *skills.Skill {
					skill := visibleSkill()
					skill.Provenance.Slug = "@other/review"
					return skill
				}(),
			}},
			wantText: "resolved slug \"@other/review\" after skill discovery, want \"@compozy/review\"",
		},
		{
			name: "Should classify registry mismatch as unavailable",
			resolver: fakeSkillResolver{skills: map[string]*skills.Skill{
				"review": func() *skills.Skill {
					skill := visibleSkill()
					skill.Provenance.Registry = "other"
					return skill
				}(),
			}},
			wantText: "resolved registry \"other\" after skill discovery, want \"clawhub\"",
		},
		{
			name: "Should classify hash mismatch as unavailable",
			resolver: fakeSkillResolver{skills: map[string]*skills.Skill{
				"review": func() *skills.Skill {
					skill := visibleSkill()
					skill.Provenance.Hash = "sha256:different"
					return skill
				}(),
			}},
			wantText: "resolved hash \"sha256:different\" after skill discovery, want \"sha256:abc\"",
		},
		{
			name: "Should classify disabled skill as unavailable",
			resolver: fakeSkillResolver{skills: map[string]*skills.Skill{
				"review": func() *skills.Skill {
					skill := visibleSkill()
					skill.Enabled = false
					return skill
				}(),
			}},
			wantText: "visible but disabled after skill discovery",
		},
		{
			name:     "Should classify missing resolver as unavailable",
			resolver: nil,
			wantText: "skill discovery is not configured",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := VerifyInstallVisible(tc.resolver, result)
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("VerifyInstallVisible() error = %v, want ErrUnavailable", err)
			}
			if !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("VerifyInstallVisible() error = %v, want %q", err, tc.wantText)
			}
		})
	}
}

// Invariant: marketplace removal must complete exposure cleanup before deleting
// the canonical skill directory. Owning layer: marketplace service lifecycle.
// Canonical suite: service_test.go.
func TestServiceRemoveExposureLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should clean exposures before deleting the canonical directory", func(t *testing.T) {
		t.Parallel()
		skillsDir := t.TempDir()
		writeMarketplaceInstalledSkill(t, skillsDir, "review", "@acme/review", "1.0.0")
		canonical := filepath.Join(skillsDir, "review")
		canonicalReal, err := filepath.EvalSymlinks(canonical)
		if err != nil {
			t.Fatalf("EvalSymlinks(canonical) error = %v", err)
		}
		lifecycle := &recordingExposureLifecycle{cleanupFn: func(path string) error {
			if path != canonicalReal {
				t.Fatalf("cleanup path = %q, want %q", path, canonicalReal)
			}
			if _, err := os.Stat(canonical); err != nil {
				t.Fatalf("canonical missing before cleanup: %v", err)
			}
			return nil
		}}
		service := NewService(
			compozyconfig.HomePaths{SkillsDir: skillsDir},
			compozyconfig.SkillsConfig{},
			WithExposureLifecycle(lifecycle),
		)
		result, err := service.Remove(context.Background(), "review")
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		if result.Name != "review" || lifecycle.cleanupCalls != 1 {
			t.Fatalf("Remove() result=%#v cleanupCalls=%d", result, lifecycle.cleanupCalls)
		}
		if _, err := os.Stat(canonical); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("canonical still exists after remove, error = %v", err)
		}
	})

	t.Run("Should preserve the canonical directory when exposure cleanup fails", func(t *testing.T) {
		t.Parallel()
		skillsDir := t.TempDir()
		writeMarketplaceInstalledSkill(t, skillsDir, "review", "@acme/review", "1.0.0")
		canonical := filepath.Join(skillsDir, "review")
		blocked := errors.New("skill_remove_blocked: provider link is not removable")
		lifecycle := &recordingExposureLifecycle{cleanupFn: func(string) error { return blocked }}
		service := NewService(
			compozyconfig.HomePaths{SkillsDir: skillsDir},
			compozyconfig.SkillsConfig{},
			WithExposureLifecycle(lifecycle),
		)
		_, err := service.Remove(context.Background(), "review")
		if !errors.Is(err, blocked) {
			t.Fatalf("Remove() error = %v, want %v", err, blocked)
		}
		if _, err := os.Stat(canonical); err != nil {
			t.Fatalf("canonical removed despite cleanup failure: %v", err)
		}
	})
}

func TestServiceMarketplaceExposureLifecycleIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve exposures through update and clean them before remove", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		base := t.TempDir()
		skillsDir := filepath.Join(base, "compozy", "skills")
		writeMarketplaceInstalledSkill(t, skillsDir, "review", "@acme/review", "1.0.0")
		canonical := filepath.Join(skillsDir, "review")
		providerRoot := filepath.Join(base, "providers", "agents", "skills")
		database, err := globaldb.OpenGlobalDB(ctx, filepath.Join(base, store.GlobalDatabaseName))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := database.Close(context.Background()); err != nil {
				t.Errorf("Close(globaldb) error = %v", err)
			}
		})
		manager := skills.NewExposeManager(database, []compozyconfig.SkillRootSpec{{
			Dir: providerRoot, SourceSlug: compozyconfig.SkillSourceAgents, Kind: compozyconfig.RootKindPreset,
			ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		}})
		skill := &skills.Skill{
			Meta: skills.SkillMeta{Name: "review"}, Source: skills.SourceMarketplace, Dir: canonical,
			ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		}
		results, err := manager.Expose(ctx, skill, []string{compozyconfig.SkillSourceAgents})
		if err != nil || len(results) != 1 || !results[0].OK {
			t.Fatalf("Expose() results=%#v error=%v", results, err)
		}
		linkPath := results[0].Exposure.Record.LinkPath

		source := &lifecycleRegistrySource{
			archive: marketplaceSkillArchive(t, "review", "Updated skill", "updated body"),
			detail: registrypkg.Detail{Listing: registrypkg.Listing{
				Slug: "@acme/review", Name: "review", Version: "2.0.0", Source: "test-registry", Type: registrypkg.PackageTypeSkill,
			}},
		}
		service := NewService(
			compozyconfig.HomePaths{SkillsDir: skillsDir},
			compozyconfig.SkillsConfig{},
			WithExposureLifecycle(manager),
			WithSourceLoader(func(compozyconfig.MarketplaceConfig) ([]registrypkg.Source, error) {
				return []registrypkg.Source{source}, nil
			}),
		)
		updates, err := service.Update(ctx, UpdateRequest{Name: "review"})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if len(updates) != 1 || updates[0].Status != UpdateStatusUpdated {
			t.Fatalf("Update() results = %#v", updates)
		}
		states, err := manager.Exposures(ctx, skill)
		if err != nil || len(states) != 1 || states[0].Status != skills.ExposureHealthy {
			t.Fatalf("Exposures(after update) states=%#v error=%v", states, err)
		}
		if _, err := service.Remove(ctx, "review"); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		if _, err := os.Lstat(linkPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("provider link remains after remove, error = %v", err)
		}
		records, err := database.ListSkillExposuresByCanonicalDir(ctx, states[0].Record.CanonicalDir)
		if err != nil {
			t.Fatalf("ListSkillExposuresByCanonicalDir() error = %v", err)
		}
		if len(records) != 0 {
			t.Fatalf("exposure records remain after remove: %#v", records)
		}
	})
}

type recordingExposureLifecycle struct {
	cleanupCalls int
	verifyCalls  int
	cleanupFn    func(string) error
	verifyFn     func(string) error
}

type lifecycleRegistrySource struct {
	archive []byte
	detail  registrypkg.Detail
}

func (s *lifecycleRegistrySource) Name() string { return "test-registry" }
func (s *lifecycleRegistrySource) Capabilities() registrypkg.SourceCaps {
	return registrypkg.SourceCaps{}
}
func (s *lifecycleRegistrySource) Search(
	context.Context,
	string,
	registrypkg.SearchOpts,
) ([]registrypkg.Listing, error) {
	return nil, registrypkg.ErrNotSupported
}
func (s *lifecycleRegistrySource) Info(context.Context, string) (*registrypkg.Detail, error) {
	copy := s.detail
	return &copy, nil
}
func (s *lifecycleRegistrySource) Download(
	context.Context,
	string,
	registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	return &registrypkg.DownloadResult{
		Reader: io.NopCloser(bytes.NewReader(s.archive)), Slug: s.detail.Slug,
		Version: s.detail.Version, ContentType: "application/gzip",
	}, nil
}
func (s *lifecycleRegistrySource) Close() error { return nil }

func (l *recordingExposureLifecycle) CleanupCanonicalDir(_ context.Context, path string) error {
	l.cleanupCalls++
	if l.cleanupFn == nil {
		return nil
	}
	return l.cleanupFn(path)
}

func (l *recordingExposureLifecycle) VerifyCanonicalDir(_ context.Context, path string) error {
	l.verifyCalls++
	if l.verifyFn == nil {
		return nil
	}
	return l.verifyFn(path)
}

func marketplaceSkillArchive(t *testing.T, name string, description string, body string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	content := "---\nname: " + name + "\ndescription: " + description + "\nversion: 1.0.0\n---\n" + body + "\n"
	writeMarketplaceTarEntry(t, tarWriter, "skill/SKILL.md", content)
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}
	return buffer.Bytes()
}

func writeMarketplaceInstalledSkill(
	t *testing.T,
	skillsDir string,
	name string,
	slug string,
	version string,
) {
	t.Helper()

	skillDir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", name, err)
	}
	content := "---\nname: " + name + "\ndescription: Test skill\n---\nTest body\n"
	if err := os.WriteFile(filepath.Join(skillDir, SkillMarkdownFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
	hash, err := skills.ComputeDirectoryHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryHash(%s) error = %v", name, err)
	}
	if err := skills.WriteSidecar(skillDir, skills.Provenance{
		Hash: hash, Registry: "test-registry", Slug: slug, Version: version, InstalledAt: timeNowForTest(),
	}); err != nil {
		t.Fatalf("WriteSidecar(%s) error = %v", name, err)
	}
}

func writeMarketplaceTarEntry(t *testing.T, writer *tar.Writer, name string, content string) {
	t.Helper()

	header := &tar.Header{
		Name:     name,
		Mode:     0o644,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := writer.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader(%q) error = %v", name, err)
	}
	if _, err := io.WriteString(writer, content); err != nil {
		t.Fatalf("Write(%q) error = %v", name, err)
	}
}
