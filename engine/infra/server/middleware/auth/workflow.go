package auth

import (
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

// WorkflowAuthMiddleware creates workflow-specific authentication middleware
// that checks for workflow exceptions and bypasses authentication for specific workflow IDs
func WorkflowAuthMiddleware(authManager *Manager, cfg *config.Config) gin.HandlerFunc {
	if cfg != nil && !cfg.Server.Auth.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	if cfg == nil {
		return func(c *gin.Context) {
			authManager.RequireAuth()(c)
		}
	}
	exceptionsMap := make(map[string]struct{}, len(cfg.Server.Auth.WorkflowExceptions))
	for _, workflowID := range cfg.Server.Auth.WorkflowExceptions {
		exceptionsMap[workflowID] = struct{}{}
	}
	return func(c *gin.Context) {
		workflowID := c.Param("workflow_id")
		if _, exists := exceptionsMap[workflowID]; exists {
			c.Next()
			return
		}
		authManager.RequireAuth()(c)
		if c.IsAborted() {
			return
		}
	}
}
