package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/fileutil"
)

type tarEntry struct {
	name     string
	content  string
	mode     int64
	typeflag byte
	linkname string
	format   tar.Format
}

func TestExtractArchive_ValidArchiveProducesDirectoryStructure(t *testing.T) {
	t.Parallel()

	t.Run("Should extract valid archive into expected directory structure", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		extractRoot := openArchiveTestRoot(t, root)
		archive := mustTarGz(t, []tarEntry{
			{name: "review/assets", typeflag: tar.TypeDir},
			{name: "review/SKILL.md", content: "name: review\n"},
			{name: "review/docs/guide.md", content: "guide"},
			{name: "review/scripts/run.sh", content: "echo ok\n"},
		})

		if err := extractArchive(bytes.NewReader(archive), extractRoot, extractLimits{}); err != nil {
			t.Fatalf("extractArchive() error = %v", err)
		}

		checks := []struct {
			name    string
			path    string
			content string
		}{
			{
				name:    "Should read expected skill document",
				path:    filepath.Join(root, "review", "SKILL.md"),
				content: "name: review\n",
			},
			{
				name:    "Should read expected guide",
				path:    filepath.Join(root, "review", "docs", "guide.md"),
				content: "guide",
			},
			{
				name:    "Should read expected script",
				path:    filepath.Join(root, "review", "scripts", "run.sh"),
				content: "echo ok\n",
			},
		}

		for _, check := range checks {
			t.Run(check.name, func(t *testing.T) {
				t.Parallel()
				data, err := os.ReadFile(check.path)
				if err != nil {
					t.Fatalf("ReadFile(%q) error = %v", check.path, err)
				}
				if got := string(data); got != check.content {
					t.Fatalf("ReadFile(%q) = %q, want %q", check.path, got, check.content)
				}
			})
		}
	})

	t.Run("Should preserve distinct leading-space entry names during extraction", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		archive := mustTarGz(t, []tarEntry{
			{name: " review/SKILL.md", content: "leading-space"},
			{name: "review/SKILL.md", content: "plain"},
		})

		if err := extractArchive(bytes.NewReader(archive), openArchiveTestRoot(t, root), extractLimits{}); err != nil {
			t.Fatalf("extractArchive() error = %v", err)
		}

		for path, want := range map[string]string{
			filepath.Join(root, " review", "SKILL.md"): "leading-space",
			filepath.Join(root, "review", "SKILL.md"):  "plain",
		} {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, err)
			}
			if got := string(data); got != want {
				t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
			}
		}
	})
}

