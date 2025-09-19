package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// registerMetaRoutes registers admin-only meta endpoints under /admin
func registerMetaRoutes(admin *gin.RouterGroup) {
	admin.GET("/meta/:type/:id", adminGetMeta)
	admin.GET("/meta/changes", adminListMetaChanges)
}

func adminResourceTypeFromPath(c *gin.Context) (resources.ResourceType, bool) {
	t := strings.TrimSpace(c.Param("type"))
	switch t {
	case "agent":
		return resources.ResourceAgent, true
	case "tool":
		return resources.ResourceTool, true
	case "mcp":
		return resources.ResourceMCP, true
	case "schema":
		return resources.ResourceSchema, true
	case "model":
		return resources.ResourceModel, true
	case "workflow":
		return resources.ResourceWorkflow, true
	case "memory":
		return resources.ResourceMemory, true
	case "project":
		return resources.ResourceProject, true
	default:
		reqErr := router.NewRequestError(http.StatusBadRequest, "unknown resource type", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return "", false
	}
}

// adminGetMeta returns meta for a given resource key
// @Summary Get resource provenance metadata (admin)
// @Tags admin
// @Produce json
// @Param type path string true "Resource type"
// @Param id path string true "Resource ID"
// @Success 200 {object} router.Response{data=object}
// @Failure 401 {object} router.Response{error=router.ErrorInfo}
// @Failure 403 {object} router.Response{error=router.ErrorInfo}
// @Failure 404 {object} router.Response{error=router.ErrorInfo}
// @Router /admin/meta/{type}/{id} [get]
func adminGetMeta(c *gin.Context) {
	st := router.GetAppState(c)
	if st == nil {
		return
	}
	rs, ok := getResourceStoreFromState(st)
	if !ok {
		return
	}
	typ, ok := adminResourceTypeFromPath(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "id is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	project := c.Query("project")
	if strings.TrimSpace(project) == "" {
		project = st.ProjectConfig.Name
	}
	metaKey := resources.ResourceKey{
		Project: project,
		Type:    resources.ResourceMeta,
		ID:      project + ":" + string(typ) + ":" + id,
	}
	v, _, err := rs.Get(c.Request.Context(), metaKey)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusNotFound, "meta not found", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "meta", v)
}

// adminListMetaChanges lists recent provenance changes sorted by updated_at desc
// @Summary List recent provenance changes (admin)
// @Tags admin
// @Produce json
// @Param project query string false "Project override"
// @Param limit query int false "Max results (default 50)"
// @Success 200 {object} router.Response{data=object}
// @Failure 401 {object} router.Response{error=router.ErrorInfo}
// @Failure 403 {object} router.Response{error=router.ErrorInfo}
// @Router /admin/meta/changes [get]
func adminListMetaChanges(c *gin.Context) {
	st := router.GetAppState(c)
	if st == nil {
		return
	}
	rs, ok := getResourceStoreFromState(st)
	if !ok {
		return
	}
	project := c.Query("project")
	if strings.TrimSpace(project) == "" {
		project = st.ProjectConfig.Name
	}
	limit := parseLimit(c.Query("limit"))
	offset := parseOffset(c.Query("offset"))
	keys, total, err := rs.ListWithValuesPage(c.Request.Context(), project, resources.ResourceMeta, 0, 0)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to list meta items", err)
		return
	}
	items := buildMetaItemsFast(keys)
	sort.Slice(items, func(i, j int) bool {
		ti, errI := time.Parse(time.RFC3339, items[i].UpdatedAt)
		if errI != nil {
			return false
		}
		tj, errJ := time.Parse(time.RFC3339, items[j].UpdatedAt)
		if errJ != nil {
			return true
		}
		return ti.After(tj)
	})
	// apply pagination window
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = len(items)
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := items[offset:end]
	router.RespondOK(c, "changes", gin.H{"items": page, "total": total, "offset": offset, "limit": limit})
}

type metaItem struct{ Project, Type, ID, Source, UpdatedAt, UpdatedBy string }

func parseLimit(raw string) int {
	n := 50
	if q := strings.TrimSpace(raw); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	return n
}

func parseOffset(raw string) int {
	n := 0
	if q := strings.TrimSpace(raw); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v >= 0 {
			n = v
		}
	}
	return n
}

func buildMetaItemsFast(items []resources.StoredItem) []metaItem {
	out := make([]metaItem, 0, len(items))
	for _, it := range items {
		m, ok := it.Value.(map[string]any)
		if !ok {
			continue
		}
		src := ""
		if vv, ok := m["source"].(string); ok {
			src = vv
		}
		at := ""
		if vv, ok := m["updated_at"].(string); ok {
			at = vv
		}
		by := ""
		if vv, ok := m["updated_by"].(string); ok {
			by = vv
		}
		prj := it.Key.Project
		t := ""
		if vv, ok := m["type"].(string); ok {
			t = vv
		} else {
			t = string(it.Key.Type)
		}
		id := ""
		if vv, ok := m["id"].(string); ok {
			id = vv
		} else {
			parts := strings.SplitN(it.Key.ID, ":", 3)
			if len(parts) == 3 {
				id = parts[2]
			}
		}
		out = append(out, metaItem{Project: prj, Type: t, ID: id, Source: src, UpdatedAt: at, UpdatedBy: by})
	}
	return out
}
