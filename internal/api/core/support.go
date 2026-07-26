package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/support"
	"github.com/gin-gonic/gin"
)

const supportBundlesPathPrefix = "/api/support/bundles/"

var errSupportBundleServiceUnavailable = errors.New("support bundle service is not configured")
var errSupportBundleConsentRequired = errors.New("support bundle consent required")

// CreateSupportBundle starts an asynchronous daemon-owned support bundle operation.
func (h *BaseHandlers) CreateSupportBundle(c *gin.Context) {
	if h.SupportBundles == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSupportBundleServiceUnavailable)
		return
	}
	request := contract.CreateSupportBundleRequest{}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
	}
	if !request.Yes {
		h.respondError(c, http.StatusBadRequest, supportBundleConsentError())
		return
	}
	includeStatus := true
	if request.IncludeStatus != nil {
		includeStatus = *request.IncludeStatus
	}
	runCtx, cancel := detachedRequestContext(c.Request.Context())
	defer cancel()
	op, err := h.SupportBundles.Create(runCtx, support.CreateRequest{
		IncludeStatus: includeStatus,
	})
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusAccepted, SupportBundleOperationResponseFromOperation(op))
}

// GetSupportBundle returns the current status for one support bundle operation.
func (h *BaseHandlers) GetSupportBundle(c *gin.Context) {
	if h.SupportBundles == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSupportBundleServiceUnavailable)
		return
	}
	op, err := h.SupportBundles.Get(c.Request.Context(), c.Param("operation_id"))
	if err != nil {
		h.respondError(c, statusForSupportBundleError(err), err)
		return
	}
	c.JSON(http.StatusOK, SupportBundleOperationResponseFromOperation(op))
}

// DownloadSupportBundle streams a completed support bundle archive.
func (h *BaseHandlers) DownloadSupportBundle(c *gin.Context) {
	if h.SupportBundles == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSupportBundleServiceUnavailable)
		return
	}
	op, path, err := h.SupportBundles.DownloadPath(c.Request.Context(), c.Param("operation_id"))
	if err != nil {
		h.respondError(c, statusForSupportBundleError(err), err)
		return
	}
	c.Header("Content-Type", "application/gzip")
	c.FileAttachment(path, strings.TrimSpace(op.FileName))
}

func supportBundleConsentError() error {
	detail := "Support bundle creation requires explicit consent because bundles include redacted config, " +
		"log tails, provider metadata, event summaries, and status artifacts."
	item := diagnostics.NewItem(
		"support.bundle.consent_required",
		contract.CodeBundleConsentRequired,
		contract.CategoryDaemon,
		"Support bundle consent required",
		detail,
		contract.SeverityError,
		contract.FreshnessLive,
		diagnostics.WithSuggestedCommand("agh support bundle --yes"),
	)
	return diagnostics.NewStructuredError(item, errSupportBundleConsentRequired)
}

func detachedRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.Background(), func() {}
	}
	detached := context.WithoutCancel(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(detached, deadline)
	}
	return detached, func() {}
}

func statusForSupportBundleError(err error) int {
	switch {
	case errors.Is(err, support.ErrOperationNotFound):
		return http.StatusNotFound
	case errors.Is(err, support.ErrOperationNotReady):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func SupportBundleOperationResponseFromOperation(
	op support.Operation,
) contract.SupportBundleOperationResponse {
	return contract.SupportBundleOperationResponse{
		Operation: SupportBundleOperationPayloadFromOperation(op),
	}
}

func SupportBundleOperationPayloadFromOperation(op support.Operation) contract.SupportBundleOperationPayload {
	payload := contract.SupportBundleOperationPayload{
		OperationID:   strings.TrimSpace(op.OperationID),
		Status:        string(op.Status),
		StatusURL:     supportBundlesPathPrefix + strings.TrimSpace(op.OperationID),
		FileName:      strings.TrimSpace(op.FileName),
		SizeBytes:     op.SizeBytes,
		FailureReason: strings.TrimSpace(op.FailureReason),
		CreatedAt:     op.CreatedAt,
		UpdatedAt:     op.UpdatedAt,
		CompletedAt:   op.CompletedAt,
	}
	if op.Status == support.OperationCompleted {
		payload.DownloadURL = payload.StatusURL + "/download"
	}
	if op.Manifest != nil {
		payload.Manifest = SupportBundleManifestPayloadFromManifest(*op.Manifest)
	}
	return payload
}

func SupportBundleManifestPayloadFromManifest(
	manifest support.Manifest,
) *contract.SupportBundleManifestPayload {
	artifacts := make([]contract.SupportBundleManifestArtifactPayload, 0, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		artifacts = append(artifacts, contract.SupportBundleManifestArtifactPayload{
			Path:             strings.TrimSpace(artifact.Path),
			Included:         artifact.Included,
			OmittedReason:    strings.TrimSpace(artifact.OmittedReason),
			Bytes:            artifact.Bytes,
			Truncated:        artifact.Truncated,
			RedactionVersion: strings.TrimSpace(artifact.RedactionVersion),
		})
	}
	return &contract.SupportBundleManifestPayload{
		SchemaVersion:        strings.TrimSpace(manifest.SchemaVersion),
		OperationID:          strings.TrimSpace(manifest.OperationID),
		CreatedAt:            manifest.CreatedAt,
		BundleMaxBytes:       manifest.BundleMaxBytes,
		ArtifactMaxBytes:     manifest.ArtifactMaxBytes,
		LogTailMaxBytes:      manifest.LogTailMaxBytes,
		EventSummaryMaxBytes: manifest.EventSummaryMaxBytes,
		RedactionVersion:     strings.TrimSpace(manifest.RedactionVersion),
		Artifacts:            artifacts,
	}
}
