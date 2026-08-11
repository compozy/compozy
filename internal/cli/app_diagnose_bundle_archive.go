package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	appDiagnosticBundleContentMaxBytes  int64 = 48 * 1024
	appDiagnosticBundleLogTailMaxBytes  int64 = 16 * 1024
	appDiagnosticBundleDesktopLogFile         = "desktop.log"
	appDiagnosticBundleBootstrapLogFile       = "desktop-bootstrap.jsonl"
)

type appDiagnosticBundleEntry struct {
	Name string
	Data []byte
}

func writeAppDiagnosticBundleArchive(file *os.File, entries []appDiagnosticBundleEntry) (err error) {
	writer := gzip.NewWriter(file)
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			closeErr = fmt.Errorf("app diagnose: close bundle gzip writer: %w", closeErr)
			if err == nil {
				err = closeErr
			} else {
				err = errors.Join(err, closeErr)
			}
		}
	}()
	archive := tar.NewWriter(writer)
	for _, entry := range entries {
		if err := writeAppDiagnosticBundleArchiveEntry(archive, entry); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("app diagnose: close bundle archive: %w", err)
	}
	return nil
}

func writeAppDiagnosticBundleArchiveEntry(archive *tar.Writer, entry appDiagnosticBundleEntry) error {
	header := &tar.Header{
		Name:     entry.Name,
		Mode:     0o600,
		Size:     int64(len(entry.Data)),
		Typeflag: tar.TypeReg,
		Format:   tar.FormatGNU,
	}
	if err := archive.WriteHeader(header); err != nil {
		return fmt.Errorf("app diagnose: write bundle entry header: %w", err)
	}
	if _, err := archive.Write(entry.Data); err != nil {
		return fmt.Errorf("app diagnose: write bundle entry: %w", err)
	}
	return nil
}

func readAppDiagnosticBundleManifest(path string) (manifest appDiagnosticBundleManifest, err error) {
	if err := requireAppDiagnosticBundleFile(path); err != nil {
		return appDiagnosticBundleManifest{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return appDiagnosticBundleManifest{}, fmt.Errorf("app diagnose: open app control bundle: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			closeErr = fmt.Errorf("app diagnose: close app control bundle: %w", closeErr)
			if err == nil {
				err = closeErr
			} else {
				err = errors.Join(err, closeErr)
			}
		}
	}()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return appDiagnosticBundleManifest{}, fmt.Errorf("app diagnose: open app control bundle gzip stream: %w", err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			closeErr = fmt.Errorf("app diagnose: close app control bundle gzip stream: %w", closeErr)
			if err == nil {
				err = closeErr
			} else {
				err = errors.Join(err, closeErr)
			}
		}
	}()
	return readAppDiagnosticBundleManifestArchive(tar.NewReader(gzipReader))
}

func readAppDiagnosticBundleManifestArchive(archive *tar.Reader) (appDiagnosticBundleManifest, error) {
	var manifestBytes []byte
	seen := make(map[string]struct{}, 3)
	var totalBytes int64
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return appDiagnosticBundleManifest{}, fmt.Errorf("app diagnose: read app control bundle entry: %w", err)
		}
		if err := validateAppDiagnosticBundleArchiveEntry(header, seen, totalBytes); err != nil {
			return appDiagnosticBundleManifest{}, err
		}
		totalBytes += header.Size
		seen[header.Name] = struct{}{}
		if header.Name == appDiagnosticBundleManifestFile {
			manifestBytes, err = readAppDiagnosticBundleArchiveEntry(archive, header.Size)
			if err != nil {
				return appDiagnosticBundleManifest{}, err
			}
			continue
		}
		if err := discardAppDiagnosticBundleArchiveEntry(archive, header.Size); err != nil {
			return appDiagnosticBundleManifest{}, err
		}
	}
	if len(manifestBytes) == 0 {
		return appDiagnosticBundleManifest{}, errors.New("app diagnose: app control bundle has no manifest")
	}
	return decodeAppDiagnosticBundleManifest(manifestBytes)
}

func validateAppDiagnosticBundleArchiveEntry(
	header *tar.Header,
	seen map[string]struct{},
	totalBytes int64,
) error {
	if header.Typeflag != tar.TypeReg || header.Size < 1 {
		return errors.New("app diagnose: app control bundle has an invalid entry")
	}
	if header.Name == appDiagnosticBundleManifestFile {
		if header.Size > appDiagnosticBundleContentMaxBytes {
			return errors.New("app diagnose: app control bundle manifest exceeds its size limit")
		}
	} else if header.Size > appDiagnosticBundleLogTailMaxBytes {
		return errors.New("app diagnose: app control bundle log entry exceeds its size limit")
	}
	if !isAppDiagnosticBundleArchiveEntryAllowed(header.Name) {
		return errors.New("app diagnose: app control bundle contains an unexpected entry")
	}
	if _, found := seen[header.Name]; found {
		return errors.New("app diagnose: app control bundle contains a duplicate entry")
	}
	if totalBytes+header.Size > appDiagnosticBundleContentMaxBytes {
		return errors.New("app diagnose: app control bundle exceeds its size limit")
	}
	return nil
}

func isAppDiagnosticBundleArchiveEntryAllowed(name string) bool {
	return name == appDiagnosticBundleManifestFile || name == appDiagnosticBundleDesktopLogFile ||
		name == appDiagnosticBundleBootstrapLogFile
}

func readAppDiagnosticBundleArchiveEntry(archive *tar.Reader, size int64) ([]byte, error) {
	contents, err := io.ReadAll(io.LimitReader(archive, size+1))
	if err != nil {
		return nil, fmt.Errorf("app diagnose: read app control bundle manifest data: %w", err)
	}
	if int64(len(contents)) != size {
		return nil, errors.New("app diagnose: app control bundle manifest is truncated")
	}
	return contents, nil
}

func discardAppDiagnosticBundleArchiveEntry(archive *tar.Reader, size int64) error {
	read, err := io.Copy(io.Discard, io.LimitReader(archive, size+1))
	if err != nil {
		return fmt.Errorf("app diagnose: read app control bundle log entry: %w", err)
	}
	if read != size {
		return errors.New("app diagnose: app control bundle log entry is truncated")
	}
	return nil
}

func decodeAppDiagnosticBundleManifest(manifestBytes []byte) (appDiagnosticBundleManifest, error) {
	var manifest appDiagnosticBundleManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return appDiagnosticBundleManifest{}, fmt.Errorf("app diagnose: decode app control bundle manifest: %w", err)
	}
	if manifest.SchemaVersion != appDiagnosticBundleManifestSchemaVersion ||
		manifest.Kind != appDiagnosticBundleKind || strings.TrimSpace(manifest.GeneratedAt) == "" {
		return appDiagnosticBundleManifest{}, errors.New("app diagnose: app control bundle manifest is incompatible")
	}
	reportBytes, err := json.Marshal(manifest.Report)
	if err != nil {
		return appDiagnosticBundleManifest{}, fmt.Errorf("app diagnose: encode app control bundle report: %w", err)
	}
	validatedReport, err := validateAppDiagnosticReport(reportBytes)
	if err != nil {
		return appDiagnosticBundleManifest{}, err
	}
	manifest.Report = validatedReport
	return manifest, nil
}

func requireAppDiagnosticBundleFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("app diagnose: inspect app control bundle: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("app diagnose: app control bundle path is invalid")
	}
	if info.Size() < 1 || info.Size() > appDiagnosticBundleMaxBytes {
		return errors.New("app diagnose: app control bundle exceeds its size limit")
	}
	return nil
}
