package core

import (
	"errors"
	"fmt"

	"net/http"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/diagnostics"

	"github.com/gin-gonic/gin"
)

// ListBridgeTargets returns the daemon-owned target directory for one bridge instance.
func (h *BaseHandlers) ListBridgeTargets(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	result, err := bridges.ListBridgeTargets(c.Request.Context(), bridgepkg.BridgeTargetQuery{
		BridgeID: strings.TrimSpace(c.Param("id")),
		Query:    strings.TrimSpace(c.Query("q")),
		Limit:    limit,
	})
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, bridgeTargetsResponse(result))
}

// ResolveBridgeTarget resolves a friendly bridge target name without sending.
func (h *BaseHandlers) ResolveBridgeTarget(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}
	var req contract.BridgeResolveTargetRequest
	if err := decodeStrictBridgeJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode bridge target resolve request: %w", h.transportName(), err),
		)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%s: bridge target name is required", h.transportName()))
		return
	}
	result, err := bridges.ResolveBridgeTarget(
		c.Request.Context(),
		strings.TrimSpace(c.Param("id")),
		name,
	)
	if err != nil {
		if errors.Is(err, bridgepkg.ErrBridgeTargetAmbiguous) {
			diagnostic := bridgeTargetResolveDiagnostic(
				strings.TrimSpace(c.Param("id")),
				name,
				result,
				contract.CodeTargetAmbiguous,
			)
			c.JSON(http.StatusUnprocessableEntity, contract.BridgeResolveTargetResponse{
				Result:     result,
				Diagnostic: &diagnostic,
			})
			return
		}
		if errors.Is(err, bridgepkg.ErrBridgeTargetUnknown) {
			diagnostic := bridgeTargetResolveDiagnostic(
				strings.TrimSpace(c.Param("id")),
				name,
				result,
				contract.CodeTargetUnknown,
			)
			c.JSON(http.StatusNotFound, contract.BridgeResolveTargetResponse{
				Result:     result,
				Diagnostic: &diagnostic,
			})
			return
		}
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BridgeResolveTargetResponse{Result: result})
}

func bridgeTargetsResponse(result bridgepkg.BridgeTargetsResult) contract.BridgeTargetsResponse {
	var lastRefresh *time.Time
	if !result.LastSuccessfulRefreshAt.IsZero() {
		value := result.LastSuccessfulRefreshAt
		lastRefresh = &value
	}
	return contract.BridgeTargetsResponse{
		BridgeID:                result.BridgeID,
		Targets:                 result.Items,
		Total:                   result.Total,
		CacheStale:              result.CacheStale,
		GeneratedAt:             result.GeneratedAt,
		LastSuccessfulRefreshAt: lastRefresh,
	}
}

func bridgeTargetResolveDiagnostic(
	bridgeID string,
	query string,
	result bridgepkg.ResolveBridgeTargetResult,
	code string,
) contract.DiagnosticItem {
	title := "Bridge target resolution failed"
	message := fmt.Sprintf("Bridge target %q could not be resolved", query)
	if code == contract.CodeTargetAmbiguous {
		title = "Bridge target is ambiguous"
		message = fmt.Sprintf("Bridge target %q matched %d candidates", query, len(result.Candidates))
	}
	return diagnostics.NewItem(
		"bridge_target_resolve:"+strings.TrimSpace(bridgeID),
		code,
		contract.CategoryBridge,
		title,
		message,
		contract.SeverityWarn,
		contract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"bridge_id":  strings.TrimSpace(bridgeID),
			"query":      strings.TrimSpace(query),
			"step":       result.Step,
			"candidates": len(result.Candidates),
		}),
	)
}
