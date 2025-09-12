package auth

import (
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

// WorkflowAuthMiddleware creates workflow-specific authentication middleware
// that checks for workflow exceptions and bypasses authentication for specific workflow IDs
func WorkflowAuthMiddleware(authManager *Manager, cfg *config.Config) gin.HandlerFunc {
	// Pre-compute workflow exceptions map for O(1) lookup performance
	exceptionsMap := make(map[string]struct{}, len(cfg.Server.Auth.WorkflowExceptions))
	for _, workflowID := range cfg.Server.Auth.WorkflowExceptions {
		exceptionsMap[workflowID] = struct{}{}
	}
	return func(c *gin.Context) {
		workflowID := c.Param("workflow_id")
		// Check if this workflow is in the exception list
		if _, exists := exceptionsMap[workflowID]; exists {
			c.Next()
			return
		}
		// Apply authentication for this workflow
		authManager.RequireAuth()(c)
		// Ensure we don't continue if auth failed
		if c.IsAborted() {
			return
		}
	}
}
