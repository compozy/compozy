package auth

import (
	"fmt"
	"net/url"
	"strings"
)

func validateExchangeInput(input ExchangeInput) (string, error) {
	redirectURL := strings.TrimSpace(input.RedirectURL)
	if redirectURL == "" {
		return "", fmt.Errorf("%w: redirect_url is required", ErrInvalidExchange)
	}
	parsed, err := url.Parse(redirectURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: redirect_url must be an absolute URL", ErrInvalidExchange)
	}
	return redirectURL, nil
}

func validateCallbackRedirect(expectedURL string, callbackURL string) error {
	expected, err := url.Parse(strings.TrimSpace(expectedURL))
	if err != nil || expected.Scheme == "" || expected.Host == "" {
		return fmt.Errorf("%w: stored redirect_url is invalid", ErrInvalidExchange)
	}
	callback, err := url.Parse(strings.TrimSpace(callbackURL))
	if err != nil || callback.Scheme == "" || callback.Host == "" {
		return fmt.Errorf("%w: callback URL is invalid", ErrInvalidExchange)
	}
	if !strings.EqualFold(expected.Scheme, callback.Scheme) ||
		!strings.EqualFold(expected.Host, callback.Host) ||
		expected.EscapedPath() != callback.EscapedPath() ||
		expected.User.String() != callback.User.String() {
		return fmt.Errorf("%w: callback URL does not match the pending redirect_url", ErrInvalidExchange)
	}
	for key, expectedValues := range expected.Query() {
		callbackValues, found := callback.Query()[key]
		if !found || len(callbackValues) != len(expectedValues) {
			return fmt.Errorf("%w: callback URL does not preserve redirect_url query", ErrInvalidExchange)
		}
		for index, value := range expectedValues {
			if callbackValues[index] != value {
				return fmt.Errorf("%w: callback URL does not preserve redirect_url query", ErrInvalidExchange)
			}
		}
	}
	return nil
}

func stateFromCallback(callbackURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(callbackURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: callback URL is invalid", ErrInvalidExchange)
	}
	state := strings.TrimSpace(parsed.Query().Get("state"))
	if state == "" {
		return "", fmt.Errorf("%w: OAuth callback state is required", ErrInvalidExchange)
	}
	return state, nil
}
