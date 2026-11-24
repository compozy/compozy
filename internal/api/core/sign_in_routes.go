package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type SignInRequirement uint8

const (
	SignInAnonymous SignInRequirement = iota + 1
	SignInLocal
	SignInAccount
	SignInAgentContinuation
)

// SignInRoutes records the policy at registration using Gin's actual route
// matrix, including every concrete method registered by Any.
type SignInRoutes struct {
	engine *gin.Engine
	routes map[string]SignInRequirement
	base   *BaseHandlers
}

func NewSignInRoutes(router gin.IRouter, base *BaseHandlers) *SignInRoutes {
	engine, ok := router.(*gin.Engine)
	if !ok {
		panic("invariant: sign-in routes require their listener's root engine")
	}
	table := &SignInRoutes{engine: engine, routes: make(map[string]SignInRequirement), base: base}
	router.Use(table.serve)
	return table
}

func (t *SignInRoutes) RequireSignedIn(requirement SignInRequirement, register func()) {
	if t == nil {
		// Encrypted tunnels are authenticated by remotehost before routing.
		register()
		return
	}
	if requirement < SignInAnonymous || requirement > SignInAgentContinuation {
		panic("invariant: invalid route sign-in requirement")
	}
	before := t.engine.Routes()
	// Gin's route ordering is by method/tree, not by registration order.
	known := make(map[string]struct{}, len(before))
	for _, route := range before {
		known[route.Method+" "+route.Path] = struct{}{}
	}
	register()
	for _, route := range t.engine.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := known[key]; !exists && t.routes[key] == 0 {
			t.routes[key] = requirement
		}
	}
}

func RegisterAgentKernelRoutes(api gin.IRouter, handlers *BaseHandlers, signIn *SignInRoutes) {
	agent := api.Group("/agent")
	tasks := agent.Group("/tasks")
	signIn.RequireSignedIn(SignInLocal, func() {
		agent.POST("/spawn", handlers.AgentSpawn)
		tasks.POST("/claim-next", handlers.AgentTaskClaimNext)
	})
	signIn.RequireSignedIn(SignInAgentContinuation, func() {
		agent.GET("/me", handlers.AgentMe)
		agent.GET("/context", handlers.AgentContext)
		agent.GET("/soul", handlers.AgentSoul)
		agent.POST("/soul/validate", handlers.ValidateAgentSoul)
		agent.GET("/coordinator/config", handlers.AgentCoordinatorRole)
		agent.POST("/notify", handlers.AgentNotify)
		tasks.POST("/:run_id/start", handlers.AgentTaskStart)
		tasks.POST("/:run_id/heartbeat", handlers.AgentTaskHeartbeat)
		tasks.POST("/:run_id/complete", handlers.AgentTaskComplete)
		tasks.POST("/:run_id/fail", handlers.AgentTaskFail)
		tasks.POST("/:run_id/release", handlers.AgentTaskRelease)
	})
}

func RegisterRemoteHostRoutes(routes gin.IRouter, handlers *BaseHandlers, signIn *SignInRoutes) {
	signIn.RequireSignedIn(SignInAnonymous, func() {
		routes.GET("", handlers.RemoteStatus)
		routes.POST("/link", handlers.LinkRemote)
	})
	signIn.RequireSignedIn(SignInAccount, func() {
		routes.PATCH("", handlers.PatchRemoteStatus)
		routes.DELETE("/link", handlers.UnlinkRemote)
		routes.GET("/devices", handlers.ListRemoteDevices)
		routes.PATCH("/devices/:id", handlers.PatchRemoteDevice)
		routes.DELETE("/devices/:id", handlers.RevokeRemoteDevice)
	})
}

func (t *SignInRoutes) Validate() {
	for _, route := range t.engine.Routes() {
		key := route.Method + " " + route.Path
		if t.routes[key] == 0 {
			panic("invariant: route has no sign-in requirement: " + key)
		}
	}
}

func (t *SignInRoutes) serve(c *gin.Context) {
	if t.base == nil || t.base.Account == nil {
		return
	}
	requirement := t.routes[c.Request.Method+" "+c.FullPath()]
	if requirement == 0 || requirement == SignInAnonymous {
		return
	}
	if requirement == SignInAgentContinuation && t.base.runningAgentCaller(c) {
		return
	}
	service := t.base.Account
	lifetime := service.OperatorLifetime()
	err := service.AdmitNewWork()
	if err == nil && requirement == SignInAccount {
		current := service.Status()
		switch {
		case current.State != account.SignedIn && current.Gate != account.GateOff:
			err = account.ErrSignInRequired
		case current.Source == "token":
			err = account.ErrTokenLocalOnly
		}
	}
	if err != nil {
		RespondRemoteError(c, err)
		return
	}
	// Prompt delivery is request-bound even when the execution outlives the stream.
	if c.Request.Method == http.MethodGet || websocket.IsWebSocketUpgrade(c.Request) ||
		strings.HasSuffix(c.FullPath(), "/prompt") {
		t.bindOperatorStream(c, lifetime)
	}
}

func (h *BaseHandlers) runningAgentCaller(c *gin.Context) bool {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return false
	}
	_, err := h.resolveAgentCaller(c.Request.Context(), credentials, c.FullPath())
	return err == nil
}

const signInCloseCode = 4401

func (t *SignInRoutes) bindOperatorStream(c *gin.Context, lifetime context.Context) {
	if lifetime == nil {
		return
	}
	ctx, cancel := context.WithCancelCause(c.Request.Context())
	defer cancel(nil)
	stop := context.AfterFunc(lifetime, func() { cancel(context.Cause(lifetime)) })
	defer stop()
	c.Request = c.Request.WithContext(ctx)
	c.Next()
	if errors.Is(context.Cause(lifetime), account.ErrSignInRequired) &&
		strings.HasPrefix(c.Writer.Header().Get("Content-Type"), "text/event-stream") {
		t.base.writeSSEBestEffort(c.Writer, SSEMessage{Name: account.ErrSignInRequired.Code, Data: struct {
			Code int `json:"code"`
		}{Code: signInCloseCode}})
	}
}

func closeForSignIn(ctx context.Context, conn *websocket.Conn) error {
	if errors.Is(context.Cause(ctx), account.ErrSignInRequired) {
		return conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(signInCloseCode, account.ErrSignInRequired.Code),
			time.Now().Add(time.Second))
	}
	return nil
}
