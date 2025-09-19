package server

import (
	authuc "github.com/compozy/compozy/engine/auth/uc"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

// CreateAdminGroup centralizes creation of the admin route group.
// It always attaches RequireAdmin() which becomes a no-op when
// authentication is disabled in runtime config.
func CreateAdminGroup(
	apiBase *gin.RouterGroup,
	factory *authuc.Factory,
	cfg *config.Config,
) *gin.RouterGroup {
	manager := authmw.NewManager(factory, cfg)
	admin := apiBase.Group("/admin")
	admin.Use(manager.RequireAdmin())
	return admin
}
