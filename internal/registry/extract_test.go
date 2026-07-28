package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type tarEntry struct {
	name     string
	content  string
	mode     int64
	typeflag byte
	linkname string
}

func TestExtractArchive_ValidArchiveProducesDirectoryStructure(t *testing.T) {
	t.Parallel()

	t.Run("ShouldExtractValidArchiveIntoExpectedDirectoryStructure", func(t *testing.T) {
		root := t.TempDir()
		archive := mustTarGz(t, []tarEntry{
			{name: "review/assets", typeflag: tar.TypeDir},
			{name: "review/SKILL.md", content: "name: review\n"},
			{name: "review/docs/guide.md", content: "guide"},
			{name: "review/scripts/run.sh", content: "echo ok\n"},
		})

		if err := ExtractArchive(bytes.NewReader(archive), root); err != nil {
			t.Fatalf("ExtractArchive() error = %v", err)
		}

		checks := []struct {
			name    string
			path    string
			content string
		}{
			{
				name:    "ShouldReadExpectedSkillDocument",
				path:    filepath.Join(root, "review", "SKILL.md"),
				content: "name: review\n",
			},
			{
				name:    "ShouldReadExpectedGuide",
				path:    filepath.Join(root, "review", "docs", "guide.md"),
				content: "guide",
			},
			{
				name:    "ShouldReadExpectedScript",
				path:    filepath.Join(root, "review", "scripts", "run.sh"),
				content: "echo ok\n",
			},
		}

		for _, check := range checks {
			t.Run(check.name, func(t *testing.T) {
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
}

func TestExtractArchive_EnforcesLimitsAndRejectsUnsafeEntries(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectArchivesThatExceedTheDecompressedSizeLimit", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		archive := mustTarGz(t, []tarEntry{
			{name: "review/SKILL.md", content: "0123456789"},
		})

		err := extractArchive(bytes.NewReader(archive), root, extractLimits{
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

	t.Run("ShouldRejectArchivesThatExceedTheFileCountLimit", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		archive := mustTarGz(t, []tarEntry{
			{name: "one.txt", content: "one"},
			{name: "two.txt", content: "two"},
			{name: "three.txt", content: "three"},
		})

		err := extractArchive(bytes.NewReader(archive), root, extractLimits{
			maxDecompressedSize: DefaultMaxDecompressedSize,
			maxFileCount:        2,
		})
		if !errors.Is(err, errArchiveTooManyFiles) {
			t.Fatalf("extractArchive(file count) error = %v, want %v", err, errArchiveTooManyFiles)
		}
	})

	t.Run("ShouldRejectSymlinkEntries", func(t *testing.T) {
		t.Parallel()

		archive := mustTarGz(t, []tarEntry{
			{name: "review/link", typeflag: tar.TypeSymlink, linkname: "../target"},
		})

		err := ExtractArchive(bytes.NewReader(archive), t.TempDir())
		if err == nil {
			t.Fatal("ExtractArchive(symlink) error = nil, want failure")
		}
		if !errors.Is(err, ErrUnsupportedArchiveEntryType) {
			t.Fatalf("ExtractArchive(symlink) error = %v, want %v", err, ErrUnsupportedArchiveEntryType)
		}
	})

	t.Run("ShouldRejectArchivePathsThatEscapeTheRoot", func(t *testing.T) {
		t.Parallel()

		archive := mustTarGz(t, []tarEntry{
			{name: "../escape.txt", content: "nope"},
		})

		err := ExtractArchive(bytes.NewReader(archive), t.TempDir())
		if err == nil {
			t.Fatal("ExtractArchive(traversal) error = nil, want failure")
		}
		if !errors.Is(err, ErrArchiveEntryEscapesRoot) {
			t.Fatalf("ExtractArchive(traversal) error = %v, want %v", err, ErrArchiveEntryEscapesRoot)
		}
	})

	t.Run("ShouldRejectExtractionThroughASymlinkedParent", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("symlink semantics are platform-specific on windows")
		}

		root := t.TempDir()
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

		err := ExtractArchive(bytes.NewReader(archive), root)
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

	t.Run("ShouldRejectAnEmptyDestinationRoot", func(t *testing.T) {
		t.Parallel()

		err := ExtractArchive(bytes.NewReader(mustTarGz(t, []tarEntry{
			{name: "review/SKILL.md", content: "name: review\n"},
		})), "")
		if err == nil {
			t.Fatal("ExtractArchive(empty dest) error = nil, want failure")
		}
		if !errors.Is(err, ErrArchiveDestinationRequired) {
			t.Fatalf("ExtractArchive(empty dest) error = %v, want %v", err, ErrArchiveDestinationRequired)
		}
	})

	t.Run("ShouldRejectInvalidGzipStreams", func(t *testing.T) {
		t.Parallel()

		err := ExtractArchive(strings.NewReader("not-a-gzip-stream"), t.TempDir())
		if err == nil {
			t.Fatal("ExtractArchive(invalid gzip) error = nil, want failure")
		}
		if !errors.Is(err, gzip.ErrHeader) {
			t.Fatalf("ExtractArchive(invalid gzip) error = %v, want %v", err, gzip.ErrHeader)
		}
	})
}

func TestPathWithinRoot(t *testing.T) {
	t.Parallel()

	t.Run("ShouldResolvePathsInsideTheRootAndRejectEscapes", func(t *testing.T) {
		root := t.TempDir()

		target, err := PathWithinRoot(root, filepath.Join("review", "SKILL.md"))
		if err != nil {
			t.Fatalf("PathWithinRoot(valid) error = %v", err)
		}
		if !strings.HasPrefix(target, root+string(filepath.Separator)) {
			t.Fatalf("PathWithinRoot(valid) = %q, want path under %q", target, root)
		}

		if _, err := PathWithinRoot(root, filepath.Join("..", "escape")); !errors.Is(err, ErrPathOutsideRoot) {
			t.Fatalf("PathWithinRoot(escape) error = %v, want %v", err, ErrPathOutsideRoot)
		}
		if _, err := PathWithinRoot("   ", "review/SKILL.md"); !errors.Is(err, ErrPathRootRequired) {
			t.Fatalf("PathWithinRoot(blank root) error = %v, want %v", err, ErrPathRootRequired)
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
		{name: "ShouldNormalizeWindowsStyleSeparators", entry: "review\\SKILL.md", want: "review/SKILL.md"},
		{name: "ShouldRejectEmptyEntryPaths", entry: "", wantErr: ErrArchiveEntryPathRequired},
		{name: "ShouldRejectAbsoluteEntryPaths", entry: "/tmp/skill.md", wantErr: ErrArchiveEntryMustBeRelative},
		{name: "ShouldRejectEscapingEntryPaths", entry: "../escape.txt", wantErr: ErrArchiveEntryEscapesRoot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CleanArchiveEntryPath(tt.entry)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("CleanArchiveEntryPath(%q) error = %v", tt.entry, err)
				}
				if got != tt.want {
					t.Fatalf("CleanArchiveEntryPath(%q) = %q, want %q", tt.entry, got, tt.want)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CleanArchiveEntryPath(%q) error = %v, want %v", tt.entry, err, tt.wantErr)
			}
		})
	}
}

func TestCleanupArchiveFileJoinsRemoveFailure(t *testing.T) {
	t.Parallel()

	t.Run("ShouldJoinBaseAndRemoveErrors", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("directory permission failure semantics are platform-specific on windows")
		}

		root := t.TempDir()
		target := filepath.Join(root, "review", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("MkdirAll(parent) error = %v", err)
		}

		file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			t.Fatalf("OpenFile(target) error = %v", err)
		}
		if _, err := file.WriteString("partial"); err != nil {
			t.Fatalf("WriteString(target) error = %v", err)
		}

		if err := os.Chmod(filepath.Dir(target), 0o555); err != nil {
			t.Fatalf("Chmod(parent read-only) error = %v", err)
		}
		defer func() {
			if chmodErr := os.Chmod(filepath.Dir(target), 0o755); chmodErr != nil {
				t.Fatalf("Chmod(parent restore) error = %v", chmodErr)
			}
		}()

		baseErr := errors.New("write failed")
		err = cleanupArchiveFile(file, target, baseErr, false)
		if err == nil {
			t.Fatal("cleanupArchiveFile() error = nil, want joined failure")
		}
		if !errors.Is(err, baseErr) {
			t.Fatalf("cleanupArchiveFile() error = %v, want base error", err)
		}

		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("cleanupArchiveFile() error = %v, want remove path error", err)
		}
	})
}

