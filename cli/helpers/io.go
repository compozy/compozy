package helpers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/fsnotify/fsnotify"
)

// OutputWriter handles different output formats
type OutputWriter struct {
	writer io.Writer
	format OutputFormat
	mode   models.Mode
}

// NewOutputWriter creates a new output writer
func NewOutputWriter(writer io.Writer, format OutputFormat, mode models.Mode) *OutputWriter {
	return &OutputWriter{
		writer: writer,
		format: format,
		mode:   mode,
	}
}

// WriteData writes data in the specified format
func (ow *OutputWriter) WriteData(data any) error {
	switch ow.format {
	case OutputFormatJSON:
		return ow.writeJSON(data)
	case OutputFormatTable:
		return ow.writeTable(data)
	case OutputFormatYAML:
		return ow.writeYAML(data)
	case OutputFormatTUI:
		return ow.writeTUI(data)
	default:
		return fmt.Errorf("unsupported output format: %s", ow.format)
	}
}

// writeJSON writes data as JSON
func (ow *OutputWriter) writeJSON(data any) error {
	encoder := json.NewEncoder(ow.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// writeTable writes data as a table (placeholder implementation)
func (ow *OutputWriter) writeTable(_ any) error {
	return fmt.Errorf("table output format not yet implemented")
}

// writeYAML writes data as YAML (placeholder implementation)
func (ow *OutputWriter) writeYAML(_ any) error {
	return fmt.Errorf("YAML output format not yet implemented")
}

// writeTUI writes data for TUI mode (placeholder implementation)
func (ow *OutputWriter) writeTUI(_ any) error {
	return fmt.Errorf("TUI output format not yet implemented")
}

// ReadInput reads input from various sources
func ReadInput(ctx context.Context, source string) ([]byte, error) {
	log := logger.FromContext(ctx)
	switch source {
	case "", "-":
		log.Debug("reading from stdin")
		return io.ReadAll(os.Stdin)
	default:
		log.Debug("reading from file", "file", source)
		return ReadFile(source)
	}
}

// ReadFile reads a file with enhanced error handling
func ReadFile(path string) ([]byte, error) {
	if path == "" {
		return nil, NewCliError("INVALID_PATH", "File path cannot be empty")
	}
	if !FileExists(path) {
		return nil, NewCliError("FILE_NOT_FOUND", fmt.Sprintf("File not found: %s", path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, NewCliError("FILE_READ_ERROR", fmt.Sprintf("Failed to read file: %s", path), err.Error())
	}
	return data, nil
}

// WriteFile writes data to a file with enhanced error handling
func WriteFile(path string, data []byte) error {
	if path == "" {
		return NewCliError("INVALID_PATH", "File path cannot be empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewCliError("DIRECTORY_CREATE_ERROR", fmt.Sprintf("Failed to create directory: %s", dir), err.Error())
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return NewCliError("FILE_WRITE_ERROR", fmt.Sprintf("Failed to write file: %s", path), err.Error())
	}
	return nil
}

// AppendToFile appends data to a file
func AppendToFile(path string, data []byte) (returnErr error) {
	if path == "" {
		return NewCliError("INVALID_PATH", "File path cannot be empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewCliError("DIRECTORY_CREATE_ERROR", fmt.Sprintf("Failed to create directory: %s", dir), err.Error())
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return NewCliError("FILE_OPEN_ERROR", fmt.Sprintf("Failed to open file: %s", path), err.Error())
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = NewCliError("FILE_CLOSE_ERROR", fmt.Sprintf("Failed to close file: %s", path), closeErr.Error())
		}
	}()
	if _, err := file.Write(data); err != nil {
		returnErr = NewCliError("FILE_WRITE_ERROR", fmt.Sprintf("Failed to write to file: %s", path), err.Error())
		return
	}
	return
}

// ReadLines reads a file line by line
func ReadLines(path string) ([]string, error) {
	if path == "" {
		return nil, NewCliError("INVALID_PATH", "File path cannot be empty")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, NewCliError("FILE_OPEN_ERROR", fmt.Sprintf("Failed to open file: %s", path), err.Error())
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, NewCliError("FILE_READ_ERROR", fmt.Sprintf("Failed to read file: %s", path), err.Error())
	}
	return lines, nil
}

// WriteLines writes lines to a file
func WriteLines(path string, lines []string) error {
	if path == "" {
		return NewCliError("INVALID_PATH", "File path cannot be empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewCliError("DIRECTORY_CREATE_ERROR", fmt.Sprintf("Failed to create directory: %s", dir), err.Error())
	}
	file, err := os.Create(path)
	if err != nil {
		return NewCliError("FILE_CREATE_ERROR", fmt.Sprintf("Failed to create file: %s", path), err.Error())
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return NewCliError("FILE_WRITE_ERROR", fmt.Sprintf("Failed to write to file: %s", path), err.Error())
		}
	}
	return writer.Flush()
}

// CreateTempFile creates a temporary file with given content
func CreateTempFile(pattern string, content []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", NewCliError("TEMP_FILE_ERROR", "Failed to create temporary file", err.Error())
	}
	defer tmpFile.Close()
	if _, err := tmpFile.Write(content); err != nil {
		_ = os.Remove(tmpFile.Name()) // Best effort cleanup, ignore error
		return "", NewCliError("TEMP_FILE_ERROR", "Failed to write to temporary file", err.Error())
	}
	return tmpFile.Name(), nil
}

// CleanupTempFile removes a temporary file
func CleanupTempFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return NewCliError("TEMP_FILE_ERROR", fmt.Sprintf("Failed to remove temporary file: %s", path), err.Error())
	}
	return nil
}

// WatchFile watches a file for changes
func WatchFile(ctx context.Context, path string, callback func([]byte) error) error {
	if err := validateWatchFilePath(path); err != nil {
		return err
	}
	if callback == nil {
		return NewCliError("INVALID_CALLBACK", "callback cannot be nil")
	}
	log := logger.FromContext(ctx)
	log.Info("watching file for changes", "file", path)
	if err := readAndNotify(ctx, path, callback); err != nil {
		return err
	}
	if err := watchFileWithFSNotify(ctx, path, callback, log); err != nil {
		if errors.Is(err, errFSNotifyUnavailable) || errors.Is(err, errFSNotifyClosed) {
			log.Warn("fsnotify unavailable, falling back to polling", "file", path, "error", err)
			return watchFileWithTicker(ctx, path, callback)
		}
		return err
	}
	return nil
}

const defaultWatchInterval time.Duration = time.Second

func resolveWatchInterval(ctx context.Context) time.Duration {
	cfg := config.FromContext(ctx)
	if cfg != nil && cfg.CLI.FileWatchInterval > 0 {
		return cfg.CLI.FileWatchInterval
	}
	return defaultWatchInterval
}

var errFileMissing = errors.New("file missing")
var errFSNotifyUnavailable = errors.New("fsnotify unavailable")
var errFSNotifyClosed = errors.New("fsnotify watcher closed unexpectedly")

func validateWatchFilePath(path string) error {
	if path != "" {
		return nil
	}
	return NewCliError("INVALID_PATH", "File path cannot be empty")
}

func readAndNotify(ctx context.Context, path string, callback func([]byte) error) error {
	data, err := ReadFile(path)
	if err != nil {
		return err
	}
	if err := callback(data); err != nil {
		return err
	}
	logger.FromContext(ctx).Debug("initial file read completed", "file", path)
	return nil
}

func fileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, errFileMissing
		}
		return time.Time{}, NewCliError("FILE_STAT_ERROR", fmt.Sprintf("Failed to stat file: %s", path), err.Error())
	}
	return info.ModTime(), nil
}

