package auth

import (
	"fmt"
	"net/url"
	"strings"
)

func validateExchangeInput(input ExchangeInput) (string, error) {
	code := strings.TrimSpace(input.Code)
	redirectURL := strings.TrimSpace(input.RedirectURL)
	if (code == "") == (redirectURL == "") {
		return "", fmt.Errorf("%w: exactly one of code or redirect_url is required", ErrInvalidExchange)
	}
	if redirectURL != "" {
		parsed, err := url.Parse(redirectURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", fmt.Errorf("%w: redirect_url must be an absolute URL", ErrInvalidExchange)
		}
	}
	return redirectURL, nil
}

func callbackURLForCode(state LoginState, code string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("%w: authorization code is required", ErrInvalidExchange)
	}
	parsed, err := url.Parse(state.RedirectURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: stored callback URL is invalid", ErrInvalidExchange)
	}
	values := parsed.Query()
	values.Set("code", code)
	values.Set("state", state.State)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
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
