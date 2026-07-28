package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DuplicateAgentDefinition clones one complete authored directory into a new target directory.
func DuplicateAgentDefinition(
	source AgentDef,
	targetDir string,
	draft AgentDefinitionDraft,
) (agent AgentDef, err error) {
	sourcePath := filepath.Clean(strings.TrimSpace(source.SourcePath))
	cleanedTarget := filepath.Clean(strings.TrimSpace(targetDir))
	if sourcePath == "." || !strings.EqualFold(filepath.Base(sourcePath), AgentDefinitionFileName) {
		return AgentDef{}, fmt.Errorf("config: invalid source agent definition path %q", source.SourcePath)
	}
	if cleanedTarget == "." {
		return AgentDef{}, fmt.Errorf("config: duplicate target directory is required")
	}
	if _, statErr := os.Lstat(cleanedTarget); statErr == nil {
		return AgentDef{}, errors.Join(
			ErrAgentDefinitionExists,
			fmt.Errorf("config: duplicate target directory already exists at %s", cleanedTarget),
		)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return AgentDef{}, fmt.Errorf("config: inspect duplicate target directory %q: %w", cleanedTarget, statErr)
	}

	created := false
	defer func() {
		if err == nil || !created {
			return
		}
		if cleanupErr := os.RemoveAll(cleanedTarget); cleanupErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("config: clean failed duplicate directory %q: %w", cleanedTarget, cleanupErr),
			)
		}
	}()

	targetPath := filepath.Join(cleanedTarget, AgentDefinitionFileName)
	agent, err = CreateAgentDefFile(targetPath, draft, false)
	if err != nil {
		return AgentDef{}, err
	}
	created = true
	if err := copyAgentDefinitionSiblings(filepath.Dir(sourcePath), cleanedTarget); err != nil {
		return AgentDef{}, err
	}
	return agent, nil
}

func copyAgentDefinitionSiblings(sourceDir string, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("config: walk source agent directory %q: %w", path, walkErr)
		}
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("config: resolve source agent relative path %q: %w", path, err)
		}
		if relative == "." || relative == AgentDefinitionFileName {
			return nil
		}
		target := filepath.Join(targetDir, relative)
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("config: inspect source agent entry %q: %w", path, err)
		}
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			return fmt.Errorf("config: source agent entry %q must not be a symlink", path)
		case entry.IsDir():
			if mkdirErr := os.MkdirAll(target, privateDirMode); mkdirErr != nil {
				return fmt.Errorf("config: create duplicate directory %q: %w", target, mkdirErr)
			}
			return nil
		case info.Mode().IsRegular():
			return copyAgentDefinitionFile(path, target, info.Mode())
		default:
			return fmt.Errorf("config: unsupported source agent entry %q", path)
		}
	})
}

func copyAgentDefinitionFile(sourcePath string, targetPath string, mode fs.FileMode) (err error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("config: open source agent file %q: %w", sourcePath, err)
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("config: close source agent file %q: %w", sourcePath, closeErr))
		}
	}()
	if err := os.MkdirAll(filepath.Dir(targetPath), privateDirMode); err != nil {
		return fmt.Errorf("config: create duplicate file directory %q: %w", filepath.Dir(targetPath), err)
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return fmt.Errorf("config: create duplicate agent file %q: %w", targetPath, err)
	}
	defer func() {
		if closeErr := target.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("config: close duplicate agent file %q: %w", targetPath, closeErr))
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("config: copy agent file %q to %q: %w", sourcePath, targetPath, err)
	}
	if err := target.Sync(); err != nil {
		return fmt.Errorf("config: sync duplicate agent file %q: %w", targetPath, err)
	}
	return nil
}
