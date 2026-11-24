// SPDX-License-Identifier: FSL-1.1-MIT

package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
)

const (
	googleIssuer        = "https://accounts.google.com"
	maxProviderResponse = 256 << 10
)

type OAuthClient struct {
	ID, Secret string
}

type OAuthEndpoints struct {
	GoogleAuthorize, GoogleToken, GoogleJWKS               string
	GitHubAuthorize, GitHubToken, GitHubUser, GitHubEmails string
}

type OAuth struct {
	google, github OAuthClient
	endpoints      OAuthEndpoints
	http           *http.Client
	googleKeys     googleKeyCache
}

type OAuthOption func(*OAuth)

func WithOAuthEndpoints(endpoints OAuthEndpoints) OAuthOption {
	return func(o *OAuth) { o.endpoints = endpoints }
}

func NewOAuth(google, github OAuthClient, options ...OAuthOption) (*OAuth, error) {
	if strings.TrimSpace(google.ID) == "" || strings.TrimSpace(google.Secret) == "" ||
		strings.TrimSpace(github.ID) == "" || strings.TrimSpace(github.Secret) == "" {
		return nil, errors.New("missing Google or GitHub OAuth credentials")
	}
	o := &OAuth{
		google: google, github: github,
		endpoints: OAuthEndpoints{ // #nosec G101 -- public OAuth endpoint URLs.
			GoogleAuthorize: googleIssuer + "/o/oauth2/v2/auth",
			GoogleToken:     "https://oauth2.googleapis.com/token",
			GoogleJWKS:      "https://www.googleapis.com/oauth2/v3/certs",
			GitHubAuthorize: "https://github.com/login/oauth/authorize",
			GitHubToken:     "https://github.com/login/oauth/access_token",
			GitHubUser:      "https://api.github.com/user",
			GitHubEmails:    "https://api.github.com/user/emails",
		},
		http: &http.Client{Timeout: 5 * time.Second, CheckRedirect: refuseRedirect},
	}
	for _, apply := range options {
		apply(o)
	}
	return o, nil
}

func refuseRedirect(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }

func (s *Service) OAuthFormOrigins() []string {
	var origins []string
	for _, endpoint := range []string{s.oauth.endpoints.GoogleAuthorize, s.oauth.endpoints.GitHubAuthorize} {
		parsed, err := url.Parse(endpoint)
		if err == nil && parsed.Host != "" && parsed.User == nil &&
			(parsed.Scheme == "https" || parsed.Scheme == "http") {
			origins = append(origins, parsed.Scheme+"://"+parsed.Host)
		}
	}
	return origins
}

func (o *OAuth) authorize(issuer, provider, state, verifier, nonce string, fresh bool) string {
	client, endpoint, scope := o.github, o.endpoints.GitHubAuthorize, "user:email"
	if provider == Google {
		client, endpoint, scope = o.google, o.endpoints.GoogleAuthorize, "openid email profile"
	}
	query := url.Values{
		"client_id": {client.ID}, "redirect_uri": {callbackURL(issuer, provider)}, "response_type": {oauthCodeField},
		"scope": {scope}, oauthStateField: {state}, pkceChallengeField: {challenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	if provider == Google {
		query.Set("nonce", nonce)
		if fresh {
			query.Set("max_age", "0")
			query.Set("claims", `{"id_token":{"auth_time":{"essential":true}}}`)
		}
	}
	return endpoint + "?" + query.Encode()
}

func callbackURL(issuer, provider string) string { return issuer + "/v1/auth/callback/" + provider }

type providerTokens struct {
	IDToken     string `json:"id_token"`
	AccessToken string `json:"access_token"`
}

func (o *OAuth) exchange(
	ctx context.Context, issuer string, attempt store.OAuthAttempt, code, verifier string, now time.Time,
) (store.VerifiedIdentity, error) {
	client, endpoint := o.github, o.endpoints.GitHubToken
	if attempt.Provider == Google {
		client, endpoint = o.google, o.endpoints.GoogleToken
	}
	form := url.Values{
		"client_id": {
			client.ID,
		},
		"client_secret": {client.Secret},
		"redirect_uri":  {callbackURL(issuer, attempt.Provider)},
		"grant_type":    {"authorization_code"},
		oauthCodeField:  {code},
		"code_verifier": {verifier},
	}
	var tokens providerTokens
	if err := o.request(ctx, endpoint, "", form, &tokens); err != nil {
		return store.VerifiedIdentity{}, err
	}
	if attempt.Provider == Google {
		return o.googleIdentity(ctx, tokens.IDToken, attempt, now)
	}
	return o.githubIdentity(ctx, tokens.AccessToken)
}

func (o *OAuth) request(ctx context.Context, endpoint, access string, form url.Values, value any) error {
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method, body = http.MethodPost, strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return errProvider
	}
	req.Header.Set("Accept", "application/json")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	res, err := o.http.Do(req)
	if err != nil {
		return errProvider
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return errProvider
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxProviderResponse+1))
	if err != nil || len(data) > maxProviderResponse || json.Unmarshal(data, value) != nil {
		return errProvider
	}
	return nil
}

func (o *OAuth) githubIdentity(ctx context.Context, access string) (store.VerifiedIdentity, error) {
	var user struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if access == "" {
		return store.VerifiedIdentity{}, errProvider
	}
	if err := o.request(ctx, o.endpoints.GitHubUser, access, nil, &user); err != nil {
		return store.VerifiedIdentity{}, err
	}
	if user.ID <= 0 {
		return store.VerifiedIdentity{}, errProvider
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := o.request(ctx, o.endpoints.GitHubEmails, access, nil, &emails); err != nil {
		return store.VerifiedIdentity{}, err
	}
	for _, email := range emails {
		if email.Primary && email.Verified {
			return identity(GitHub, strconv.FormatInt(user.ID, 10), email.Email, user.Name)
		}
	}
	return store.VerifiedIdentity{}, errEmailUnverified
}

func identity(provider, subject, email, name string) (store.VerifiedIdentity, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validLabel(email, 320) {
		return store.VerifiedIdentity{}, errEmailUnverified
	}
	if !validLabel(subject, 255) {
		return store.VerifiedIdentity{}, errProvider
	}
	if !validLabel(name, 160) {
		name = ""
	}
	return store.VerifiedIdentity{Provider: provider, Subject: subject, Email: email, Name: name}, nil
}

func equalHash(left, right []byte) bool {
	return subtle.ConstantTimeCompare(left, right) == 1
}
