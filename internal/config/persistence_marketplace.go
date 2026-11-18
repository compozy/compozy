package config

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/fileutil"
)

const retiredSkillRegistryKey = "registry"

const configMigrationOperation = "migrate"

func loadPersistedConfigOverlay(
	path string,
	decode func([]byte, string) (configOverlay, error),
) (configOverlay, error) {
	return loadPersistedConfigOverlayWithLockState(path, decode, false)
}

// loadPersistedConfigOverlayLocked is used by effective-write validation while the write mutex is held.
func loadPersistedConfigOverlayLocked(
	path string,
	decode func([]byte, string) (configOverlay, error),
) (configOverlay, error) {
	return loadPersistedConfigOverlayWithLockState(path, decode, true)
}

func loadPersistedConfigOverlayWithLockState(
	path string,
	decode func([]byte, string) (configOverlay, error),
	lockHeld bool,
) (overlay configOverlay, err error) {
	name := filepath.Base(path)
	directory, err := fileutil.OpenDirectory(filepath.Dir(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return overlay, nil
		}
		return overlay, FileError{Op: mergeReadKey, Path: path, Err: err}
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	contents, original, err := directory.ReadRegularFile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return overlay, nil
		}
		return overlay, FileError{Op: mergeReadKey, Path: path, Err: err}
	}
	rendered, err := archiveRetiredSkillMarketplace(contents, path)
	if err != nil {
		return overlay, FileError{Op: configMigrationOperation, Path: path, Err: err}
	}
	// required is the document after the migration whose publication must succeed. Every later
	// step is a retired-key shim whose publication is best effort.
	required := rendered
	rendered, migration, err := migrateRetiredConfigKeys(rendered, path)
	if err != nil {
		return overlay, FileError{Op: configMigrationOperation, Path: path, Err: err}
	}
	overlay, err = decode(rendered, path)
	if err != nil {
		return overlay, err
	}
	if bytes.Equal(rendered, contents) {
		return overlay, nil
	}
	publish := func(next []byte) error {
		if lockHeld {
			return publishRetiredConfigArchiveLocked(directory, path, original, contents, next, nil, nil)
		}
		return publishRetiredConfigArchive(directory, path, original, contents, next)
	}
	if !migration.changed() {
		err = publish(rendered)
	} else {
		err = publishRetiredConfigKeys(directory, path, contents, required, rendered, migration, publish)
	}
	if err != nil {
		return configOverlay{}, err
	}
	return overlay, nil
}

// retiredConfigMigration reports what the one-generation retired-key shims changed in one
// persisted config document. Restored by #1618 because v0.6.1 removed them while v0.6.0 homes
// still carried the values; remove this type, its helpers and their tests in v0.7.0.
type retiredConfigMigration struct {
	// archived names the retired tables and keys kept as inactive comments in the same file.
	archived []string
	// keyRemoved reports a removed permissions.provider_full_access line; the original bytes go
	// to the sibling backup because that removal leaves no archive comment.
	keyRemoved bool
}

func (m retiredConfigMigration) changed() bool {
	return len(m.archived) > 0 || m.keyRemoved
}

func migrateRetiredConfigKeys(contents []byte, path string) ([]byte, retiredConfigMigration, error) {
	rendered, archived, err := archiveRetiredMemorySettings(contents, path)
	if err != nil {
		return nil, retiredConfigMigration{}, err
	}
	rendered, migrated, err := migrateRetiredAlwaysOnTable(rendered, path)
	if err != nil {
		return nil, retiredConfigMigration{}, err
	}
	rendered, keyRemoved := removeRetiredProviderFullAccess(rendered, path)
	return rendered, retiredConfigMigration{archived: append(archived, migrated...), keyRemoved: keyRemoved}, nil
}

// publishRetiredConfigKeys writes the migrated document. A retired key never blocks a start: when
// the file cannot be rewritten the caller keeps the overlay decoded with the retired values
// inactive, the file stays untouched, and the next load retries. Only a pending skill-marketplace
// archive that is not accompanied by an archived table keeps its hard publication error.
func publishRetiredConfigKeys(
	directory *fileutil.Directory,
	path string,
	contents, required, rendered []byte,
	migration retiredConfigMigration,
	publish func([]byte) error,
) error {
	var publishErr error
	if migration.keyRemoved {
		publishErr = backupRetiredConfigOriginal(directory, path, contents)
	}
	if publishErr == nil {
		publishErr = publish(rendered)
	}
	if publishErr == nil {
		if len(migration.archived) > 0 {
			slog.Warn("config.retired_keys_archived", "path", path, "keys", strings.Join(migration.archived, ","))
		}
		if migration.keyRemoved {
			slog.Warn("config.retired_keys_removed",
				"path", path,
				"keys", strings.Join(retiredProviderFullAccessPath(), "."),
				"backup", path+RetiredConfigKeysBackupSuffix,
			)
		}
		return nil
	}
	slog.Warn("config.retired_keys_archive_failed", "path", path, "reason", publishErr.Error())
	if len(migration.archived) > 0 || bytes.Equal(required, contents) {
		return nil
	}
	return publish(required)
}

func publishRetiredConfigArchive(
	directory *fileutil.Directory,
	path string,
	original os.FileInfo,
	contents, rendered []byte,
) error {
	return publishRetiredConfigArchiveWithHook(directory, path, original, contents, rendered, nil, nil)
}