func TestExtractArchive_EnforcesLimitsAndRejectsUnsafeEntries(t *testing.T) {
	t.Parallel()

	t.Run("Should reject archives that exceed the decompressed size limit", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		extractRoot := openArchiveTestRoot(t, root)
		archive := mustTarGz(t, []tarEntry{
			{name: "review/SKILL.md", content: "0123456789"},
		})

		err := extractArchive(bytes.NewReader(archive), extractRoot, extractLimits{
			maxDecompressedSize: 5,
			maxFileCount:        DefaultMaxFileCount,
		})
		if !errors.Is(err, errArchiveTooLarge) {
			t.Fatalf("extractArchive(size limit) error = %v, want %v", err, errArchiveTooLarge)
		}

		target := filepath.Join(root, "review", "SKILL.md")
		info, statErr := os.Stat(target)
		switch {
		case statErr == nil && info.Size() > 5:
			t.Fatalf("partial file size = %d, want <= 5", info.Size())
		case statErr != nil && !errors.Is(statErr, os.ErrNotExist):
			t.Fatalf("Stat(%q) error = %v", target, statErr)
		}
	})

	t.Run("Should count raw TAR framing and extended-name metadata toward the decompressed limit", func(t *testing.T) {
		t.Parallel()

		for _, format := range []tar.Format{tar.FormatPAX, tar.FormatGNU} {
			t.Run(format.String(), func(t *testing.T) {
				t.Parallel()

				longName := strings.Repeat("portable-name-", 12) + "skill.md"
				archive := mustTarGz(t, []tarEntry{{
					name: longName, content: "x", format: format,
				}})
				rawSize := decompressedArchiveSize(t, archive)
				if rawSize <= 1 {
					t.Fatalf("decompressed archive size = %d, want > 1", rawSize)
				}

				err := extractArchive(bytes.NewReader(archive), openArchiveTestRoot(t, t.TempDir()), extractLimits{
					maxDecompressedSize: rawSize - 1,
					maxFileCount:        DefaultMaxFileCount,
				})
				if !errors.Is(err, errArchiveTooLarge) {
					t.Fatalf("extractArchive(%s raw limit) error = %v, want %v", format, err, errArchiveTooLarge)
				}
			})
		}
	})

	t.Run("Should count decompressed bytes after the logical TAR end marker", func(t *testing.T) {
		t.Parallel()

		archive := mustTarGz(t, []tarEntry{{name: "SKILL.md", content: "name: review\n"}})
		rawSize := decompressedArchiveSize(t, archive)
		archive = appendGzipMember(t, archive, "trailing decompressed payload")

		err := extractArchive(bytes.NewReader(archive), openArchiveTestRoot(t, t.TempDir()), extractLimits{
			maxDecompressedSize: rawSize,
			maxFileCount:        DefaultMaxFileCount,
		})
		if !errors.Is(err, errArchiveTooLarge) {
			t.Fatalf("extractArchive(trailing decompressed payload) error = %v, want %v", err, errArchiveTooLarge)
		}
	})

	t.Run("Should reject archives that exceed the file count limit", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		extractRoot := openArchiveTestRoot(t, root)
		archive := mustTarGz(t, []tarEntry{
			{name: "one.txt", content: "one"},
			{name: "two.txt", content: "two"},
			{name: "three.txt", content: "three"},
		})

		err := extractArchive(bytes.NewReader(archive), extractRoot, extractLimits{
			maxDecompressedSize: DefaultMaxDecompressedSize,
			maxFileCount:        2,
		})
		if !errors.Is(err, errArchiveTooManyFiles) {
			t.Fatalf("extractArchive(file count) error = %v, want %v", err, errArchiveTooManyFiles)
		}
	})

	t.Run("Should count implicit directories before materializing an entry", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		err := extractArchive(
			bytes.NewReader(mustTarGz(t, []tarEntry{{name: "one/two/SKILL.md", content: "name: review\n"}})),
			openArchiveTestRoot(t, root),
			extractLimits{maxDecompressedSize: DefaultMaxDecompressedSize, maxFileCount: 2},
		)
		if !errors.Is(err, errArchiveTooManyFiles) {
			t.Fatalf("extractArchive(implicit directory count) error = %v, want %v", err, errArchiveTooManyFiles)
		}
		if _, statErr := os.Stat(filepath.Join(root, "one")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(implicit directory after rejected reservation) error = %v, want %v", statErr, os.ErrNotExist)
		}
	})

	t.Run("Should reject paths deeper than the configured limit before materializing them", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		err := extractArchive(
			bytes.NewReader(mustTarGz(t, []tarEntry{{name: "one/two/three/SKILL.md", content: "name: review\n"}})),
			openArchiveTestRoot(t, root),
			extractLimits{
				maxDecompressedSize: DefaultMaxDecompressedSize,
				maxFileCount:        DefaultMaxFileCount,
				maxDepth:            3,
			},
		)
		if !errors.Is(err, errArchiveTooDeep) {
			t.Fatalf("extractArchive(depth limit) error = %v, want %v", err, errArchiveTooDeep)
		}
		if _, statErr := os.Stat(filepath.Join(root, "one")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(deep path after rejection) error = %v, want %v", statErr, os.ErrNotExist)
		}
	})

	t.Run("Should reject archive link entries", func(t *testing.T) {
		t.Parallel()

		for _, typeflag := range []byte{tar.TypeSymlink, tar.TypeLink} {
			t.Run(fmt.Sprintf("type-%d", typeflag), func(t *testing.T) {
				t.Parallel()

				archive := mustTarGz(t, []tarEntry{
					{name: "review/link", typeflag: typeflag, linkname: "../target"},
				})
				err := extractArchive(bytes.NewReader(archive), openArchiveTestRoot(t, t.TempDir()), extractLimits{})
				if !errors.Is(err, ErrUnsupportedArchiveEntryType) {
					t.Fatalf(
						"ExtractArchive(link type %d) error = %v, want %v",
						typeflag,
						err,
						ErrUnsupportedArchiveEntryType,
					)
				}
			})
		}
	})

	t.Run("Should reject archive paths that escape the root", func(t *testing.T) {
		t.Parallel()

		archive := mustTarGz(t, []tarEntry{
			{name: "../escape.txt", content: "nope"},
		})

		err := extractArchive(bytes.NewReader(archive), openArchiveTestRoot(t, t.TempDir()), extractLimits{})
		if err == nil {
			t.Fatal("ExtractArchive(traversal) error = nil, want failure")
		}
		if !errors.Is(err, ErrArchiveEntryEscapesRoot) {
			t.Fatalf("ExtractArchive(traversal) error = %v, want %v", err, ErrArchiveEntryEscapesRoot)
		}
	})

	t.Run("Should reject extraction through a symlinked parent", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("symlink semantics are platform-specific on windows")
		}

		root := t.TempDir()
		extractRoot := openArchiveTestRoot(t, root)
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.MkdirAll(outside, 0o755); err != nil {
			t.Fatalf("MkdirAll(outside) error = %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "review")); err != nil {
			t.Fatalf("Symlink(review) error = %v", err)
		}

		archive := mustTarGz(t, []tarEntry{
			{name: "review/SKILL.md", content: "name: review\n"},
		})

		err := extractArchive(bytes.NewReader(archive), extractRoot, extractLimits{})
		if err == nil {
			t.Fatal("ExtractArchive(symlinked parent) error = nil, want failure")
		}
		if !errors.Is(err, ErrPathTraversesSymlink) {
			t.Fatalf("ExtractArchive(symlinked parent) error = %v, want %v", err, ErrPathTraversesSymlink)
		}
		if _, statErr := os.Stat(filepath.Join(outside, "SKILL.md")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("outside manifest stat error = %v, want not-exist", statErr)
		}
	})

	t.Run("Should reject extraction through an internal symlinked parent", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("symlink semantics are platform-specific on windows")
		}

		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "safe"), 0o755); err != nil {
			t.Fatalf("Mkdir(safe) error = %v", err)
		}
		if err := os.Symlink("safe", filepath.Join(root, "review")); err != nil {
			t.Fatalf("Symlink(review) error = %v", err)
		}

		err := extractArchive(bytes.NewReader(mustTarGz(t, []tarEntry{
			{name: "review/SKILL.md", content: "name: review\n"},
		})), openArchiveTestRoot(t, root), extractLimits{})
		if !errors.Is(err, ErrPathTraversesSymlink) {
			t.Fatalf("ExtractArchive(internal symlinked parent) error = %v, want %v", err, ErrPathTraversesSymlink)
		}
		if _, statErr := os.Stat(filepath.Join(root, "safe", "SKILL.md")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(internal symlink target) error = %v, want not-exist", statErr)
		}
	})

	t.Run("Should reject extraction over an existing final symlink", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("symlink semantics are platform-specific on windows")
		}

		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "review"), 0o755); err != nil {
			t.Fatalf("Mkdir(review) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "target"), []byte("original"), 0o600); err != nil {
			t.Fatalf("WriteFile(target) error = %v", err)
		}
		if err := os.Symlink("../target", filepath.Join(root, "review", "SKILL.md")); err != nil {
			t.Fatalf("Symlink(SKILL.md) error = %v", err)
		}

		err := extractArchive(bytes.NewReader(mustTarGz(t, []tarEntry{
			{name: "review/SKILL.md", content: "name: review\n"},
		})), openArchiveTestRoot(t, root), extractLimits{})
		if !errors.Is(err, ErrPathTraversesSymlink) {
			t.Fatalf("ExtractArchive(final symlink) error = %v, want %v", err, ErrPathTraversesSymlink)
		}
		content, readErr := os.ReadFile(filepath.Join(root, "target"))
		if readErr != nil {
			t.Fatalf("ReadFile(target) error = %v", readErr)
		}
		if got := string(content); got != "original" {
			t.Fatalf("target content = %q, want original", got)
		}
	})

	t.Run("Should reject duplicate archive entries", func(t *testing.T) {
		t.Parallel()

		err := extractArchive(bytes.NewReader(mustTarGz(t, []tarEntry{
			{name: "review/SKILL.md", content: "first"},
			{name: "review/SKILL.md", content: "second"},
		})), openArchiveTestRoot(t, t.TempDir()), extractLimits{})
		if !errors.Is(err, ErrArchiveDuplicateEntry) {
			t.Fatalf("ExtractArchive(duplicate) error = %v, want %v", err, ErrArchiveDuplicateEntry)
		}
	})

	t.Run("Should reject a missing extraction root", func(t *testing.T) {
		t.Parallel()

		err := extractArchive(bytes.NewReader(mustTarGz(t, []tarEntry{
			{name: "review/SKILL.md", content: "name: review\n"},
		})), nil, extractLimits{})
		if err == nil {
			t.Fatal("ExtractArchive(nil root) error = nil, want failure")
		}
		if !errors.Is(err, ErrArchiveRootRequired) {
			t.Fatalf("ExtractArchive(nil root) error = %v, want %v", err, ErrArchiveRootRequired)
		}
	})

	t.Run("Should reject invalid gzip streams", func(t *testing.T) {
		t.Parallel()

		err := extractArchive(
			strings.NewReader("not-a-gzip-stream"),
			openArchiveTestRoot(t, t.TempDir()),
			extractLimits{},
		)
		if err == nil {
			t.Fatal("ExtractArchive(invalid gzip) error = nil, want failure")
		}
		if !errors.Is(err, gzip.ErrHeader) {
			t.Fatalf("ExtractArchive(invalid gzip) error = %v, want %v", err, gzip.ErrHeader)
		}
	})
}

func TestCleanArchiveEntryPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   string
		want    string
		wantErr error
	}{
		{
			name:    "Should reject Windows-style separators",
			entry:   "review\\SKILL.md",
			wantErr: fileutil.ErrInvalidPath,
		},
		{name: "Should reject empty entry paths", entry: "", wantErr: ErrArchiveEntryPathRequired},
		{
			name:    "Should reject absolute entry paths",
			entry:   "/tmp/skill.md",
			wantErr: ErrArchiveEntryMustBeRelative,
		},
		{
			name:    "Should reject Windows volume paths",
			entry:   "C:\\tmp\\skill.md",
			wantErr: ErrArchiveEntryMustBeRelative,
		},
		{
			name:    "Should reject alternate data streams",
			entry:   "review/SKILL.md:payload",
			wantErr: fileutil.ErrInvalidPath,
		},
		{
			name:    "Should reject Windows device names",
			entry:   "review/CON.md",
			wantErr: fileutil.ErrInvalidPath,
		},
		{
			name:    "Should reject Windows trailing-dot names",
			entry:   "review/skill.",
			wantErr: fileutil.ErrInvalidPath,
		},
		{
			name:    "Should reject Windows trailing-space names without normalization",
			entry:   "review/skill ",
			wantErr: fileutil.ErrInvalidPath,
		},
		{
			name:    "Should reject escaping entry paths",
			entry:   "../escape.txt",
			wantErr: ErrArchiveEntryEscapesRoot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := cleanArchiveEntryPath(tt.entry)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("cleanArchiveEntryPath(%q) error = %v", tt.entry, err)
				}
				if got != tt.want {
					t.Fatalf("cleanArchiveEntryPath(%q) = %q, want %q", tt.entry, got, tt.want)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("cleanArchiveEntryPath(%q) error = %v, want %v", tt.entry, err, tt.wantErr)
			}
		})
	}
}

