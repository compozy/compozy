package handlers

import (
	"github.com/gin-gonic/gin"
)

// Agent Handlers
func HandleListAgents(c *gin.Context)          { PlaceholderHandler(c) }
func HandleGetAgentDefinition(c *gin.Context)  { PlaceholderHandler(c) }
func HandleListAgentExecutions(c *gin.Context) { PlaceholderHandler(c) } // This is for a specific agent_id

// Agent Execution Handlers (Global)
func HandleListAllAgentExecutions(c *gin.Context)  { PlaceholderHandler(c) } // This is for all agents
func HandleGetAgentExecution(c *gin.Context)       { PlaceholderHandler(c) }
func HandleGetAgentExecutionStatus(c *gin.Context) { PlaceholderHandler(c) }
