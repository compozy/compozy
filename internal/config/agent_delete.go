package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func openAgentDeleteRoot(agentsRoot string) (*os.Root, error) {
	rootInfo, err := os.Lstat(agentsRoot)
	if err != nil {
		return nil, fmt.Errorf("config: inspect agents root %q: %w", agentsRoot, err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("config: agents root %q must be a real directory", agentsRoot)
	}
	root, err := os.OpenRoot(agentsRoot)
	if err != nil {
		return nil, fmt.Errorf("config: open agents root %q: %w", agentsRoot, err)
	}
	return root, nil
}

func validateAgentDeleteTarget(root *os.Root, target agentDeleteTarget) error {
	agentInfo, err := root.Lstat(target.agentName)
	if errors.Is(err, os.ErrNotExist) {
		return agentDefinitionNotFoundError(target.definitionPath)
	}
	if err != nil {
		return fmt.Errorf("config: inspect agent directory %q: %w", target.agentDir, err)
	}
	if !agentInfo.IsDir() || agentInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("config: agent directory %q must be a real directory", target.agentDir)
	}
	definitionPath := filepath.Join(target.agentName, AgentDefinitionFileName)
	definitionInfo, err := root.Lstat(definitionPath)
	if errors.Is(err, os.ErrNotExist) {
		return agentDefinitionNotFoundError(target.definitionPath)
	}
	if err != nil {
		return fmt.Errorf("config: inspect agent definition %q: %w", target.definitionPath, err)
	}
	if !definitionInfo.Mode().IsRegular() {
		return fmt.Errorf(
			"config: agent definition %q must be a regular file",
			target.definitionPath,
		)
	}
	return nil
}

func agentDefinitionNotFoundError(sourcePath string) error {
	return errors.Join(
		ErrAgentDefinitionNotFound,
		fmt.Errorf("config: agent definition %q: %w", sourcePath, os.ErrNotExist),
	)
}