func TestCleanupArchiveFileRemovesPartialFile(t *testing.T) {
	t.Parallel()

	t.Run("Should remove the partial direct child and preserve the source error", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		extractRoot := openArchiveTestRoot(t, root)
		parent, err := extractRoot.CreateDirectory("review", 0o755)
		if err != nil {
			t.Fatalf("CreateDirectory(review) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := parent.Close(); closeErr != nil {
				t.Errorf("Directory.Close(review) error = %v", closeErr)
			}
		})

		target := filepath.Join(root, "review", "SKILL.md")

		file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			t.Fatalf("OpenFile(target) error = %v", err)
		}
		if _, err := file.WriteString("partial"); err != nil {
			t.Fatalf("WriteString(target) error = %v", err)
		}

		baseErr := errors.New("write failed")
		err = cleanupArchiveFile(file, parent, "SKILL.md", baseErr, false)
		if err == nil {
			t.Fatal("cleanupArchiveFile() error = nil, want source failure")
		}
		if !errors.Is(err, baseErr) {
			t.Fatalf("cleanupArchiveFile() error = %v, want base error", err)
		}
		if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(partial file) error = %v, want os.ErrNotExist", statErr)
		}
	})
}

func TestMoveInstalledDir(t *testing.T) {
	t.Parallel()

	t.Run("Should install into a new target", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")

		result, err := MoveInstalledDir(source, target, false)
		if err != nil {
			t.Fatalf("MoveInstalledDir(new target) error = %v", err)
		}
		if result == nil || !result.Committed {
			t.Fatalf("MoveInstalledDir(new target) result = %#v, want committed", result)
		}

		data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target) error = %v", err)
		}
		if got := string(data); got != "new" {
			t.Fatalf("target content = %q, want new", got)
		}
	})

	t.Run("Should install into a new target when replacement is requested", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")

		result, err := MoveInstalledDir(source, target, true)
		if err != nil {
			t.Fatalf("MoveInstalledDir(replace new target) error = %v", err)
		}
		if result == nil || !result.Committed {
			t.Fatalf("MoveInstalledDir(replace new target) result = %#v, want committed", result)
		}

		data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target) error = %v", err)
		}
		if got := string(data); got != "new" {
			t.Fatalf("target content = %q, want new", got)
		}
	})

	t.Run("Should replace an existing target", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(target, "SKILL.md"), "old")

		result, err := MoveInstalledDir(source, target, true)
		if err != nil {
			t.Fatalf("MoveInstalledDir(replace) error = %v", err)
		}
		if result == nil || !result.Committed {
			t.Fatalf("MoveInstalledDir(replace) result = %#v, want committed", result)
		}

		data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target) error = %v", err)
		}
		if got := string(data); got != "new" {
			t.Fatalf("target content = %q, want new", got)
		}
	})

	t.Run("Should reject existing targets when replacement is disabled", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(target, "SKILL.md"), "old")

		result, err := MoveInstalledDir(source, target, false)
		if result != nil {
			t.Fatalf("MoveInstalledDir(no replace) result = %#v, want nil", result)
		}
		if !errors.Is(err, ErrInstallTargetExists) {
			t.Fatalf("MoveInstalledDir(no replace) error = %v, want %v", err, ErrInstallTargetExists)
		}
	})

	t.Run("Should restore the backup when replacement fails", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "missing-source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(target, "SKILL.md"), "old")

		if result, err := MoveInstalledDir(source, target, true); err == nil {
			t.Fatal("MoveInstalledDir(missing source) error = nil, want failure")
		} else if result != nil {
			t.Fatalf("MoveInstalledDir(missing source) result = %#v, want nil", result)
		}

		data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target after rollback) error = %v", err)
		}
		if got := string(data); got != "old" {
			t.Fatalf("target content after rollback = %q, want old", got)
		}
	})

	t.Run("Should require the install parent to exist", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		targetParent := filepath.Join(parent, "missing")
		target := filepath.Join(targetParent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")

		result, err := MoveInstalledDir(source, target, false)
		if err == nil {
			t.Fatal("MoveInstalledDir(missing parent) error = nil, want failure")
		}
		if result != nil {
			t.Fatalf("MoveInstalledDir(missing parent) result = %#v, want nil", result)
		}
		if _, statErr := os.Stat(targetParent); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(missing parent) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should complete a journaled replacement after a crash gap", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		journal, err := newInstallPublicationJournal("target")
		if err != nil {
			t.Fatalf("newInstallPublicationJournal() error = %v", err)
		}
		journal.Phase = installPublicationPhaseBackup
		writeTestFile(t, filepath.Join(parent, journal.IncomingName, "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(parent, journal.BackupName, "SKILL.md"), "old")
		root := openPublicationTestRoot(t, parent)
		journalName := installPublicationJournalName("target")
		if err := writeInstallPublicationJournal(root, journalName, journal, false); err != nil {
			t.Fatalf("writeInstallPublicationJournal() error = %v", err)
		}

		diagnostics, err := recoverInstalledDirPublication(root, "target")
		if err != nil {
			t.Fatalf("recoverInstalledDirPublication() error = %v", err)
		}
		if len(diagnostics) != 0 {
			t.Fatalf("recoverInstalledDirPublication() diagnostics = %#v, want none", diagnostics)
		}
		data, err := os.ReadFile(filepath.Join(parent, "target", "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(recovered target) error = %v", err)
		}
		if got := string(data); got != "new" {
			t.Fatalf("recovered target content = %q, want new", got)
		}
		for _, path := range []string{journalName, journal.IncomingName, journal.BackupName} {
			if _, statErr := os.Stat(filepath.Join(parent, path)); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", path, statErr)
			}
		}
	})

	t.Run("Should keep a committed target and remove its crash backup", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		journal, err := newInstallPublicationJournal("target")
		if err != nil {
			t.Fatalf("newInstallPublicationJournal() error = %v", err)
		}
		journal.Phase = installPublicationPhasePublished
		writeTestFile(t, filepath.Join(parent, "target", "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(parent, journal.BackupName, "SKILL.md"), "old")
		root := openPublicationTestRoot(t, parent)
		journalName := installPublicationJournalName("target")
		if err := writeInstallPublicationJournal(root, journalName, journal, false); err != nil {
			t.Fatalf("writeInstallPublicationJournal() error = %v", err)
		}

		if _, err := recoverInstalledDirPublication(root, "target"); err != nil {
			t.Fatalf("recoverInstalledDirPublication() error = %v", err)
		}
		data, err := os.ReadFile(filepath.Join(parent, "target", "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(committed target) error = %v", err)
		}
		if got := string(data); got != "new" {
			t.Fatalf("committed target content = %q, want new", got)
		}
		if _, statErr := os.Stat(filepath.Join(parent, journal.BackupName)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(crash backup) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should preserve a backup when an uncommitted target occupies the publication name", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		journal, err := newInstallPublicationJournal("target")
		if err != nil {
			t.Fatalf("newInstallPublicationJournal() error = %v", err)
		}
		journal.Phase = installPublicationPhaseBackup
		writeTestFile(t, filepath.Join(parent, "target", "SKILL.md"), "concurrent")
		writeTestFile(t, filepath.Join(parent, journal.BackupName, "SKILL.md"), "old")
		root := openPublicationTestRoot(t, parent)
		journalName := installPublicationJournalName("target")
		if err := writeInstallPublicationJournal(root, journalName, journal, false); err != nil {
			t.Fatalf("writeInstallPublicationJournal() error = %v", err)
		}

		_, err = recoverInstalledDirPublication(root, "target")
		if !errors.Is(err, ErrInstallPublicationRecoveryRequired) {
			t.Fatalf("recoverInstalledDirPublication() error = %v, want %v", err, ErrInstallPublicationRecoveryRequired)
		}
		for path, want := range map[string]string{
			"target":           "concurrent",
			journal.BackupName: "old",
		} {
			data, readErr := os.ReadFile(filepath.Join(parent, path, "SKILL.md"))
			if readErr != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, readErr)
			}
			if got := string(data); got != want {
				t.Fatalf("content for %q = %q, want %q", path, got, want)
			}
		}
		if _, err := os.Stat(filepath.Join(parent, journalName)); err != nil {
			t.Fatalf("Stat(publication journal) error = %v, want preserved journal", err)
		}
	})

	t.Run("Should preserve the journal when a concurrent target blocks replacement rollback", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		journal, err := newInstallPublicationJournal("target")
		if err != nil {
			t.Fatalf("newInstallPublicationJournal() error = %v", err)
		}
		journal.Phase = installPublicationPhaseBackup
		writeTestFile(t, filepath.Join(parent, "target", "SKILL.md"), "concurrent")
		writeTestFile(t, filepath.Join(parent, journal.IncomingName, "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(parent, journal.BackupName, "SKILL.md"), "old")
		root := openPublicationTestRoot(t, parent)
		journalName := installPublicationJournalName("target")
		if err := writeInstallPublicationJournal(root, journalName, journal, false); err != nil {
			t.Fatalf("writeInstallPublicationJournal() error = %v", err)
		}

		_, err = publishInstallReplacementTarget(
			root,
			root,
			"source",
			journalName,
			journal,
			true,
			nil,
		)
		if err == nil {
			t.Fatal("publishInstallReplacementTarget() error = nil, want collision rollback failure")
		}
		for path, want := range map[string]string{
			"target":           "concurrent",
			journal.BackupName: "old",
			"source":           "new",
		} {
			data, readErr := os.ReadFile(filepath.Join(parent, path, "SKILL.md"))
			if readErr != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, readErr)
			}
			if got := string(data); got != want {
				t.Fatalf("content for %q = %q, want %q", path, got, want)
			}
		}
		if _, err := os.Stat(filepath.Join(parent, journalName)); err != nil {
			t.Fatalf("Stat(publication journal) error = %v, want preserved journal", err)
		}
	})

	t.Run("Should preserve all directories when crash recovery is ambiguous", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		journal, err := newInstallPublicationJournal("target")
		if err != nil {
			t.Fatalf("newInstallPublicationJournal() error = %v", err)
		}
		journal.Phase = installPublicationPhaseBackup
		writeTestFile(t, filepath.Join(parent, "target", "SKILL.md"), "unknown")
		writeTestFile(t, filepath.Join(parent, journal.IncomingName, "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(parent, journal.BackupName, "SKILL.md"), "old")
		root := openPublicationTestRoot(t, parent)
		if err := writeInstallPublicationJournal(
			root,
			installPublicationJournalName("target"),
			journal,
			false,
		); err != nil {
			t.Fatalf("writeInstallPublicationJournal() error = %v", err)
		}

		_, err = recoverInstalledDirPublication(root, "target")
		if !errors.Is(err, ErrInstallPublicationRecoveryRequired) {
			t.Fatalf("recoverInstalledDirPublication() error = %v, want %v", err, ErrInstallPublicationRecoveryRequired)
		}
		for path, want := range map[string]string{
			"target":             "unknown",
			journal.IncomingName: "new",
			journal.BackupName:   "old",
		} {
			data, readErr := os.ReadFile(filepath.Join(parent, path, "SKILL.md"))
			if readErr != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, readErr)
			}
			if got := string(data); got != want {
				t.Fatalf("content for %q = %q, want %q", path, got, want)
			}
		}
	})

	t.Run("Should reject a concurrent publisher before inspecting its journal", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(target, "SKILL.md"), "old")
		root := openPublicationTestRoot(t, parent)
		publicationLock, err := acquireInstallPublicationLock(root, "target")
		if err != nil {
			t.Fatalf("acquireInstallPublicationLock() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := publicationLock.Close(); closeErr != nil {
				t.Errorf("installPublicationLock.Close() error = %v", closeErr)
			}
		})

		result, err := MoveInstalledDir(source, target, true)
		if !errors.Is(err, ErrInstallPublicationBusy) {
			t.Fatalf("MoveInstalledDir(concurrent) error = %v, want %v", err, ErrInstallPublicationBusy)
		}
		if result != nil {
			t.Fatalf("MoveInstalledDir(concurrent) result = %#v, want nil", result)
		}
		for path, want := range map[string]string{"source": "new", "target": "old"} {
			data, readErr := os.ReadFile(filepath.Join(parent, path, "SKILL.md"))
			if readErr != nil {
				t.Fatalf("ReadFile(%q) error = %v", path, readErr)
			}
			if got := string(data); got != want {
				t.Fatalf("content for %q = %q, want %q", path, got, want)
			}
		}
	})
}

