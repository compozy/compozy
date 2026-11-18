package httpapi

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/gin-gonic/gin"
)

var errCrossSiteFetchNotAllowed = errors.New("cross-site file fetch not allowed")

// sameOriginFetchGuard refuses raw file bytes to a page another site or a sibling origin
// initiated, so it cannot embed or frame project files through the browser's credentials.
// Clients without Fetch Metadata (CLI, SDK) and app-token callers keep access.
func sameOriginFetchGuard(c *gin.Context) {
	switch strings.ToLower(strings.TrimSpace(c.GetHeader("Sec-Fetch-Site"))) {
	case "cross-site", "same-site":
		if _, app := core.AppTokenFromContext(c.Request.Context()); !app {
			abortForbiddenRequest(c, errCrossSiteFetchNotAllowed)
			return
		}
	}
	c.Next()
}
