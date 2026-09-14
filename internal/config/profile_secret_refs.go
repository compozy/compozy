package config

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/vault"
	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

type profileRefEdit struct {
	start, end int
	content    []byte
}

// CountProfileSecretRefRewrites includes configured references in the rename preview.
func CountProfileSecretRefRewrites(root, oldName, newName string) (int, error) {
	return profileFileSecretRefs(root, oldName, newName, false)
}

// RewriteProfileSecretRefs preserves surrounding bytes and can resume after a partial rename.
func RewriteProfileSecretRefs(root, oldName, newName string) error {
	_, err := profileFileSecretRefs(root, oldName, newName, true)
	return err
}

func profileFileSecretRefs(root, oldName, newName string, apply bool) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == root && errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || (entry.Name() != ConfigName && entry.Name() != MCPJSONName) {
			return nil
		}
		changed, err := profileFileSecretRefEdits(path, oldName, newName, apply)
		count += changed
		return err
	})
	return count, err
}

func profileFileSecretRefEdits(path, oldName, newName string, apply bool) (count int, err error) {
	directory, name, err := fileutil.OpenParentDirectory(path)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	original, exists, err := readOptionalRegularFileFromDirectory(directory, name, path, "profile config")
	if err != nil || !exists {
		return 0, err
	}
	rename := func(ref string) string { return vault.RenameProfileSecretRef(ref, oldName, newName) }
	var edits []profileRefEdit
	if name == MCPJSONName {
		edits, err = profileJSONRefEdits(original, rename)
	} else {
		edits, err = profileTOMLRefEdits(original, rename)
	}
	if err != nil {
		return 0, fmt.Errorf("config: inspect profile references in %q: %w", path, err)
	}
	if !apply || len(edits) == 0 {
		return len(edits), nil
	}
	slices.SortFunc(edits, func(a, b profileRefEdit) int { return cmp.Compare(b.start, a.start) })
	rendered := original
	for _, edit := range edits {
		rendered = replaceOffsets(rendered, edit.start, edit.end, edit.content)
	}
	if err := requireOverlayContents(directory, name, path, original, true); err != nil {
		return 0, err
	}
	if err := writePersistedFileInDirectory(directory, name, path, rendered, true); err != nil {
		return 0, err
	}
	return len(edits), nil
}

func profileJSONRefEdits(source []byte, rename func(string) string) ([]profileRefEdit, error) {
	if !json.Valid(source) {
		return nil, errors.New("invalid MCP JSON document")
	}
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()
	var edits []profileRefEdit
	for {
		start := int(decoder.InputOffset())
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return edits, nil
		}
		if err != nil {
			return nil, err
		}
		value, ok := token.(string)
		if !ok {
			continue
		}
		end := int(decoder.InputOffset())
		if tail := bytes.TrimSpace(source[end:]); len(tail) > 0 && tail[0] == ':' {
			continue
		}
		updated := rename(value)
		if updated == value {
			continue
		}
		content, err := json.Marshal(updated)
		if err != nil {
			return nil, err
		}
		start += bytes.IndexByte(source[start:end], '"')
		edits = append(edits, profileRefEdit{start: start, end: end, content: content})
	}
}

func profileTOMLRefEdits(source []byte, rename func(string) string) ([]profileRefEdit, error) {
	parser := tomlast.Parser{}
	parser.Reset(source)
	var edits []profileRefEdit
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind != tomlast.KeyValue {
			continue
		}
		if err := profileTOMLValueEdits(node.Value(), rename, &edits); err != nil {
			return nil, err
		}
	}
	return edits, parser.Error()
}

func profileTOMLValueEdits(node *tomlast.Node, rename func(string) string, edits *[]profileRefEdit) error {
	if node.Kind == tomlast.String {
		value := string(node.Data)
		updated := rename(value)
		if updated == value {
			return nil
		}
		content, err := renderBareValue(updated)
		if err != nil {
			return err
		}
		*edits = append(*edits, profileRefEdit{
			start: rangeStart(node.Raw), end: rangeEnd(node.Raw), content: []byte(content),
		})
		return nil
	}
	if node.Kind == tomlast.KeyValue {
		return profileTOMLValueEdits(node.Value(), rename, edits)
	}
	children := node.Children()
	for children.Next() {
		if err := profileTOMLValueEdits(children.Node(), rename, edits); err != nil {
			return err
		}
	}
	return nil
}