func processFileChange(
	ctx context.Context,
	path string,
	callback func([]byte) error,
	lastModTime time.Time,
) (bool, time.Time, error) {
	log := logger.FromContext(ctx)
	modTime, err := fileModTime(path)
	if err != nil {
		if errors.Is(err, errFileMissing) {
			log.Debug("file no longer exists", "file", path)
		}
		return false, lastModTime, err
	}
	if !modTime.After(lastModTime) {
		return false, lastModTime, nil
	}
	log.Debug("file changed", "file", path)
	data, err := ReadFile(path)
	if err != nil {
		log.Error("failed to read changed file", "file", path, "error", err)
		return false, lastModTime, err
	}
	if err := callback(data); err != nil {
		log.Error("callback failed for file change", "file", path, "error", err)
		return false, lastModTime, fmt.Errorf("watch callback failed: %w", err)
	}
	return true, modTime, nil
}

func watchFileWithTicker(ctx context.Context, path string, callback func([]byte) error) error {
	interval := resolveWatchInterval(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastModTime, err := fileModTime(path)
	if err != nil {
		if errors.Is(err, errFileMissing) {
			logger.FromContext(ctx).Debug("file not found; will wait for creation", "file", path)
			lastModTime = time.Time{}
		} else {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			updated, modTime, err := processFileChange(ctx, path, callback, lastModTime)
			if err != nil {
				if errors.Is(err, errFileMissing) {
					lastModTime = time.Time{}
					continue
				}
				return err
			}
			if updated {
				lastModTime = modTime
			}
		}
	}
}

func watchFileWithFSNotify(
	ctx context.Context,
	path string,
	callback func([]byte) error,
	log logger.Logger,
) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("%w: %v", errFSNotifyUnavailable, err)
	}
	defer watcher.Close()
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("%w: %v", errFSNotifyUnavailable, err)
	}
	lastModTime, err := fileModTime(path)
	if err != nil {
		if errors.Is(err, errFileMissing) {
			log.Debug("file not found; waiting for creation", "file", path)
			lastModTime = time.Time{}
		} else {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return errFSNotifyClosed
			}
			var err error
			lastModTime, err = handleFSNotifyEvent(ctx, path, callback, lastModTime, event)
			if err != nil {
				if errors.Is(err, errFileMissing) {
					lastModTime = time.Time{}
					continue
				}
				return err
			}
		case watchErr, ok := <-watcher.Errors:
			if err := handleFSNotifyError(log, path, watchErr, ok); err != nil {
				return err
			}
		}
	}
}