// publishRetiredConfigArchiveWithHook keeps the snapshot check adjacent to publication.
// The hook exercises a concurrent save at the real file-write boundary in the owning suite.
func publishRetiredConfigArchiveWithHook(
	directory *fileutil.Directory,
	path string,
	original os.FileInfo,
	contents, rendered []byte,
	beforeCompare, afterCompare func(),
) error {
	configOverlayWriteMu.Lock()
	defer configOverlayWriteMu.Unlock()
	return publishRetiredConfigArchiveLocked(directory, path, original, contents, rendered, beforeCompare, afterCompare)
}

// publishRetiredConfigArchiveLocked holds the caller's config write lease through the rename.
func publishRetiredConfigArchiveLocked(
	directory *fileutil.Directory,
	path string,
	original os.FileInfo,
	contents, rendered []byte,
	beforeCompare, afterCompare func(),
) (err error) {
	writable, err := directory.ReopenForMutation(filepath.Dir(path))
	if err != nil {
		return FileError{Op: configMigrationOperation, Path: path, Err: err}
	}
	defer func() { err = errors.Join(err, writable.Close()) }()
	name := filepath.Base(path)
	err = writable.AtomicWriteFileIf(name, rendered, privateFileMode, func() error {
		if beforeCompare != nil {
			beforeCompare()
		}
		current, info, err := writable.ReadRegularFile(name)
		if err != nil {
			return err
		}
		if !os.SameFile(original, info) || !bytes.Equal(contents, current) {
			return errors.New("config changed during retirement migration")
		}
		if afterCompare != nil {
			afterCompare()
		}
		return nil
	})
	if err != nil {
		return FileError{Op: configMigrationOperation, Path: path, Err: err}
	}
	return nil
}

func archiveRetiredSkillMarketplace(contents []byte, source string) ([]byte, error) {
	var values map[string]any
	if _, err := burnttoml.Decode(string(contents), &values); err != nil {
		return nil, err
	}
	skills, ok := values[SkillsDirName].(map[string]any)
	if !ok {
		return contents, nil
	}
	retiredValues := make(map[string]any)
	if retired, exists := skills[toolSurfaceMarketplaceKey]; exists {
		registry, ok := retired.(map[string]any)
		if !ok {
			return nil, errors.New("skills.marketplace must be a table")
		}
		for key := range registry {
			if key != retiredSkillRegistryKey && key != configBaseURLKey {
				return nil, fmt.Errorf("unknown config key skills.marketplace.%s", key)
			}
		}
		retiredValues[toolSurfaceMarketplaceKey] = registry
	}
	if allowed, exists := skills["allowed_marketplace_mcp"]; exists {
		entries, ok := allowed.([]any)
		if !ok {
			return nil, errors.New("skills.allowed_marketplace_mcp must be an array of strings")
		}
		for _, entry := range entries {
			if _, ok := entry.(string); !ok {
				return nil, errors.New("skills.allowed_marketplace_mcp must be an array of strings")
			}
		}
		retiredValues["allowed_marketplace_mcp"] = allowed
	}
	if len(retiredValues) == 0 {
		return contents, nil
	}
	var archive bytes.Buffer
	if err := burnttoml.NewEncoder(&archive).
		Encode(map[string]any{SkillsDirName: retiredValues}); err != nil {
		return nil, fmt.Errorf("archive retired skill acquisition settings: %w", err)
	}
	editor, err := newOverlayEditor(source, contents)
	if err != nil {
		return nil, err
	}
	if err := removeRetiredSkillSettings(editor, skills); err != nil {
		return nil, err
	}
	rendered, err := editor.Bytes()
	if err != nil {
		return nil, err
	}
	result := bytes.NewBuffer(rendered)
	result.WriteString("\n# Archived retired skill acquisition settings; these values are inactive.\n")
	for line := range strings.SplitSeq(strings.TrimRight(archive.String(), "\n"), "\n") {
		result.WriteString("# " + line + "\n")
	}
	return result.Bytes(), nil
}

func removeRetiredSkillSettings(editor *OverlayEditor, skills map[string]any) error {
	document, err := parseOverlayDocument(editor.content)
	if err != nil {
		return err
	}
	if document.findKeyValue([]string{SkillsDirName}) != nil {
		delete(skills, toolSurfaceMarketplaceKey)
		delete(skills, "allowed_marketplace_mcp")
		if err := editor.Delete([]string{SkillsDirName}); err != nil {
			return err
		}
		if len(skills) > 0 {
			if err := editor.SetTable([]string{SkillsDirName}, skills); err != nil {
				return err
			}
		}
	} else {
		for _, path := range [][]string{
			{SkillsDirName, "allowed_marketplace_mcp"},
			{SkillsDirName, toolSurfaceMarketplaceKey, retiredSkillRegistryKey},
			{SkillsDirName, toolSurfaceMarketplaceKey, configBaseURLKey},
		} {
			if err := editor.Delete(path); err != nil {
				return err
			}
		}
	}
	remaining, err := parseOverlayDocument(editor.content)
	if err != nil {
		return err
	}
	path := []string{SkillsDirName, toolSurfaceMarketplaceKey}
	if header := remaining.findTable(path); header != nil {
		// Removing the empty header must not eat comments before the next section.
		editor.content = replaceRange(editor.content, header.raw, nil)
	} else if err := editor.Delete(path); err != nil {
		return err
	}
	return nil
}
