package billing

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	stripeVersion   = "2025-03-31.basil"
	stripeTimeout   = 10 * time.Second
	accountMetadata = "compozy_account_id"
)

type StripeConfig struct {
	SecretKey, WebhookSecret string
	Endpoint                 string
}

type Stripe struct {
	config StripeConfig
	client *http.Client
}

func NewStripe(cfg StripeConfig) (*Stripe, error) {
	if cfg.SecretKey == "" || cfg.WebhookSecret == "" {
		return nil, errors.New("Stripe API key and webhook signing secret are required")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://api.stripe.com"
	}
	origin, err := url.Parse(cfg.Endpoint)
	if err != nil || origin.Host == "" || origin.User != nil || origin.Path != "" ||
		origin.RawQuery != "" || origin.Fragment != "" || (origin.Scheme != "https" && origin.Scheme != "http") {
		return nil, errors.New("invalid Stripe API origin")
	}
	return &Stripe{config: cfg, client: &http.Client{
		Timeout:       stripeTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

type stripeFailure struct {
	Status int
	Code   string
}

func (e *stripeFailure) Error() string {
	return fmt.Sprintf("Stripe request failed with status %d", e.Status)
}

func stripeCode(err error, code string) bool {
	failure, ok := errors.AsType[*stripeFailure](err)
	return ok && failure.Code == code
}

func (s *Stripe) request(
	ctx context.Context, method, path string, params url.Values, key string, out any,
) (retErr error) {
	var body io.Reader
	switch method {
	case http.MethodGet:
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	case http.MethodPost:
		body = strings.NewReader(params.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, s.config.Endpoint+path, body)
	if err != nil {
		return errors.New("cannot prepare Stripe request")
	}
	req.SetBasicAuth(s.config.SecretKey, "")
	req.Header.Set("Stripe-Version", stripeVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return errors.New("cannot reach Stripe")
	}
	defer func() {
		_, drainErr := io.Copy(io.Discard, res.Body)
		retErr = errors.Join(retErr, drainErr, res.Body.Close())
	}()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return errors.New("cannot read Stripe response")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var result struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		failure := &stripeFailure{Status: res.StatusCode}
		if json.Unmarshal(raw, &result) == nil {
			failure.Code = result.Error.Code
		}
		return failure
	}
	if out != nil && json.Unmarshal(raw, out) != nil {
		return errors.New("invalid Stripe response")
	}
	return nil
}

type stripeCheckout struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	Status          string `json:"status"`
	PaymentStatus   string `json:"payment_status"`
	Subscription    string `json:"subscription"`
	ClientReference string `json:"client_reference_id"`
}

type stripeCheckoutRequest struct {
	AccountID, Email, CustomerID, PriceID, Key string
	SuccessURL, CancelURL                      string
}

func (s *Stripe) createCheckout(ctx context.Context, request stripeCheckoutRequest) (stripeCheckout, error) {
	params := url.Values{
		"mode":                      {"subscription"},
		"managed_payments[enabled]": {"true"},
		"allow_promotion_codes":     {"true"},
		"line_items[0][price]":      {request.PriceID},
		"line_items[0][quantity]":   {"1"},
		"client_reference_id":       {request.AccountID},
		"subscription_data[metadata][" + accountMetadata + "]": {request.AccountID},
		"success_url": {request.SuccessURL},
		"cancel_url":  {request.CancelURL},
	}
	if request.CustomerID != "" {
		params.Set("customer", request.CustomerID)
	} else {
		params.Set("customer_email", request.Email)
	}
	var result stripeCheckout
	err := s.request(ctx, http.MethodPost, "/v1/checkout/sessions", params, request.Key, &result)
	if err == nil && (!validStripeID(result.ID, "cs_") || !s.validRedirect(result.URL, "checkout.stripe.com")) {
		err = errors.New("invalid Stripe Checkout session")
	}
	return result, err
}

func (s *Stripe) checkout(ctx context.Context, id string) (stripeCheckout, error) {
	var result stripeCheckout
	err := s.request(ctx, http.MethodGet, "/v1/checkout/sessions/"+url.PathEscape(id), nil, "", &result)
	if err == nil && result.ID != id {
		err = errors.New("Stripe returned a different Checkout session")
	}
	return result, err
}

func (s *Stripe) portal(ctx context.Context, customer, returnURL string) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	params := url.Values{"customer": {customer}, "return_url": {returnURL}}
	err := s.request(ctx, http.MethodPost, "/v1/billing_portal/sessions", params, "", &result)
	if err == nil && !s.validRedirect(result.URL, "billing.stripe.com") {
		err = errors.New("invalid Stripe Customer Portal URL")
	}
	return result.URL, err
}

func (s *Stripe) validRedirect(target, host string) bool {
	u, err := url.Parse(target)
	if err != nil || u.User != nil || u.Path == "" {
		return false
	}
	return (u.Scheme == "https" && u.Host == host) || u.Scheme+"://"+u.Host == s.config.Endpoint
}

func validStripeID(id, prefix string) bool {
	tail, ok := strings.CutPrefix(id, prefix)
	return ok && tail != "" && len(id) <= 128 && !strings.ContainsFunc(tail, func(r rune) bool {
		return r != '_' && r != '-' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
}
