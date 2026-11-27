package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remotehost"
)

func NewRemoteWebhookRoutes(
	manager core.AutomationManager,
	admit func(context.Context, string, ingress.SubjectRef) error,
	now func() time.Time,
) http.Handler {
	router := gin.New()
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Automation: manager, MaskInternalErrors: true})
	limiter := ingress.NewRateLimiter(defaultIngressRequestLimit, defaultIngressRequestWindow, now)
	check := func(ctx context.Context, trigger automation.Trigger) error {
		return admit(ctx, remotehost.WebhookIngressBase(ctx),
			ingress.SubjectRef{Kind: ingress.SubjectWebhookTrigger, ID: trigger.ID})
	}
	registerWebhookEndpoints(router.Group("/api"), ingressRateLimit(limiter),
		handlers.RemoteWebhookHandler(automation.AutomationScopeGlobal, check),
		handlers.RemoteWebhookHandler(automation.AutomationScopeWorkspace, check))
	return router
}

func registerWebhookEndpoints(api gin.IRouter, limiter, global, workspace gin.HandlerFunc) {
	webhooks := api.Group("/webhooks", limiter)
	webhooks.POST("/global/:endpoint", global)
	webhooks.POST("/workspaces/:workspace_id/:endpoint", workspace)
}
