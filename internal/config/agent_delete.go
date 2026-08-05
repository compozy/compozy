package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

var (
	// ErrAgentDefinitionNotFound marks an authored definition that no longer exists.
	ErrAgentDefinitionNotFound = errors.New("config: agent definition not found")
)

type agentDeleteTarget struct {
	agentsRoot     string
	definitionPath string
	agentDir       string
	agentName      string
}

// DeleteAgentDefinition removes one authored directory beneath an authorized agents root.
func DeleteAgentDefinition(agentsRoot string, sourcePath string) (retErr error) {
	target, err := resolveAgentDeleteTarget(agentsRoot, sourcePath)
	if err != nil {
		return err
	}
	root, err := openAgentDeleteRoot(target.agentsRoot)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("config: close agents root %q: %w", target.agentsRoot, closeErr),
			)
		}
	}()

	return deleteAgentDefinitionFromRoot(root, target)
}

func deleteAgentDefinitionFromRoot(root *fileutil.Directory, target agentDeleteTarget) error {
	if err := validateAgentDeleteTarget(root, target); err != nil {
		return err
	}
	if err := root.RemoveAll(target.agentName); err != nil {
		return fmt.Errorf("config: remove agent definition directory %q: %w", target.agentDir, err)
	}
	return nil
}

func resolveAgentDeleteTarget(agentsRoot string, sourcePath string) (agentDeleteTarget, error) {
	cleanRoot := filepath.Clean(strings.TrimSpace(agentsRoot))
	cleaned := filepath.Clean(strings.TrimSpace(sourcePath))
	if cleanRoot == "." || cleaned == "." || !filepath.IsAbs(cleanRoot) ||
		!filepath.IsAbs(cleaned) || filepath.Base(cleanRoot) != AgentsDirName ||
		!strings.EqualFold(filepath.Base(cleaned), AgentDefinitionFileName) {
		return agentDeleteTarget{}, fmt.Errorf(
			"config: invalid agent definition source path %q",
			sourcePath,
		)
	}
	agentDir := filepath.Dir(cleaned)
	agentName := filepath.Base(agentDir)
	expectedPath := filepath.Join(cleanRoot, agentName, AgentDefinitionFileName)
	if agentName == "." || agentName == AgentsDirName || cleaned != expectedPath {
		return agentDeleteTarget{}, fmt.Errorf(
			"config: invalid agent definition delete target %q",
			sourcePath,
		)
	}
	return agentDeleteTarget{
		agentsRoot:     cleanRoot,
		definitionPath: cleaned,
		agentDir:       agentDir,
		agentName:      agentName,
	}, nil
}

func openAgentDeleteRoot(agentsRoot string) (*fileutil.Directory, error) {
	root, err := fileutil.OpenDirectory(agentsRoot)
	if err != nil {
		if errors.Is(err, fileutil.ErrSymlink) || errors.Is(err, fileutil.ErrNotDirectory) {
			return nil, fmt.Errorf("config: agents root %q must be a real directory: %w", agentsRoot, err)
		}
		return nil, fmt.Errorf("config: inspect agents root %q: %w", agentsRoot, err)
	}
	return root, nil
}

func validateAgentDeleteTarget(root *fileutil.Directory, target agentDeleteTarget) (err error) {
	agentDirectory, err := root.OpenDirectory(target.agentName)
	if errors.Is(err, os.ErrNotExist) {
		return agentDefinitionNotFoundError(target.definitionPath)
	}
	if err != nil {
		if errors.Is(err, fileutil.ErrSymlink) || errors.Is(err, fileutil.ErrNotDirectory) {
			return fmt.Errorf("config: agent directory %q must be a real directory: %w", target.agentDir, err)
		}
		return fmt.Errorf("config: inspect agent directory %q: %w", target.agentDir, err)
	}
	defer func() {
		if closeErr := agentDirectory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("config: close agent directory %q: %w", target.agentDir, closeErr))
		}
	}()

	definition, err := agentDirectory.OpenRegularFile(AgentDefinitionFileName)
	if errors.Is(err, os.ErrNotExist) {
		return agentDefinitionNotFoundError(target.definitionPath)
	}
	if err != nil {
		if errors.Is(err, fileutil.ErrDirectory) || errors.Is(err, fileutil.ErrNotRegular) ||
			errors.Is(err, fileutil.ErrSymlink) {
			return fmt.Errorf("config: agent definition %q must be a regular file: %w", target.definitionPath, err)
		}
		return fmt.Errorf("config: inspect agent definition %q: %w", target.definitionPath, err)
	}
	if closeErr := definition.Close(); closeErr != nil {
		return fmt.Errorf("config: close agent definition %q: %w", target.definitionPath, closeErr)
	}
	return nil
}

func agentDefinitionNotFoundError(sourcePath string) error {
	return errors.Join(
		ErrAgentDefinitionNotFound,
		fmt.Errorf("config: agent definition %q: %w", sourcePath, os.ErrNotExist),
	)
}
