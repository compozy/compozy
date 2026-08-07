package httpapi

import (
	"net/http"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

func registerGatewayChallengeRoute(
	router gin.IRouter,
	surfaceSet SurfaceSet,
	tier gateway.Tier,
	resolver gateway.ChallengeResolver,
) {
	if router == nil || surfaceSet == SurfaceSetLocal || resolver == nil {
		return
	}
	router.GET(gateway.ChallengePathPrefix+":attempt", func(c *gin.Context) {
		nonce, ok := resolver.Resolve(tier, c.Request.URL.Path)
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(nonce))
	})
}
