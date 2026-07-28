package storeschema

import (
	"fmt"
	"os"
	"path/filepath"
)

const sqlFileExtension = ".sql"

type schemaSource struct {
	path string
}

func (source schemaSource) files() ([]string, error) {
	info, err := os.Stat(source.path)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", source.path, err)
	}
	if !info.IsDir() {
		if filepath.Ext(source.path) != sqlFileExtension {
			return nil, fmt.Errorf("schema source %q is not a SQL file", source.path)
		}
		return []string{source.path}, nil
	}

	entries, err := os.ReadDir(source.path)
	if err != nil {
		return nil, fmt.Errorf("read schema directory %q: %w", source.path, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != sqlFileExtension {
			continue
		}
		files = append(files, filepath.Join(source.path, entry.Name()))
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("schema directory %q contains no SQL files", source.path)
	}
	return files, nil
}