func TestMoveInstalledDir(t *testing.T) {
	t.Parallel()

	t.Run("ShouldInstallIntoANewTarget", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")

		if err := MoveInstalledDir(source, target, false); err != nil {
			t.Fatalf("MoveInstalledDir(new target) error = %v", err)
		}

		data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target) error = %v", err)
		}
		if got := string(data); got != "new" {
			t.Fatalf("target content = %q, want new", got)
		}
	})

	t.Run("ShouldReplaceIntoANewTargetWhenRequested", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")

		if err := MoveInstalledDir(source, target, true); err != nil {
			t.Fatalf("MoveInstalledDir(replace new target) error = %v", err)
		}

		data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target) error = %v", err)
		}
		if got := string(data); got != "new" {
			t.Fatalf("target content = %q, want new", got)
		}
	})

	t.Run("ShouldReplaceAnExistingTarget", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(target, "SKILL.md"), "old")

		if err := MoveInstalledDir(source, target, true); err != nil {
			t.Fatalf("MoveInstalledDir(replace) error = %v", err)
		}

		data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target) error = %v", err)
		}
		if got := string(data); got != "new" {
			t.Fatalf("target content = %q, want new", got)
		}
	})

	t.Run("ShouldRejectExistingTargetsWhenReplaceIsDisabled", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(source, "SKILL.md"), "new")
		writeTestFile(t, filepath.Join(target, "SKILL.md"), "old")

		if err := MoveInstalledDir(source, target, false); err == nil {
			t.Fatal("MoveInstalledDir(no replace) error = nil, want failure")
		}
	})

	t.Run("ShouldRestoreTheBackupWhenReplacementFails", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		source := filepath.Join(parent, "missing-source")
		target := filepath.Join(parent, "target")
		writeTestFile(t, filepath.Join(target, "SKILL.md"), "old")

		if err := MoveInstalledDir(source, target, true); err == nil {
			t.Fatal("MoveInstalledDir(missing source) error = nil, want failure")
		}

		data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
		if err != nil {
			t.Fatalf("ReadFile(target after rollback) error = %v", err)
		}
		if got := string(data); got != "old" {
			t.Fatalf("target content after rollback = %q, want old", got)
		}
	})
}

