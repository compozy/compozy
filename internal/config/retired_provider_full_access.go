package config

import (
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sync"

	burnttoml "github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/fileutil"
)

// Shim generation for the retired permissions.provider_full_access key (#1618).
// v0.6.0 accepted the key as a warning-only no-op and never rewrote the file, so v0.6.1 turned a
// documented no-op into a startup failure. The key has no behavior to preserve: loaders drop it,
// and persisted files are rewritten without it after a backup of the original.
// Remove this file, its call sites and their tests in v0.7.0.
const (
	retiredProviderFullAccessKey = "provider_full_access"
	// RetiredConfigKeysBackupSuffix names the sibling file that keeps the original bytes of a
	// config file rewritten by a retired-key boundary migration.
	RetiredConfigKeysBackupSuffix = ".retired-keys.bak"

	retiredProviderFullAccessWarning = "permissions.provider_full_access is retired and has no effect: " +
		"permissions.mode selects the provider mode; the key is ignored and is removed from config files " +
		"when they can be rewritten (support for ignoring it ends in v0.7.0)"
)

var warnRetiredProviderFullAccess = sync.OnceFunc(func() { slog.Warn(retiredProviderFullAccessWarning) })

func retiredProviderFullAccessPath() []string {
	return []string{toolSurfacePermissionsKey, retiredProviderFullAccessKey}
}

func isRetiredProviderFullAccessKey(key burnttoml.Key) bool {
	return len(key) == 2 && key[0] == toolSurfacePermissionsKey && key[1] == retiredProviderFullAccessKey
}

// withoutRetiredProviderFullAccessKeys keeps the retired no-op from failing a load as an unknown
// key in every config layer, including files the boundary migration could not rewrite.
func withoutRetiredProviderFullAccessKeys(keys []burnttoml.Key) []burnttoml.Key {
	filtered := make([]burnttoml.Key, 0, len(keys))
	for _, key := range keys {
		if isRetiredProviderFullAccessKey(key) {
			warnRetiredProviderFullAccess()
			continue
		}
		filtered = append(filtered, key)
	}
	return filtered
}

// removeRetiredProviderFullAccess drops the retired key's line from one persisted config document
// and leaves every other key, comment and table byte-for-byte. Layouts it cannot address safely
// (for example an inline table) are returned unchanged; the loader filter still ignores the key.
func removeRetiredProviderFullAccess(contents []byte, source string) ([]byte, bool) {
	document, err := parseOverlayDocument(contents)
	if err != nil {
		return contents, false
	}
	path := retiredProviderFullAccessPath()
	existing := document.findKeyValue(path)
	if existing == nil {
		return contents, false
	}
	start := min(max(rangeStart(existing.raw), 0), len(contents))
	end := min(max(rangeEnd(existing.raw), start), len(contents))
	for start > 0 && (contents[start-1] == ' ' || contents[start-1] == '\t') {
		start--
	}
	if start > 0 && contents[start-1] != '\n' {
		return contents, false
	}
	rest := end
	for rest < len(contents) && (contents[rest] == ' ' || contents[rest] == '\t' || contents[rest] == '\r') {
		rest++
	}
	switch {
	case rest == len(contents):
		end = rest
	case contents[rest] == '\n':
		end = rest + 1
	case contents[rest] != '#':
		return contents, false
	}
	// A trailing comment stays on its own line; only the key and its value are removed.
	rendered := replaceOffsets(contents, start, end, nil)
	remaining, err := parseOverlayDocument(rendered)
	if err != nil || remaining.findKeyValue(path) != nil {
		return contents, false
	}
	var values map[string]any
	if _, err := burnttoml.Decode(string(rendered), &values); err != nil {
		slog.Warn("config.retired_keys_removal_skipped", "path", source, "reason", err.Error())
		return contents, false
	}
	return rendered, true
}

// backupRetiredConfigOriginal keeps the first original of a file a retired-key migration rewrites.
// An existing backup already holds an older original and is never replaced.
func backupRetiredConfigOriginal(directory *fileutil.Directory, path string, contents []byte) (err error) {
	writable, err := directory.ReopenForMutation(filepath.Dir(path))
	if err != nil {
		return FileError{Op: configMigrationOperation, Path: path, Err: err}
	}
	defer func() { err = errors.Join(err, writable.Close()) }()
	name := filepath.Base(path) + RetiredConfigKeysBackupSuffix
	writeErr := writable.AtomicWriteFile(name, contents, privateFileMode, false)
	if writeErr == nil || errors.Is(writeErr, fs.ErrExist) || errors.Is(writeErr, fileutil.ErrPublishedNotDurable) {
		return nil
	}
	return FileError{Op: configMigrationOperation, Path: path + RetiredConfigKeysBackupSuffix, Err: writeErr}
}
