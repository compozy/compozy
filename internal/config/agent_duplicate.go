package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
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

	contents, agent, err := RenderAgentDefinition(draft)
	if err != nil {
		return AgentDef{}, err
	}
	sourceDirectory, err := fileutil.OpenDirectory(filepath.Dir(sourcePath))
	if err != nil {
		return AgentDef{}, fmt.Errorf("config: open source agent directory %q: %w", filepath.Dir(sourcePath), err)
	}
	defer func() {
		if closeErr := sourceDirectory.Close(); closeErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("config: close source agent directory %q: %w", filepath.Dir(sourcePath), closeErr),
			)
		}
	}()

	targetParent, targetName, err := fileutil.OpenOrCreateParentDirectory(cleanedTarget, privateDirMode)
	if err != nil {
		return AgentDef{}, fmt.Errorf("config: open duplicate target parent %q: %w", cleanedTarget, err)
	}
	defer func() {
		if closeErr := targetParent.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("config: close duplicate target parent %q: %w", cleanedTarget, closeErr))
		}
	}()

	if err := duplicateAgentDefinitionInDirectories(
		sourceDirectory,
		targetParent,
		targetName,
		cleanedTarget,
		contents,
	); err != nil {
		return AgentDef{}, err
	}

	agent.SourcePath = filepath.Join(cleanedTarget, AgentDefinitionFileName)
	return agent, nil
}

func duplicateAgentDefinitionInDirectories(
	sourceDirectory *fileutil.Directory,
	targetParent *fileutil.Directory,
	targetName string,
	cleanedTarget string,
	contents []byte,
) (err error) {
	createdTarget, err := targetParent.CreateDirectory(targetName, privateDirMode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.Join(
				ErrAgentDefinitionExists,
				fmt.Errorf("config: duplicate target directory already exists at %s: %w", cleanedTarget, err),
			)
		}
		return fmt.Errorf("config: create duplicate target directory %q: %w", cleanedTarget, err)
	}
	targetClosed := false
	defer func() {
		if !targetClosed {
			if closeErr := createdTarget.Close(); closeErr != nil {
				err = errors.Join(
					err,
					fmt.Errorf("config: close duplicate target directory %q: %w", cleanedTarget, closeErr),
				)
			}
		}
		if err != nil {
			if cleanupErr := targetParent.RemoveAll(targetName); cleanupErr != nil {
				err = errors.Join(
					err,
					fmt.Errorf("config: clean failed duplicate directory %q: %w", cleanedTarget, cleanupErr),
				)
			}
		}
	}()

	if err := createdTarget.AtomicWriteFile(AgentDefinitionFileName, contents, privateFileMode, false); err != nil {
		return fmt.Errorf("config: write duplicate agent definition %q: %w", cleanedTarget, err)
	}
	if err := createdTarget.CopyContentsFrom(sourceDirectory, AgentDefinitionFileName); err != nil {
		if errors.Is(err, fileutil.ErrSymlink) {
			return fmt.Errorf("config: source agent entry must not be a symlink: %w", err)
		}
		return fmt.Errorf("config: copy source agent siblings into %q: %w", cleanedTarget, err)
	}
	if err := createdTarget.Close(); err != nil {
		return fmt.Errorf("config: close duplicate target directory %q: %w", cleanedTarget, err)
	}
	targetClosed = true
	if err := targetParent.Sync(); err != nil {
		return fmt.Errorf("config: sync duplicate target parent %q: %w", cleanedTarget, err)
	}
	return nil
}
