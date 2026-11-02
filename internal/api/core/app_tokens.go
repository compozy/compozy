package core

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

type appTokenContextKey struct{}

// ContextWithAppToken attributes a local request without changing its local privileges.
func ContextWithAppToken(ctx context.Context, token gateway.AppToken) context.Context {
	return context.WithValue(ctx, appTokenContextKey{}, token)
}

// AppTokenFromContext returns the app credential attributed to this request.
func AppTokenFromContext(ctx context.Context) (gateway.AppToken, bool) {
	token, ok := ctx.Value(appTokenContextKey{}).(gateway.AppToken)
	return token, ok
}

// AppTokenManager owns operator-local app credentials.
type AppTokenManager interface {
	CreateAppToken(context.Context, string, []string) (gateway.AppTokenIssued, error)
	ListAppTokens(context.Context, gateway.AppTokenState) ([]gateway.AppToken, error)
	UpdateAppToken(context.Context, string, gateway.AppTokenPatch) (gateway.AppToken, error)
	RevokeAppToken(context.Context, string) (gateway.AppTokenRevokeResult, error)
}

func (h *BaseHandlers) CreateAppToken(c *gin.Context) {
	if !h.requireAppTokens(c) {
		return
	}
	var request contract.CreateAppTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		RespondError(c, http.StatusBadRequest, err, false)
		return
	}
	issued, err := h.AppTokens.CreateAppToken(c.Request.Context(), request.Name, request.Origins)
	if err != nil {
		RespondAppTokenError(c, err, false)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	h.logAppTokenMutation(c, "app_token.created", issued.Token, 0)
	c.JSON(
		http.StatusCreated,
		contract.AppTokenIssuedResponse{AppToken: AppTokenPayload(issued.Token), Token: issued.Secret},
	)
}

func (h *BaseHandlers) ListAppTokens(c *gin.Context) {
	if !h.requireAppTokens(c) {
		return
	}
	state := gateway.AppTokenState(c.DefaultQuery("state", "active"))
	switch state {
	case gateway.AppTokenStateActive, gateway.AppTokenStateRevoked, gateway.AppTokenStateAll:
	default:
		RespondAppTokenError(c, gateway.ErrAppTokenStateInvalid, false)
		return
	}
	tokens, err := h.AppTokens.ListAppTokens(c.Request.Context(), state)
	if err != nil {
		RespondAppTokenError(c, err, false)
		return
	}
	payload := contract.AppTokensResponse{AppTokens: make([]contract.AppTokenPayload, 0, len(tokens))}
	for _, token := range tokens {
		payload.AppTokens = append(payload.AppTokens, AppTokenPayload(token))
	}
	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) UpdateAppToken(c *gin.Context) {
	if !h.requireAppTokens(c) {
		return
	}
	var request contract.UpdateAppTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		RespondError(c, http.StatusBadRequest, err, false)
		return
	}
	token, err := h.AppTokens.UpdateAppToken(
		c.Request.Context(),
		c.Param("app_token_id"),
		gateway.AppTokenPatch{Name: request.Name, Origins: request.Origins},
	)
	if err != nil {
		RespondAppTokenError(c, err, false)
		return
	}
	h.logAppTokenMutation(c, "app_token.updated", token, 0)
	c.JSON(http.StatusOK, contract.AppTokenResponse{AppToken: AppTokenPayload(token)})
}

func (h *BaseHandlers) RevokeAppToken(c *gin.Context) {
	if !h.requireAppTokens(c) {
		return
	}
	result, err := h.AppTokens.RevokeAppToken(c.Request.Context(), c.Param("app_token_id"))
	if err != nil {
		RespondAppTokenError(c, err, false)
		return
	}
	h.logAppTokenMutation(c, "app_token.revoked", result.Token, result.Canceled)
	c.JSON(
		http.StatusOK,
		contract.AppTokenRevokeResponse{
			AppToken: AppTokenPayload(result.Token),
			Changed:  result.Changed,
			Canceled: result.Canceled,
		},
	)
}

func (h *BaseHandlers) logAppTokenMutation(c *gin.Context, event string, token gateway.AppToken, canceled int) {
	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(c.Request.Context(), event,
		"app_token_id", token.Session.ID, "name", token.Session.Name,
		"origin_count", len(token.Origins), "canceled_streams", canceled)
}

func (h *BaseHandlers) requireAppTokens(c *gin.Context) bool {
	if h != nil && h.AppTokens != nil {
		return true
	}
	RespondError(c, http.StatusServiceUnavailable, errors.New("app token service is not configured"), false)
	return false
}

func AppTokenPayload(token gateway.AppToken) contract.AppTokenPayload {
	payload := contract.AppTokenPayload{
		ID:        token.Session.ID,
		Name:      token.Session.Name,
		Origins:   append([]string{}, token.Origins...),
		CreatedAt: token.Session.CreatedAt,
	}
	if !token.Session.LastSeenAt.IsZero() {
		payload.LastUsedAt = new(token.Session.LastSeenAt)
	}
	if !token.Session.RevokedAt.IsZero() {
		payload.RevokedAt = new(token.Session.RevokedAt)
	}
	return payload
}

// requestOperatorOrigin is the creation origin of a request without agent credentials: an app
// token is a non-human principal with no grant ceiling; anything else is the operator.
func requestOperatorOrigin(c *gin.Context) workspaceaccess.Origin {
	if c != nil && c.Request != nil {
		if token, ok := AppTokenFromContext(c.Request.Context()); ok {
			return workspaceaccess.Origin{Kind: workspaceaccess.ActorApp, ID: token.Session.ID}
		}
	}
	return workspaceaccess.Origin{Kind: workspaceaccess.ActorHuman}
}
