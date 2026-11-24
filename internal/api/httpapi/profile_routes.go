package httpapi

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/gin-gonic/gin"
)

func registerProfileRoutes(api operatorRouteGroups, handlers *Handlers, allowMutations bool) {
	profiles := api.group("/profiles", remoteproto.ScopeSessions)
	profiles.GET("", handlers.ListProfiles)
	profiles.GET("/ops", handlers.ListProfileOperations)
	profiles.GET("/selection", handlers.GetProfileSelections)
	profiles.GET("/:name", handlers.GetProfile)
	profiles.GET("/:name/rename-plan", handlers.PrepareProfileRename)
	profiles.GET("/:name/archive-plan", handlers.PrepareProfileArchive)
	profiles.GET("/:name/delete-plan", handlers.PrepareProfileDelete)

	writes := api.group("/profiles", remoteproto.ScopeSettings)
	routes := []struct {
		method  string
		path    string
		handler gin.HandlerFunc
	}{
		{method: http.MethodPost, path: "", handler: handlers.CreateProfile},
		{method: http.MethodPut, path: "/selection", handler: handlers.PutProfileSelection},
		{method: http.MethodPost, path: "/ops/:op_id/retry", handler: handlers.RetryProfileOperation},
		{method: http.MethodPatch, path: "/:name", handler: handlers.UpdateProfile},
		{method: http.MethodPost, path: "/:name/rename", handler: handlers.RenameProfile},
		{method: http.MethodPost, path: "/:name/archive", handler: handlers.ArchiveProfile},
		{method: http.MethodPost, path: "/:name/unarchive", handler: handlers.UnarchiveProfile},
		{method: http.MethodDelete, path: "/:name", handler: handlers.DeleteProfile},
	}
	for _, route := range routes {
		if !allowMutations {
			writes.Handle(route.method, route.path, refuseGatewayProfileWrite)
		} else {
			writes.Handle(route.method, route.path, route.handler)
		}
	}
}

// The paired gateway retains its public refusal until the gateway is retired.
func refuseGatewayProfileWrite(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, contract.ProfileErrorPayload{Error: contract.ProfileError{
		Code:    "profile_remote_management_forbidden",
		Message: "profile management is available only on the local Compozy surface",
		Action:  "manage profiles from the local CLI or desktop app",
	}})
}
