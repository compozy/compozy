package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"

	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func statusForHeartbeatWakeDecision(decision heartbeat.WakeDecision) int {
	switch decision.Result {
	case heartbeat.WakeResultSent:
		return http.StatusOK
	case heartbeat.WakeResultFailed:
		return http.StatusInternalServerError
	default:
		return http.StatusConflict
	}
}

func decodeAuthoredJSONBody(c *gin.Context, dst any, allowEmpty bool) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		if allowEmpty {
			return nil
		}
		return errors.New("request body is required")
	}
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) && allowEmpty {
			return nil
		}
		return fmt.Errorf("decode request body: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode request body: multiple JSON values are not allowed")
		}
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}

func authoredWorkspaceRefFromQuery(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return firstNonEmpty(c.Query("workspace_id"), c.Query("workspace"))
}

func pathAgentName(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Param("name"))
}

func authoredRouteAgentName(pathName string) (string, error) {
	path := strings.TrimSpace(pathName)
	if path != "" {
		return path, nil
	}
	return "", newAuthoredValidationError("agent_name is required")
}

func authoredAgentPath(workspace *workspacepkg.ResolvedWorkspace, agentName string) string {
	name := strings.TrimSpace(agentName)
	if workspace == nil {
		return ""
	}
	for _, agent := range workspace.Agents {
		if strings.TrimSpace(agent.Name) == name && strings.TrimSpace(agent.SourcePath) != "" {
			return strings.TrimSpace(agent.SourcePath)
		}
	}
	if root := strings.TrimSpace(workspace.RootDir); root != "" && name != "" {
		return filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, name, "AGENT.md")
	}
	return ""
}

func authoredSidecarPath(agentPath string, fileName string) string {
	trimmedAgentPath := strings.TrimSpace(agentPath)
	trimmedFileName := strings.TrimSpace(fileName)
	if trimmedAgentPath == "" {
		return trimmedFileName
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(trimmedAgentPath), trimmedFileName))
}

func newAuthoredValidationError(message string) error {
	return fmt.Errorf("%w: %s", errAuthoredContextValidation, strings.TrimSpace(message))
}