func TestExtractArchivePreservesPermissionBits(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve permission bits from the archive", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		extractRoot := openArchiveTestRoot(t, root)
		archive := mustTarGz(t, []tarEntry{
			{name: "review", typeflag: tar.TypeDir, mode: 0o750},
			{name: "review/scripts", typeflag: tar.TypeDir, mode: 0o755},
			{name: "review/scripts/run.sh", content: "#!/bin/sh\necho ok\n", mode: 0o755},
		})

		if err := extractArchive(bytes.NewReader(archive), extractRoot, extractLimits{}); err != nil {
			t.Fatalf("ExtractArchive() error = %v", err)
		}

		scriptInfo, err := os.Stat(filepath.Join(root, "review", "scripts", "run.sh"))
		if err != nil {
			t.Fatalf("Stat(script) error = %v", err)
		}
		if got := scriptInfo.Mode().Perm(); got != 0o755 {
			t.Fatalf("script mode = %#o, want 0o755", got)
		}

		dirInfo, err := os.Stat(filepath.Join(root, "review"))
		if err != nil {
			t.Fatalf("Stat(review dir) error = %v", err)
		}
		if got := dirInfo.Mode().Perm(); got != 0o750 {
			t.Fatalf("review dir mode = %#o, want 0o750", got)
		}
	})
}

