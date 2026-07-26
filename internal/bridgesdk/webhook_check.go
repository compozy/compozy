package bridgesdk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

const (
	webhookReachabilityCheck   = "webhook.reachability"
	webhookReachabilityTimeout = 5 * time.Second
)

type webhookHTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type webhookHTTPDoerFunc func(*http.Request) (*http.Response, error)

func (f webhookHTTPDoerFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

// WebhookCheckRecords validates the external callback configuration and, only
// for an enabled instance, proves that its public route accepts an HTTP
// connection. Authentication and validation responses prove route ownership;
// redirects and missing routes do not.
func WebhookCheckRecords(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
	doer webhookHTTPDoer,
) []bridgepkg.BridgeCheckRecord {
	publicURL, err := bridgepkg.WebhookPublicURL(instance)
	if err != nil {
		return []bridgepkg.BridgeCheckRecord{{
			Check:  "webhook.configuration",
			Status: bridgepkg.BridgeCheckStatusFail,
			Remediation: "Set provider_config.webhook.public_url to an absolute HTTPS URL, " +
				"then run bridge verification again.",
		}}
	}

	records := []bridgepkg.BridgeCheckRecord{bridgepkg.PassCheck("webhook.configuration")}
	if !instance.Enabled {
		return append(records, bridgepkg.SkippedCheck(
			webhookReachabilityCheck,
			"Enable the bridge, then run bridge verification again to test webhook reachability.",
		))
	}
	if ctx == nil {
		return append(records, failedWebhookReachabilityCheck(
			"Run bridge verification with an active request context.",
		))
	}
	if doer == nil {
		doer = newWebhookHTTPClient(nil, nil)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodHead, publicURL, http.NoBody)
	if err != nil {
		return append(records, failedWebhookReachabilityCheck(
			"Fix webhook.public_url, then run bridge verification again.",
		))
	}
	response, err := doer.Do(request)
	if err != nil {
		return append(records, webhookReachabilityErrorCheck(err))
	}
	if response == nil {
		return append(records, failedWebhookReachabilityCheck(
			"Confirm the public webhook route is available, then run bridge verification again.",
		))
	}
	if response.Body != nil {
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			closeErr := response.Body.Close()
			if closeErr != nil {
				return append(records, failedWebhookReachabilityCheck(
					"The webhook route response could not be read or closed. Run bridge verification again.",
				))
			}
			return append(records, failedWebhookReachabilityCheck(
				"The webhook route response could not be read. Run bridge verification again.",
			))
		}
		if err := response.Body.Close(); err != nil {
			return append(records, failedWebhookReachabilityCheck(
				"The webhook route response could not be closed. Run bridge verification again.",
			))
		}
	}
	return append(records, webhookReachabilityStatusCheck(response.StatusCode))
}

func webhookReachabilityErrorCheck(err error) bridgepkg.BridgeCheckRecord {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
		return bridgepkg.BridgeCheckRecord{
			Check:       webhookReachabilityCheck,
			Status:      bridgepkg.BridgeCheckStatusWarn,
			Remediation: "The public webhook route timed out. Check connectivity, then run bridge verification again.",
		}
	}
	if errors.Is(err, context.Canceled) {
		return bridgepkg.BridgeCheckRecord{
			Check:       webhookReachabilityCheck,
			Status:      bridgepkg.BridgeCheckStatusWarn,
			Remediation: "Webhook reachability was canceled. Run bridge verification again.",
		}
	}
	return failedWebhookReachabilityCheck(
		"Confirm the public webhook route is externally reachable, then run bridge verification again.",
	)
}

func webhookReachabilityStatusCheck(statusCode int) bridgepkg.BridgeCheckRecord {
	switch {
	case statusCode >= http.StatusMultipleChoices && statusCode < http.StatusBadRequest:
		return failedWebhookReachabilityCheck(
			"Set webhook.public_url to the final external route without redirects, then run bridge verification again.",
		)
	case statusCode == http.StatusNotFound || statusCode == http.StatusGone:
		return failedWebhookReachabilityCheck(
			"The configured webhook route does not exist. Fix webhook.public_url, then run bridge verification again.",
		)
	case statusCode >= http.StatusInternalServerError:
		return bridgepkg.BridgeCheckRecord{
			Check:       webhookReachabilityCheck,
			Status:      bridgepkg.BridgeCheckStatusWarn,
			Remediation: "The public webhook route returned a server error. Check bridge health, then run verification again.",
		}
	default:
		return bridgepkg.PassCheck(webhookReachabilityCheck)
	}
}

func failedWebhookReachabilityCheck(remediation string) bridgepkg.BridgeCheckRecord {
	return bridgepkg.BridgeCheckRecord{
		Check:       webhookReachabilityCheck,
		Status:      bridgepkg.BridgeCheckStatusFail,
		Remediation: remediation,
	}
}