func eventMatchesFile(event fsnotify.Event, target string) bool {
	if event.Name == "" {
		return false
	}
	cleanEvent := filepath.Clean(event.Name)
	cleanTarget := filepath.Clean(target)
	if cleanEvent != cleanTarget {
		return false
	}
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
		return true
	}
	return false
}

func handleFSNotifyEvent(
	ctx context.Context,
	path string,
	callback func([]byte) error,
	lastModTime time.Time,
	event fsnotify.Event,
) (time.Time, error) {
	if !eventMatchesFile(event, path) {
		return lastModTime, nil
	}
	updated, modTime, err := processFileChange(ctx, path, callback, lastModTime)
	if err != nil {
		return lastModTime, err
	}
	if updated {
		return modTime, nil
	}
	return lastModTime, nil
}

func handleFSNotifyError(log logger.Logger, path string, watchErr error, ok bool) error {
	if !ok {
		return errFSNotifyClosed
	}
	if watchErr != nil {
		log.Error("file watcher error", "file", path, "error", watchErr)
	}
	return nil
}

// ReadJSONFile reads and parses a JSON file
func ReadJSONFile(path string, v any) error {
	data, err := ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return NewCliError("JSON_PARSE_ERROR", fmt.Sprintf("Failed to parse JSON file: %s", path), err.Error())
	}
	return nil
}

// WriteJSONFile writes data as JSON to a file
func WriteJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return NewCliError("JSON_MARSHAL_ERROR", fmt.Sprintf("Failed to marshal JSON for file: %s", path), err.Error())
	}
	return WriteFile(path, data)
}

