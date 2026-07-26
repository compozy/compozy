package soul

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

type resolvedAuthoringTarget struct {
	workspaceID   string
	workspaceRoot string
	agentName     string
	agentPath     string
	soulPath      string
	sourcePath    string
	config        aghconfig.SoulConfig
	configSource  string
}

type normalizedAuthoringTarget struct {
	workspaceID   string
	workspaceRoot string
	agentName     string
	agentPath     string
	config        aghconfig.SoulConfig
	configSource  string
}

type authoringMutationState struct {
	resolved     ResolvedSoul
	compareToken string
}

func (s *ManagedSoulAuthoringService) resolveTarget(
	ctx context.Context,
	target AuthoringTarget,
) (resolvedAuthoringTarget, error) {
	if err := checkContext(ctx); err != nil {
		return resolvedAuthoringTarget{}, err
	}
	normalized, err := normalizeAuthoringTarget(target)
	if err != nil {
		return resolvedAuthoringTarget{}, err
	}
	if diagnostic := validateManagedPath(
		normalized.workspaceRoot,
		normalized.agentPath,
		"AGENT.md",
	); diagnostic != nil {
		return resolvedAuthoringTarget{}, authoringDiagnosticFromDiagnostic(diagnostic, ErrAuthoringPathRejected)
	}
	if err := ensureAuthoringAgent(normalized.agentPath, normalized.agentName); err != nil {
		return resolvedAuthoringTarget{}, err
	}
	soulPath, sourcePath, err := resolveAuthoringSoulPath(normalized.workspaceRoot, normalized.agentPath)
	if err != nil {
		return resolvedAuthoringTarget{}, err
	}
	return resolvedAuthoringTarget{
		workspaceID:   normalized.workspaceID,
		workspaceRoot: normalized.workspaceRoot,
		agentName:     normalized.agentName,
		agentPath:     normalized.agentPath,
		soulPath:      soulPath,
		sourcePath:    sourcePath,
		config:        normalized.config,
		configSource:  normalized.configSource,
	}, nil
}

func normalizeAuthoringTarget(target AuthoringTarget) (normalizedAuthoringTarget, error) {
	config := target.Config
	if err := config.Validate(); err != nil {
		return normalizedAuthoringTarget{}, err
	}
	workspaceID := strings.TrimSpace(target.WorkspaceID)
	workspaceRoot := strings.TrimSpace(target.WorkspaceRoot)
	agentName := strings.TrimSpace(target.AgentName)
	if workspaceID == "" {
		return normalizedAuthoringTarget{}, errors.New("soul: authoring workspace id is required")
	}
	if workspaceRoot == "" {
		return normalizedAuthoringTarget{}, errors.New("soul: authoring workspace root is required")
	}
	if !validAgentNameInput(agentName) {
		return normalizedAuthoringTarget{}, authoringDiagnosticError(
			diagnosticAgentNotFound,
			FileName,
			"agent name is required",
			nil,
			ErrAuthoringAgentNotFound,
		)
	}

	agentPath := strings.TrimSpace(target.AgentPath)
	if agentPath == "" {
		agentPath = filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.AgentsDirName, agentName, "AGENT.md")
	}
	return normalizedAuthoringTarget{
		workspaceID:   workspaceID,
		workspaceRoot: workspaceRoot,
		agentName:     agentName,
		agentPath:     agentPath,
		config:        config,
		configSource:  strings.TrimSpace(target.ConfigSource),
	}, nil
}

func ensureAuthoringAgent(agentPath string, agentName string) error {
	agent, err := aghconfig.LoadAgentDefFile(agentPath)
	if err == nil && agent.Name == agentName {
		return nil
	}

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return authoringDiagnosticError(
				diagnosticAgentNotFound,
				safePathWithoutRoot(agentPath),
				"agent definition was not found",
				err,
				ErrAuthoringAgentNotFound,
			)
		}
		return authoringDiagnosticError(
			diagnosticAgentNotFound,
			safePathWithoutRoot(agentPath),
			"agent definition could not be loaded",
			err,
			ErrAuthoringAgentNotFound,
		)
	}
	return authoringDiagnosticError(
		diagnosticAgentNotFound,
		safePathWithoutRoot(agentPath),
		"agent definition does not match requested agent",
		nil,
		ErrAuthoringAgentNotFound,
	)
}