func TestExtractArchivePreservesPermissionBits(t *testing.T) {
	t.Parallel()

	t.Run("ShouldPreservePermissionBitsFromTheArchive", func(t *testing.T) {
		root := t.TempDir()
		archive := mustTarGz(t, []tarEntry{
			{name: "review", typeflag: tar.TypeDir, mode: 0o750},
			{name: "review/scripts", typeflag: tar.TypeDir, mode: 0o755},
			{name: "review/scripts/run.sh", content: "#!/bin/sh\necho ok\n", mode: 0o755},
		})

		if err := ExtractArchive(bytes.NewReader(archive), root); err != nil {
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

	t.Run("ShouldStripSpecialPermissionBits", func(t *testing.T) {
		root := t.TempDir()
		archive := mustTarGz(t, []tarEntry{
			{name: "review", typeflag: tar.TypeDir, mode: 0o2750},
			{name: "review/run.sh", content: "#!/bin/sh\necho ok\n", mode: 0o4755},
		})

		if err := ExtractArchive(bytes.NewReader(archive), root); err != nil {
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

	t.Run("ShouldExposeTheDefaultExtractionLimits", func(t *testing.T) {
		if got, want := DefaultMaxDecompressedSize, int64(500*1024*1024); got != want {
			t.Fatalf("DefaultMaxDecompressedSize = %d, want %d", got, want)
		}
		if got, want := DefaultMaxFileCount, 10000; got != want {
			t.Fatalf("DefaultMaxFileCount = %d, want %d", got, want)
		}
	})
}

func TestCountingLimitWriter(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectANilTotalCounter", func(t *testing.T) {
		t.Parallel()

		writer := &countingLimitWriter{limit: 10}
		if _, err := writer.Write([]byte("abc")); err == nil {
			t.Fatal("countingLimitWriter.Write(nil total) error = nil, want failure")
		}
	})

	t.Run("ShouldCountBytesWithoutEnforcingALimitWhenUnlimited", func(t *testing.T) {
		t.Parallel()

		var total int64
		writer := &countingLimitWriter{total: &total}
		if n, err := writer.Write([]byte("abcdef")); err != nil {
			t.Fatalf("countingLimitWriter.Write(no limit) error = %v", err)
		} else if n != 6 {
			t.Fatalf("countingLimitWriter.Write(no limit) = %d, want 6", n)
		}
		if total != 6 {
			t.Fatalf("total = %d, want 6", total)
		}
	})
}

func TestRemoveInstalledDirBackup(t *testing.T) {
	t.Parallel()

	t.Run("ShouldIgnoreBackupCleanupFailuresAfterReplacementCommits", func(t *testing.T) {
		t.Parallel()

		calls := 0
		removeInstalledDirBackup(filepath.Join(t.TempDir(), "backup"), func(string) error {
			calls++
			return errors.New("cleanup failed")
		})
		if calls != 1 {
			t.Fatalf("removeInstalledDirBackup() calls = %d, want 1", calls)
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

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
