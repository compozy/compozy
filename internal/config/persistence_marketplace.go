package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/fileutil"
)

func loadPersistedConfigOverlay(
	path string,
	decode func([]byte, string) (configOverlay, error),
) (overlay configOverlay, err error) {
	directory, name, err := fileutil.OpenParentDirectory(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return overlay, nil
		}
		return overlay, FileError{Op: mergeReadKey, Path: path, Err: err}
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	contents, _, err := directory.ReadRegularFile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return overlay, nil
		}
		return overlay, FileError{Op: mergeReadKey, Path: path, Err: err}
	}
	rendered, err := archiveRetiredSkillMarketplace(contents, path)
	if err != nil {
		return overlay, FileError{Op: "migrate", Path: path, Err: err}
	}
	overlay, err = decode(rendered, path)
	if err != nil {
		return overlay, err
	}
	if !bytes.Equal(rendered, contents) {
		// The archive and active config commit together through the same open parent.
		if err := writePersistedFileInDirectory(directory, name, path, rendered, true); err != nil {
			return configOverlay{}, err
		}
	}
	return overlay, nil
}

func archiveRetiredSkillMarketplace(contents []byte, source string) ([]byte, error) {
	var values map[string]any
	if _, err := burnttoml.Decode(string(contents), &values); err != nil {
		return nil, err
	}
	skills, ok := values["skills"].(map[string]any)
	if !ok {
		return contents, nil
	}
	retired, exists := skills["marketplace"]
	if !exists {
		return contents, nil
	}
	registry, ok := retired.(map[string]any)
	if !ok {
		return nil, errors.New("skills.marketplace must be a table")
	}
	for key := range registry {
		if key != "registry" && key != configBaseURLKey {
			return nil, fmt.Errorf("unknown config key skills.marketplace.%s", key)
		}
	}
	var archive bytes.Buffer
	if err := burnttoml.NewEncoder(&archive).
		Encode(map[string]any{"skills": map[string]any{"marketplace": registry}}); err != nil {
		return nil, fmt.Errorf("archive retired skill acquisition settings: %w", err)
	}
	editor, err := newOverlayEditor(source, contents)
	if err != nil {
		return nil, err
	}
	if err := removeSkillMarketplaceTable(editor, skills); err != nil {
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

func removeSkillMarketplaceTable(editor *OverlayEditor, skills map[string]any) error {
	document, err := parseOverlayDocument(editor.content)
	if err != nil {
		return err
	}
	if document.findKeyValue([]string{"skills"}) != nil {
		delete(skills, "marketplace")
		if err := editor.Delete([]string{"skills"}); err != nil {
			return err
		}
		if len(skills) > 0 {
			if err := editor.SetTable([]string{"skills"}, skills); err != nil {
				return err
			}
		}
	} else {
		for _, path := range [][]string{{"skills", "marketplace", "registry"}, {"skills", "marketplace", configBaseURLKey}} {
			if err := editor.Delete(path); err != nil {
				return err
			}
		}
	}
	remaining, err := parseOverlayDocument(editor.content)
	if err != nil {
		return err
	}
	path := []string{"skills", "marketplace"}
	if header := remaining.findTable(path); header != nil {
		// Removing the empty header must not eat comments before the next section.
		editor.content = replaceRange(editor.content, header.raw, nil)
	} else if err := editor.Delete(path); err != nil {
		return err
	}
	return nil
}