func resolveAuthoringSoulPath(workspaceRoot string, agentPath string) (string, string, error) {
	soulPath, err := soulPathForAgent(agentPath)
	if err != nil {
		return "", "", authoringDiagnosticError(
			"invalid_source_path",
			safePathWithoutRoot(agentPath),
			"SOUL.md source path is required",
			err,
			ErrAuthoringPathRejected,
		)
	}
	sourcePath, diagnostic := safeSourcePath(soulPath, workspaceRoot)
	if diagnostic != nil {
		return "", "", authoringDiagnosticFromDiagnostic(diagnostic, ErrAuthoringPathRejected)
	}
	if diagnostic := validateManagedPath(workspaceRoot, soulPath, FileName); diagnostic != nil {
		diagnostic.SourcePath = firstNonEmpty(diagnostic.SourcePath, sourcePath)
		return "", "", authoringDiagnosticFromDiagnostic(diagnostic, ErrAuthoringPathRejected)
	}
	return soulPath, sourcePath, nil
}

func (s *ManagedSoulAuthoringService) currentSoulForMutation(
	ctx context.Context,
	target resolvedAuthoringTarget,
) (authoringMutationState, error) {
	current, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err == nil {
		return authoringMutationState{
			resolved:     current,
			compareToken: authoringMutationCompareToken(&current, nil),
		}, nil
	}
	if !errors.Is(err, ErrInvalid) {
		return authoringMutationState{}, fmt.Errorf("soul: resolve current SOUL.md: %w", err)
	}
	if hasBlockingCurrentDiagnostic(current.Diagnostics) {
		return authoringMutationState{resolved: current}, authoringInvalidError(current.Diagnostics)
	}
	content, readErr := os.ReadFile(target.soulPath)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			absent, emptyErr := Empty(target.config, target.sourcePath)
			if emptyErr != nil {
				return authoringMutationState{}, emptyErr
			}
			return authoringMutationState{
				resolved:     absent,
				compareToken: authoringMutationCompareToken(&absent, nil),
			}, nil
		}
		return authoringMutationState{}, fmt.Errorf("soul: read current SOUL.md for mutation CAS: %w", readErr)
	}
	current, err = Parse(ctx, ParseRequest{
		SourcePath:    target.soulPath,
		WorkspaceRoot: target.workspaceRoot,
		Content:       content,
		Config:        target.config,
	})
	if err == nil {
		return authoringMutationState{
			resolved:     current,
			compareToken: authoringMutationCompareToken(&current, nil),
		}, nil
	}
	if !errors.Is(err, ErrInvalid) {
		return authoringMutationState{}, fmt.Errorf("soul: parse current SOUL.md for mutation CAS: %w", err)
	}
	return authoringMutationState{
		resolved:     current,
		compareToken: authoringMutationCompareToken(&current, content),
	}, nil
}

func (s *ManagedSoulAuthoringService) verifyUnchangedSoul(
	ctx context.Context,
	target resolvedAuthoringTarget,
	previous *authoringMutationState,
) error {
	latest, err := s.currentSoulForMutation(ctx, target)
	if err != nil {
		return err
	}
	if previous == nil {
		return conflictError(target.sourcePath, "current SOUL.md state is required")
	}
	if latest.resolved.Present != previous.resolved.Present || latest.compareToken != previous.compareToken {
		return conflictError(target.sourcePath, "SOUL.md changed before managed mutation completed")
	}
	return nil
}

func authoringMutationCompareToken(resolved *ResolvedSoul, invalidContent []byte) string {
	if resolved == nil {
		return "absent"
	}
	if !resolved.Present {
		return "absent"
	}
	if digest := strings.TrimSpace(resolved.Digest); digest != "" {
		return "digest:" + digest
	}
	sum := sha256.Sum256(invalidContent)
	return fmt.Sprintf("invalid:%x", sum[:])
}
