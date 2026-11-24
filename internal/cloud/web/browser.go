// SPDX-License-Identifier: FSL-1.1-MIT

package web

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/cloud/auth"
)

func (p signInPages) browserAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	form := r.URL.Query()
	browser := readCookie(r, activationCookie)
	if r.Method == http.MethodPost {
		if browser == "" || !parseForm(w, r) {
			p.render(w, signInPage{}, auth.ErrInvalidRequest)
			return
		}
		form = r.PostForm
	}
	request, err := p.service.ParseBrowserRequest(form)
	if err != nil {
		p.render(w, signInPage{}, err)
		return
	}
	result, err := p.service.ReuseBrowserSession(
		r.Context(),
		request,
		readCookie(r, webSessionCookie),
		readCookie(r, browserCookie),
		r.UserAgent(),
		p.service.RequestLocation(r.Header),
	)
	if result.Redirect != "" {
		redirectBrowser(w, r, result)
		return
	}
	if !errors.Is(err, auth.ErrWebSession) {
		p.render(w, signInPage{}, err)
		return
	}
	browser = activationBrowser(w, r)
	err = p.service.VerifyBrowser(r, form.Get("cf-turnstile-response"), auth.BrowserAction)
	if err == nil && request.Provider != "" {
		var next string
		next, err = p.service.BeginBrowserOAuth(r.Context(), request, browser)
		if err == nil {
			setPageHeaders(w)
			// #nosec G710 -- OAuth uses a configured endpoint and encodes the query parameters.
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}
	}
	p.render(w, signInPage{
		Title: signInTitle, Action: authorizePath, Browser: request, Providers: providerButtons(request.Provider),
		SiteKey: p.service.TurnstileSiteKey(), TurnstileAction: auth.BrowserAction,
	}, err)
}

func redirectBrowser(w http.ResponseWriter, r *http.Request, result auth.BrowserResult) {
	if result.Binding != "" {
		setCookie(w, browserCookie, result.Binding, revalidateBrowserTTL)
	}
	if result.WebSession != "" {
		setCookie(w, webSessionCookie, result.WebSession, auth.WebSessionTTL)
	}
	setPageHeaders(w)
	http.Redirect(w, r, result.Redirect, http.StatusSeeOther)
}
