package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// PlaceholderHandler is a generic handler for endpoints not yet implemented.
func PlaceholderHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":   "Endpoint not yet implemented",
		"path":      c.Request.URL.Path,
		"timestamp": time.Now(),
	})
}

// System Handlers
func HandleGetAPIInfo(c *gin.Context)       { PlaceholderHandler(c) }
func HandleGetHealth(c *gin.Context)        { PlaceholderHandler(c) }
func HandleGetMetrics(c *gin.Context)       { PlaceholderHandler(c) }
func HandleGetVersion(c *gin.Context)       { PlaceholderHandler(c) }
func HandleGetOpenAPISchema(c *gin.Context) { PlaceholderHandler(c) }
func HandleGetSwaggerUI(c *gin.Context)     { PlaceholderHandler(c) }
