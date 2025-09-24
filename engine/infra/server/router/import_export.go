package router

import (
	"net/http"
	"path/filepath"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resources/exporter"
	"github.com/compozy/compozy/engine/resources/importer"
	"github.com/gin-gonic/gin"
)

// ExportResource handles exporting a single resource type to its YAML directory.
func ExportResource(c *gin.Context, resourceType resources.ResourceType) {
	store, ok := GetResourceStore(c)
	if !ok {
		return
	}
	state := GetAppState(c)
	if state == nil {
		return
	}
	project := ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	root, ok := ProjectRootPath(state)
	if !ok {
		RespondWithServerError(c, ErrInternalCode, "project configuration not available", nil)
		return
	}
	out, err := exporter.ExportTypeToDir(
		c.Request.Context(),
		project,
		store,
		filepath.Clean(root),
		resourceType,
	)
	if err != nil {
		RespondWithServerError(c, ErrInternalCode, "export failed", err)
		return
	}
	payload := map[string]any{"written": safeCount(out.Written, resourceType)}
	RespondOK(c, "export completed", payload)
}

// ImportResource handles importing a single resource type from its YAML directory.
func ImportResource(c *gin.Context, resourceType resources.ResourceType) {
	store, ok := GetResourceStore(c)
	if !ok {
		return
	}
	state := GetAppState(c)
	if state == nil {
		return
	}
	project := ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	strategy, err := ParseImportStrategyParam(c.Query("strategy"))
	if err != nil {
		RespondWithError(
			c,
			http.StatusBadRequest,
			NewRequestError(http.StatusBadRequest, "invalid strategy", err),
		)
		return
	}
	root, ok := ProjectRootPath(state)
	if !ok {
		RespondWithServerError(c, ErrInternalCode, "project configuration not available", nil)
		return
	}
	out, err := importer.ImportTypeFromDir(
		c.Request.Context(),
		project,
		store,
		filepath.Clean(root),
		strategy,
		UpdatedBy(c),
		resourceType,
	)
	if err != nil {
		RespondWithServerError(c, ErrInternalCode, "import failed", err)
		return
	}
	payload := map[string]any{
		"imported":    safeCount(out.Imported, resourceType),
		"skipped":     safeCount(out.Skipped, resourceType),
		"overwritten": safeCount(out.Overwritten, resourceType),
		"strategy":    string(strategy),
	}
	RespondOK(c, "import completed", payload)
}

func safeCount(m map[resources.ResourceType]int, resourceType resources.ResourceType) int {
	if m == nil {
		return 0
	}
	return m[resourceType]
}
