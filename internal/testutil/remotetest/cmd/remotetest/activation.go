//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func httpClientFor(d daemonInstance) *http.Client {
	return &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return new(net.Dialer).DialContext(ctx, "unix", d.Socket)
		},
	}}
}

func requestJSON(ctx context.Context, client *http.Client, method, path string, input, output any) (err error) {
	var data []byte
	if input != nil {
		data, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://localhost"+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, body)
	}
	if output == nil {
		return nil
	}
	return json.Unmarshal(body, output)
}

func signIn(f *fixture, d daemonInstance) string {
	f.Helper()
	client := httpClientFor(d)
	defer client.CloseIdleConnections()
	var login contract.AccountLoginResponse
	if err := requestJSON(f.Context(), client, http.MethodPost, contract.AccountLoginPath,
		contract.AccountLoginRequest{Provider: new(auth.Google)}, &login); err != nil {
		f.Fatal(err)
	}
	activate(f, login)
	var status contract.AccountStatusResponse
	remotetest.Await(f, "device enrollment after Google sign-in", 20*time.Second, func(ctx context.Context) bool {
		return requestJSON(ctx, client, http.MethodGet, contract.AccountPath, nil, &status) == nil &&
			status.SignedIn && status.Email == fixtureEmail && status.Device != nil
	})
	return status.Device.ID
}

func activate(f *fixture, login contract.AccountLoginResponse) {
	f.Helper()
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	defer client.CloseIdleConnections()
	target := login.VerificationURIComplete
	page, html := activationRequest(f, client, http.MethodGet, target, nil, nil)
	if page.StatusCode != http.StatusOK || !bytes.Contains(html, []byte(login.UserCode)) {
		f.Fatalf("activation page did not show the device code (HTTP %d)", page.StatusCode)
	}
	cookies := page.Cookies()
	if len(cookies) == 0 {
		f.Fatal("activation page did not bind the browser cookie")
	}
	form := url.Values{"user_code": {login.UserCode}, "provider": {auth.Google}, "cf-turnstile-response": {"pass-activate"}}
	continuation, _ := activationRequest(f, client, http.MethodPost, target, form, cookies)
	provider := redirect(f, continuation)
	authorization, _ := activationRequest(f, client, http.MethodGet, provider, nil, nil)
	callback := redirect(f, authorization)
	result, html := activationRequest(f, client, http.MethodGet, callback, nil, cookies)
	if result.StatusCode != http.StatusOK || !bytes.Contains(html, []byte("Return to CompozyOS.")) {
		f.Fatalf("OAuth callback did not complete the device grant (HTTP %d)", result.StatusCode)
	}
}

func redirect(f *fixture, response *http.Response) string {
	f.Helper()
	target, err := response.Location()
	if response.StatusCode != http.StatusSeeOther || err != nil {
		f.Fatalf("OAuth did not redirect (HTTP %d)", response.StatusCode)
	}
	return target.String()
}

func activationRequest(
	f *fixture, client *http.Client, method, target string, form url.Values, cookies []*http.Cookie,
) (*http.Response, []byte) {
	f.Helper()
	req, err := http.NewRequestWithContext(f.Context(), method, target, strings.NewReader(form.Encode()))
	if err != nil {
		f.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		f.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	if err := errors.Join(err, resp.Body.Close()); err != nil {
		f.Fatal(err)
	}
	return resp, data
}
