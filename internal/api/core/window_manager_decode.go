package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

func decodeWindowManagerJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return normalizeStrictJSONDecodeError(err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return normalizeStrictJSONDecodeError(err)
		}
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func windowManagerWorkspace(c *gin.Context) windowmanager.WorkspaceID {
	return windowmanager.WorkspaceID(strings.TrimSpace(c.Param("workspace_id")))
}

func windowManagerClientID(c *gin.Context) (windowmanager.ClientID, error) {
	clientID := windowmanager.ClientID(strings.TrimSpace(c.Param("client_id")))
	if clientID == "" {
		return "", fmt.Errorf("client_id is required: %w", windowmanager.ErrInvalidCommand)
	}
	return clientID, nil
}

func windowManagerAfterRevision(c *gin.Context) (windowmanager.Revision, error) {
	value := strings.TrimSpace(c.Query("after_revision"))
	if value == "" {
		return 0, nil
	}
	revision, err := strconv.ParseUint(value, 10, 64)
	if err != nil || revision > uint64(contract.WindowManagerMaxSafeRevision) {
		return 0, fmt.Errorf("after_revision is invalid: %w", windowmanager.ErrInvalidCommand)
	}
	return windowmanager.Revision(revision), nil
}

func windowManagerStreamClientID(c *gin.Context) *windowmanager.ClientID {
	value := strings.TrimSpace(c.Query("client_id"))
	if value == "" {
		return nil
	}
	clientID := windowmanager.ClientID(value)
	return &clientID
}

func validateWindowManagerWorkspace(pathWorkspace, bodyWorkspace windowmanager.WorkspaceID) error {
	if bodyWorkspace == "" || bodyWorkspace != pathWorkspace {
		return fmt.Errorf("workspace_id must match the request path: %w", windowmanager.ErrInvalidCommand)
	}
	return nil
}
