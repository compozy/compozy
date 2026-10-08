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
	rendered, archived, err := archiveRetiredMemorySettings(rendered, path)
	if err != nil {
		return overlay, FileError{Op: configMigrationOperation, Path: path, Err: err}
	}
	overlay, err = decode(rendered, path)
	if err != nil {
		return overlay, err
	}
	if !bytes.Equal(rendered, contents) {
		if publishErr := publishRetiredConfigArchive(directory, path, original, contents, rendered); publishErr != nil {
			if len(archived) == 0 {
				return configOverlay{}, publishErr
			}
			slog.Warn("config.retired_keys_archive_failed", "path", path, "reason", publishErr.Error())
			return overlay, nil
		}
		if len(archived) > 0 {
			slog.Warn("config.retired_keys_archived", "path", path, "keys", strings.Join(archived, ","))
		}
	}
	return overlay, nil
}

func publishRetiredConfigArchive(
	directory *fileutil.Directory,
	path string,
	original os.FileInfo,
	contents, rendered []byte,
) (err error) {
	writable, err := directory.ReopenForMutation(filepath.Dir(path))
	if err != nil {
		return FileError{Op: configMigrationOperation, Path: path, Err: err}
	}
	defer func() { err = errors.Join(err, writable.Close()) }()
	name := filepath.Base(path)
	current, info, err := writable.ReadRegularFile(name)
	if err != nil {
		return FileError{Op: configMigrationOperation, Path: path, Err: err}
	}
	if !os.SameFile(original, info) || !bytes.Equal(contents, current) {
		return FileError{
			Op: configMigrationOperation, Path: path, Err: errors.New("config changed during retirement migration"),
		}
	}
	return writePersistedFileInDirectory(writable, name, path, rendered, true)
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
