package helpers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/logger"
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
func (ow *OutputWriter) writeTable(data any) error {
	// This would need to be implemented based on specific data types
	// For now, fall back to JSON
	return ow.writeJSON(data)
}

// writeYAML writes data as YAML (placeholder implementation)
func (ow *OutputWriter) writeYAML(data any) error {
	// This would need YAML library integration
	// For now, fall back to JSON
	return ow.writeJSON(data)
}

// writeTUI writes data for TUI mode (placeholder implementation)
func (ow *OutputWriter) writeTUI(data any) error {
	// This would need TUI-specific formatting
	// For now, fall back to JSON
	return ow.writeJSON(data)
}

// ReadInput reads input from various sources
func ReadInput(ctx context.Context, source string) ([]byte, error) {
	log := logger.FromContext(ctx)

	switch source {
	case "", "-":
		// Read from stdin
		log.Debug("reading from stdin")
		return io.ReadAll(os.Stdin)
	default:
		// Read from file
		log.Debug("reading from file", "file", source)
		return ReadFile(source)
	}
}

// ReadFile reads a file with enhanced error handling
func ReadFile(path string) ([]byte, error) {
	if path == "" {
		return nil, NewCliError("INVALID_PATH", "File path cannot be empty")
	}

	// Check if file exists
	if !FileExists(path) {
		return nil, NewCliError("FILE_NOT_FOUND", fmt.Sprintf("File not found: %s", path))
	}

	// Read file
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

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewCliError("DIRECTORY_CREATE_ERROR", fmt.Sprintf("Failed to create directory: %s", dir), err.Error())
	}

	// Write file
	if err := os.WriteFile(path, data, 0600); err != nil {
		return NewCliError("FILE_WRITE_ERROR", fmt.Sprintf("Failed to write file: %s", path), err.Error())
	}

	return nil
}

// AppendToFile appends data to a file
func AppendToFile(path string, data []byte) error {
	if path == "" {
		return NewCliError("INVALID_PATH", "File path cannot be empty")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewCliError("DIRECTORY_CREATE_ERROR", fmt.Sprintf("Failed to create directory: %s", dir), err.Error())
	}

	// Open file for appending
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return NewCliError("FILE_OPEN_ERROR", fmt.Sprintf("Failed to open file: %s", path), err.Error())
	}
	defer file.Close()

	// Write data
	if _, err := file.Write(data); err != nil {
		return NewCliError("FILE_WRITE_ERROR", fmt.Sprintf("Failed to write to file: %s", path), err.Error())
	}

	return nil
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

	// Ensure directory exists
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
		os.Remove(tmpFile.Name()) // Clean up on error
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
	if path == "" {
		return NewCliError("INVALID_PATH", "File path cannot be empty")
	}

	log := logger.FromContext(ctx)
	log.Info("watching file for changes", "file", path)

	// Initial read
	data, err := ReadFile(path)
	if err != nil {
		return err
	}

	if err := callback(data); err != nil {
		return err
	}

	// Watch for changes (simplified implementation)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastModTime time.Time
	if info, err := os.Stat(path); err == nil {
		lastModTime = info.ModTime()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					log.Debug("file no longer exists", "file", path)
					continue
				}
				return NewCliError("FILE_STAT_ERROR", fmt.Sprintf("Failed to stat file: %s", path), err.Error())
			}

			if info.ModTime().After(lastModTime) {
				log.Debug("file changed", "file", path)
				lastModTime = info.ModTime()

				data, err := ReadFile(path)
				if err != nil {
					log.Error("failed to read changed file", "file", path, "error", err)
					continue
				}

				if err := callback(data); err != nil {
					log.Error("callback failed for file change", "file", path, "error", err)
					continue
				}
			}
		}
	}
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

	// Filter out directories
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

	// Ensure destination directory exists
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

	// Try rename first (fastest if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + remove
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

	// Check for invalid characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		if strings.Contains(filename, char) {
			return false
		}
	}

	// Check for reserved names (Windows)
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
	for _, reservedName := range reserved {
		if upperFilename == reservedName {
			return false
		}
	}

	return true
}