func TestExtractArchiveStripsSpecialPermissionBits(t *testing.T) {
	t.Parallel()

	t.Run("Should strip special permission bits", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		extractRoot := openArchiveTestRoot(t, root)
		archive := mustTarGz(t, []tarEntry{
			{name: "review", typeflag: tar.TypeDir, mode: 0o2750},
			{name: "review/run.sh", content: "#!/bin/sh\necho ok\n", mode: 0o4755},
		})

		if err := extractArchive(bytes.NewReader(archive), extractRoot, extractLimits{}); err != nil {
			t.Fatalf("ExtractArchive() error = %v", err)
		}

		dirInfo, err := os.Stat(filepath.Join(root, "review"))
		if err != nil {
			t.Fatalf("Stat(review dir) error = %v", err)
		}
		if got := dirInfo.Mode().Perm(); got != 0o750 {
			t.Fatalf("review dir mode = %#o, want 0o750 after stripping special bits", got)
		}

		fileInfo, err := os.Stat(filepath.Join(root, "review", "run.sh"))
		if err != nil {
			t.Fatalf("Stat(run.sh) error = %v", err)
		}
		if got := fileInfo.Mode().Perm(); got != 0o755 {
			t.Fatalf("run.sh mode = %#o, want 0o755 after stripping special bits", got)
		}
	})
}

