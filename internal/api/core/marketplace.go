package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	settingspkg "github.com/compozy/compozy/internal/settings"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

const (
	defaultMarketplaceLimit = 20
	maxMarketplaceLimit     = 100
)

var (
	ErrMarketplaceValidation  = errors.New("marketplace: validation error")
	ErrMarketplaceNotFound    = errors.New("marketplace: not found")
	ErrMarketplaceUnavailable = errors.New("marketplace: service unavailable")
)

type marketplaceReadScope struct {
	scope       settingspkg.ScopeKind
	workspaceID string
	profileName string
	actor       *taskpkg.ActorContext
}

// RefreshMarketplaceCatalog refreshes the extension catalog.
func (h *BaseHandlers) RefreshMarketplaceCatalog(c *gin.Context) {
	if c.Request.URL.Query().Has("kind") {
		h.respondMarketplaceError(c, marketplaceValidationf("kind is not supported by the Marketplace catalog"))
		return
	}
	if h == nil || h.MarketplaceCatalog == nil {
		h.respondMarketplaceError(c, errors.Join(ErrMarketplaceUnavailable, errors.New("catalog is not configured")))
		return
	}
	report, refreshErr := h.MarketplaceCatalog.Refresh(c.Request.Context())
	if refreshErr != nil && len(report.Outcomes) == 0 {
		h.respondMarketplaceError(c, refreshErr)
		return
	}
	items := make([]contract.MarketplaceRefreshSourcePayload, 0, len(report.Outcomes))
	for _, outcome := range report.Outcomes {
		items = append(items, contract.MarketplaceRefreshSourcePayload{
			Source: outcome.Source, Outcome: outcome.Outcome, EntryCount: outcome.EntryCount,
			Stale: outcome.Stale, ErrorClass: outcome.ErrorClass,
		})
	}
	c.JSON(http.StatusOK, contract.MarketplaceRefreshResponse{Sources: items})
}

func marketplaceLimit(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultMarketplaceLimit, nil
	}
	limit, err := strconv.Atoi(trimmed)
	if err != nil || limit <= 0 || limit > maxMarketplaceLimit {
		return 0, marketplaceValidationf("limit must be between 1 and %d", maxMarketplaceLimit)
	}
	return limit, nil
}

func marketplaceLimitValue(limit int) (int, error) {
	if limit == 0 {
		return defaultMarketplaceLimit, nil
	}
	if limit < 1 || limit > maxMarketplaceLimit {
		return 0, marketplaceValidationf("limit must be between 1 and %d", maxMarketplaceLimit)
	}
	return limit, nil
}

func marketplaceValidationf(format string, args ...any) error {
	return errors.Join(ErrMarketplaceValidation, fmt.Errorf(format, args...))
}

func normalizeCuratedMarketplaceError(err error) error {
	if errors.Is(err, marketplacepkg.ErrSourceUnavailable) {
		return errors.Join(ErrMarketplaceUnavailable, err)
	}
	return err
}

func (h *BaseHandlers) respondMarketplaceError(c *gin.Context, err error) {
	if errors.Is(err, extensionpkg.ErrExtensionSourceChanged) {
		h.respondExtensionError(c, http.StatusConflict, err)
		return
	}
	if errors.Is(err, ErrMarketplaceCursorStale) {
		c.JSON(
			http.StatusConflict,
			contract.MarketplaceCursorStalePayload{
				Error:   "Catalog changed; restart from the first page",
				Code:    "marketplace_cursor_stale",
				Restart: true,
			},
		)
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrMarketplaceValidation):
		status = http.StatusBadRequest
	case errors.Is(err, ErrMarketplaceNotFound), errors.Is(err, marketplacepkg.ErrEntryNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrMarketplaceUnavailable):
		status = http.StatusServiceUnavailable
	case errors.Is(err, taskpkg.ErrPermissionDenied):
		status = StatusForTaskError(err)
	}
	h.respondError(c, status, err)
}
