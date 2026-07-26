package heartbeat

import (
	"context"

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
	heartbeatPath string
	sourcePath    string
	config        aghconfig.HeartbeatConfig
}

type normalizedAuthoringTarget struct {
	workspaceID   string
	workspaceRoot string
	agentName     string
	agentPath     string
	config        aghconfig.HeartbeatConfig
}

func (s *ManagedHeartbeatAuthoringService) resolveTarget(
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
	if diagnostic := validateManagedHeartbeatPath(
		normalized.workspaceRoot,
		normalized.agentPath,
		"AGENT.md",
	); diagnostic != nil {
		return resolvedAuthoringTarget{}, authoringDiagnosticFromDiagnostic(diagnostic, ErrAuthoringPathRejected)
	}
	if err := ensureAuthoringAgent(normalized.agentPath, normalized.agentName); err != nil {
		return resolvedAuthoringTarget{}, err
	}
	heartbeatPath, sourcePath, err := resolveAuthoringHeartbeatPath(normalized.workspaceRoot, normalized.agentPath)
	if err != nil {
		return resolvedAuthoringTarget{}, err
	}
	return resolvedAuthoringTarget{
		workspaceID:   normalized.workspaceID,
		workspaceRoot: normalized.workspaceRoot,
		agentName:     normalized.agentName,
		agentPath:     normalized.agentPath,
		heartbeatPath: heartbeatPath,
		sourcePath:    sourcePath,
		config:        normalized.config,
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
		return normalizedAuthoringTarget{}, errors.New("heartbeat: authoring workspace id is required")
	}
	if workspaceRoot == "" {
		return normalizedAuthoringTarget{}, errors.New("heartbeat: authoring workspace root is required")
	}
	if !validAgentNameInput(agentName) {
		return normalizedAuthoringTarget{}, authoringDiagnosticError(
			diagnosticHeartbeatAgentNotFound,
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
				diagnosticHeartbeatAgentNotFound,
				safePathWithoutRoot(agentPath),
				"agent definition was not found",
				err,
				ErrAuthoringAgentNotFound,
			)
		}
		return authoringDiagnosticError(
			diagnosticHeartbeatAgentNotFound,
			safePathWithoutRoot(agentPath),
			"agent definition could not be loaded",
			err,
			ErrAuthoringAgentNotFound,
		)
	}
	return authoringDiagnosticError(
		diagnosticHeartbeatAgentNotFound,
		safePathWithoutRoot(agentPath),
		"agent definition does not match requested agent",
		nil,
		ErrAuthoringAgentNotFound,
	)
}

func resolveAuthoringHeartbeatPath(workspaceRoot string, agentPath string) (string, string, error) {
	heartbeatPath, err := heartbeatPathForAgent(agentPath)
	if err != nil {
		return "", "", authoringDiagnosticError(
			"heartbeat_invalid_source_path",
			safePathWithoutRoot(agentPath),
			"HEARTBEAT.md source path is required",
			err,
			ErrAuthoringPathRejected,
		)
	}
	sourcePath, diagnostic := safeSourcePath(heartbeatPath, workspaceRoot)
	if diagnostic != nil {
		return "", "", authoringDiagnosticFromDiagnostic(diagnostic, ErrAuthoringPathRejected)
	}
	if diagnostic := validateManagedHeartbeatPath(workspaceRoot, heartbeatPath, FileName); diagnostic != nil {
		diagnostic.SourcePath = firstNonEmpty(diagnostic.SourcePath, sourcePath)
		return "", "", authoringDiagnosticFromDiagnostic(diagnostic, ErrAuthoringPathRejected)
	}
	return heartbeatPath, sourcePath, nil
}

func (s *ManagedHeartbeatAuthoringService) currentHeartbeatForMutation(
	ctx context.Context,
	target resolvedAuthoringTarget,
) (ResolvedPolicy, error) {
	current, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err == nil {
		return current, nil
	}
	if !errors.Is(err, ErrInvalid) {
		return ResolvedPolicy{}, fmt.Errorf("heartbeat: resolve current HEARTBEAT.md: %w", err)
	}
	if current.Present {
		return current, nil
	}
	return current, authoringInvalidError(current.Diagnostics)
}

func (s *ManagedHeartbeatAuthoringService) verifyUnchangedHeartbeat(
	ctx context.Context,
	target resolvedAuthoringTarget,
	previous *ResolvedPolicy,
) error {
	latest, err := s.currentHeartbeatForMutation(ctx, target)
	if err != nil {
		return err
	}
	if previous == nil {
		return conflictError(target.sourcePath, "current HEARTBEAT.md state is required")
	}
	if latest.Present != previous.Present || latest.Digest != previous.Digest {
		return conflictError(target.sourcePath, "HEARTBEAT.md changed before managed mutation completed")
	}
	if latest.Digest == "" && previous.Present && latest.contentDigest != previous.contentDigest {
		return conflictError(target.sourcePath, "HEARTBEAT.md changed before managed mutation completed")
	}
	return nil
}