func TestDefaultExtractLimits(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the default extraction limits", func(t *testing.T) {
		t.Parallel()
		if got, want := DefaultMaxDecompressedSize, int64(500*1024*1024); got != want {
			t.Fatalf("DefaultMaxDecompressedSize = %d, want %d", got, want)
		}
		if got, want := DefaultMaxFileCount, 10000; got != want {
			t.Fatalf("DefaultMaxFileCount = %d, want %d", got, want)
		}
		if got, want := DefaultMaxArchiveDepth, 64; got != want {
			t.Fatalf("DefaultMaxArchiveDepth = %d, want %d", got, want)
		}
	})
}

func mustTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}

		header := &tar.Header{
			Name:     entry.name,
			Mode:     0o644,
			Typeflag: typeflag,
			Linkname: entry.linkname,
			Format:   entry.format,
		}
		if entry.mode != 0 {
			header.Mode = entry.mode
		}
		switch typeflag {
		case tar.TypeDir:
			if entry.mode == 0 {
				header.Mode = 0o755
			}
		case tar.TypeReg:
			header.Size = int64(len(entry.content))
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", entry.name, err)
		}
		if typeflag == tar.TypeReg {
			if _, err := io.WriteString(tarWriter, entry.content); err != nil {
				t.Fatalf("Write(%q) error = %v", entry.name, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}
	return buffer.Bytes()
}

func decompressedArchiveSize(t *testing.T, archive []byte) int64 {
	t.Helper()

	reader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	contents, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read decompressed archive error = %v; close error = %v", readErr, closeErr)
	}
	return int64(len(contents))
}

func appendGzipMember(t *testing.T, archive []byte, payload string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	if _, err := buffer.Write(archive); err != nil {
		t.Fatalf("Write(existing archive) error = %v", err)
	}
	writer := gzip.NewWriter(&buffer)
	if _, err := io.WriteString(writer, payload); err != nil {
		t.Fatalf("Write(trailing gzip member) error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(trailing gzip member) error = %v", err)
	}
	return buffer.Bytes()
}

func openArchiveTestRoot(t *testing.T, path string) *fileutil.Directory {
	t.Helper()

	root, err := fileutil.OpenDirectory(path)
	if err != nil {
		t.Fatalf("OpenDirectory(%q) error = %v", path, err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("Directory.Close(%q) error = %v", path, closeErr)
		}
	})
	return root
}

func openPublicationTestRoot(t *testing.T, path string) *fileutil.Directory {
	t.Helper()

	root, err := fileutil.OpenDirectoryForMutation(path)
	if err != nil {
		t.Fatalf("OpenDirectoryForMutation(%q) error = %v", path, err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("Directory.Close(%q) error = %v", path, closeErr)
		}
	})
	return root
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
