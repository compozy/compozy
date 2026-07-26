package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *whatsappProvider) newGraphAPI(cfg resolvedInstanceConfig) whatsappAPI {
	return &whatsappGraphClient{
		baseURL:     cfg.apiBaseURL,
		apiVersion:  cfg.apiVersion,
		accessToken: cfg.accessToken,
		reportRetry: func(ctx context.Context, attempt bridgesdk.RetryAttempt) error {
			return p.reportRetry(ctx, cfg.instanceID, attempt)
		},
		reportRetryFailure: func(err error) {
			p.markers.ReportError("report WhatsApp retry state", err)
		},
		reportResponseCleanup: func(err error) {
			p.markers.ReportError("clean up WhatsApp Graph API response", err)
		},
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *whatsappProvider) reportRetry(
	ctx context.Context,
	instanceID string,
	attempt bridgesdk.RetryAttempt,
) error {
	session := p.lifecycle.Session()
	if session == nil {
		return errors.New("whatsapp: runtime session is required to report retry state")
	}
	if _, _, err := session.ReportClassifiedError(ctx, instanceID, attempt.Classified); err != nil {
		return fmt.Errorf("whatsapp: report classified retry state: %w", err)
	}
	return nil
}
