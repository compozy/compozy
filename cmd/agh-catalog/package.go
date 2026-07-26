package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	extensionpkg "github.com/compozy/agh/internal/extension"
)

func packageCatalogExtension(sourceDirectory string, outputArchive string) (err error) {
	sourceRoot, outputPath, err := resolveCatalogPackagePaths(sourceDirectory, outputArchive)
	if err != nil {
		return err
	}
	if _, err := extensionpkg.LoadManifest(sourceRoot); err != nil {
		return fmt.Errorf("agh-catalog: validate extension package source: %w", err)
	}
	// #nosec G703 -- agh-catalog intentionally writes to an explicit local path selected by the operator.
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("agh-catalog: create artifact directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(outputPath), ".agh-catalog-*.tar.gz")
	if err != nil {
		return fmt.Errorf("agh-catalog: create temporary artifact: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	committed := false
	defer func() {
		if !closed {
			err = errors.Join(err, temporary.Close())
		}
		if !committed {
			err = errors.Join(err, removeCatalogTemporaryArtifact(temporaryPath))
		}
	}()

	gzipWriter, err := gzip.NewWriterLevel(temporary, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("agh-catalog: create gzip writer: %w", err)
	}
	gzipWriter.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	if err := writeCatalogPackageArchive(tarWriter, sourceRoot); err != nil {
		return errors.Join(err, tarWriter.Close(), gzipWriter.Close())
	}
	if err := errors.Join(tarWriter.Close(), gzipWriter.Close(), temporary.Close()); err != nil {
		closed = true
		return fmt.Errorf("agh-catalog: close extension artifact: %w", err)
	}
	closed = true
	// #nosec G703 -- temporaryPath is returned by os.CreateTemp in the operator-selected output directory.
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		return fmt.Errorf("agh-catalog: set extension artifact permissions: %w", err)
	}
	// #nosec G703 -- both paths belong to the explicit local publication operation requested by the operator.
	if err := os.Rename(temporaryPath, outputPath); err != nil {
		return fmt.Errorf("agh-catalog: publish extension artifact: %w", err)
	}
	committed = true
	return nil
}

func resolveCatalogPackagePaths(sourceDirectory string, outputArchive string) (string, string, error) {
	sourceRoot, err := filepath.Abs(strings.TrimSpace(sourceDirectory))
	if err != nil {
		return "", "", fmt.Errorf("agh-catalog: resolve package source: %w", err)
	}
	info, err := os.Stat(sourceRoot)
	if err != nil {
		return "", "", fmt.Errorf("agh-catalog: stat package source: %w", err)
	}
	if !info.IsDir() {
		return "", "", errors.New("agh-catalog: package source must be a directory")
	}
	outputPath, err := filepath.Abs(strings.TrimSpace(outputArchive))
	if err != nil {
		return "", "", fmt.Errorf("agh-catalog: resolve artifact output: %w", err)
	}
	if !strings.HasSuffix(strings.ToLower(outputPath), ".tar.gz") {
		return "", "", errors.New("agh-catalog: extension artifact must use a .tar.gz suffix")
	}
	if outputPath == sourceRoot || strings.HasPrefix(outputPath, sourceRoot+string(filepath.Separator)) {
		return "", "", errors.New("agh-catalog: extension artifact must be outside its package source")
	}
	return sourceRoot, outputPath, nil
}

func writeCatalogPackageArchive(writer *tar.Writer, sourceRoot string) error {
	rootName := filepath.Base(sourceRoot)
	// #nosec G703 -- packaging intentionally traverses the explicit local source directory selected by the operator.
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("agh-catalog: walk extension package: %w", walkErr)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("agh-catalog: stat extension package entry %q: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("agh-catalog: extension package symlink %q is not allowed", path)
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return fmt.Errorf("agh-catalog: resolve extension package entry %q: %w", path, err)
		}
		archiveName := rootName
		if relative != "." {
			archiveName = filepath.ToSlash(filepath.Join(rootName, relative))
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("agh-catalog: create archive header for %q: %w", path, err)
		}
		header.Name = archiveName
		header.Uid, header.Gid = 0, 0
		header.Uname, header.Gname = "", ""
		header.ModTime = time.Unix(0, 0).UTC()
		header.AccessTime, header.ChangeTime = time.Time{}, time.Time{}
		header.Format = tar.FormatUSTAR
		switch {
		case info.IsDir():
			header.Mode = 0o755
			header.Name += "/"
		case info.Mode().IsRegular():
			header.Mode = 0o644
		default:
			return fmt.Errorf("agh-catalog: unsupported extension package entry %q", path)
		}
		if err := writer.WriteHeader(header); err != nil {
			return fmt.Errorf("agh-catalog: write archive header for %q: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		return writeCatalogPackageFile(writer, path)
	})
}

func writeCatalogPackageFile(writer *tar.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("agh-catalog: open extension package file %q: %w", path, err)
	}
	_, copyErr := io.Copy(writer, file)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return fmt.Errorf("agh-catalog: archive extension package file %q: %w", path, err)
	}
	return nil
}

func removeCatalogTemporaryArtifact(path string) error {
	// #nosec G703 -- path is returned by os.CreateTemp and is removed only during local artifact cleanup.
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("agh-catalog: remove temporary artifact: %w", err)
}