// GetFileSize returns the size of a file
func GetFileSize(path string) (int64, error) {
	if path == "" {
		return 0, NewCliError("INVALID_PATH", "File path cannot be empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, NewCliError("FILE_STAT_ERROR", fmt.Sprintf("Failed to stat file: %s", path), err.Error())
	}
	return info.Size(), nil
}

// GetFileModTime returns the modification time of a file
func GetFileModTime(path string) (time.Time, error) {
	if path == "" {
		return time.Time{}, NewCliError("INVALID_PATH", "File path cannot be empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, NewCliError("FILE_STAT_ERROR", fmt.Sprintf("Failed to stat file: %s", path), err.Error())
	}
	return info.ModTime(), nil
}

// IsFileOlderThan checks if a file is older than a duration
func IsFileOlderThan(path string, duration time.Duration) (bool, error) {
	modTime, err := GetFileModTime(path)
	if err != nil {
		return false, err
	}
	return time.Since(modTime) > duration, nil
}

// FindFilesWithPattern finds files matching a pattern
func FindFilesWithPattern(dir, pattern string) ([]string, error) {
	if dir == "" {
		return nil, NewCliError("INVALID_PATH", "Directory path cannot be empty")
	}
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil, NewCliError(
			"GLOB_ERROR",
			fmt.Sprintf("Failed to find files with pattern: %s", pattern),
			err.Error(),
		)
	}
	var files []string
	for _, match := range matches {
		if FileExists(match) {
			files = append(files, match)
		}
	}
	return files, nil
}

// EnsureDirectory ensures a directory exists
func EnsureDirectory(path string) error {
	if path == "" {
		return NewCliError("INVALID_PATH", "Directory path cannot be empty")
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return NewCliError("DIRECTORY_CREATE_ERROR", fmt.Sprintf("Failed to create directory: %s", path), err.Error())
	}
	return nil
}

// RemoveDirectory removes a directory and all its contents
func RemoveDirectory(path string) error {
	if path == "" {
		return NewCliError("INVALID_PATH", "Directory path cannot be empty")
	}
	if err := os.RemoveAll(path); err != nil {
		return NewCliError("DIRECTORY_REMOVE_ERROR", fmt.Sprintf("Failed to remove directory: %s", path), err.Error())
	}
	return nil
}

// CopyFile copies a file from source to destination
func CopyFile(src, dst string) error {
	if src == "" || dst == "" {
		return NewCliError("INVALID_PATH", "Source and destination paths cannot be empty")
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return NewCliError("FILE_OPEN_ERROR", fmt.Sprintf("Failed to open source file: %s", src), err.Error())
	}
	defer srcFile.Close()
	dstDir := filepath.Dir(dst)
	if err := EnsureDirectory(dstDir); err != nil {
		return err
	}
	dstFile, err := os.Create(dst)
	if err != nil {
		return NewCliError("FILE_CREATE_ERROR", fmt.Sprintf("Failed to create destination file: %s", dst), err.Error())
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return NewCliError("FILE_COPY_ERROR", fmt.Sprintf("Failed to copy file from %s to %s", src, dst), err.Error())
	}
	return nil
}

// MoveFile moves a file from source to destination
func MoveFile(src, dst string) error {
	if src == "" || dst == "" {
		return NewCliError("INVALID_PATH", "Source and destination paths cannot be empty")
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := CopyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// GetRelativePath returns the relative path from base to target
func GetRelativePath(base, target string) (string, error) {
	if base == "" || target == "" {
		return "", NewCliError("INVALID_PATH", "Base and target paths cannot be empty")
	}
	relPath, err := filepath.Rel(base, target)
	if err != nil {
		return "", NewCliError(
			"PATH_ERROR",
			fmt.Sprintf("Failed to get relative path from %s to %s", base, target),
			err.Error(),
		)
	}
	return relPath, nil
}

// IsValidFilename checks if a filename is valid
func IsValidFilename(filename string) bool {
	if filename == "" {
		return false
	}
	if filename != filepath.Base(filename) {
		return false
	}
	invalidChars := []string{":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		if strings.Contains(filename, char) {
			return false
		}
	}
	reserved := []string{
		"CON",
		"PRN",
		"AUX",
		"NUL",
		"COM1",
		"COM2",
		"COM3",
		"COM4",
		"COM5",
		"COM6",
		"COM7",
		"COM8",
		"COM9",
		"LPT1",
		"LPT2",
		"LPT3",
		"LPT4",
		"LPT5",
		"LPT6",
		"LPT7",
		"LPT8",
		"LPT9",
	}
	upperFilename := strings.ToUpper(filename)
	return !slices.Contains(reserved, upperFilename)
}
