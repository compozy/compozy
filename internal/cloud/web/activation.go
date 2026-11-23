package web

import (
	"net/http"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/store"
)

const signInTitle = "Sign in to CompozyOS"

func (p signInPages) activate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	browser := activationBrowser(w, r)
	code, provider, fresh := query.Get("user_code"), query.Get("provider"), query.Get("max_age") == "0"
	if code == "" {
		p.render(w, signInPage{Title: signInTitle, EnterCode: true, Fresh: fresh}, nil)
		return
	}
	grant, err := p.service.FindActivation(r.Context(), browser, code)
	p.render(w, p.activationPage(grant, provider, fresh, err), err)
}

func (p signInPages) continueActivation(w http.ResponseWriter, r *http.Request) {
	browser := readCookie(r, activationCookie)
	if browser == "" || !parseForm(w, r) {
		p.render(w, signInPage{Title: signInTitle, EnterCode: true}, auth.ErrInvalidRequest)
		return
	}
	code, provider := r.PostForm.Get("user_code"), r.PostForm.Get("provider")
	fresh := r.PostForm.Get("max_age") == "0"
	grant, err := p.service.FindActivation(r.Context(), browser, code)
	if err == nil {
		err = p.service.VerifyBrowser(r, r.PostForm.Get("cf-turnstile-response"), auth.ActivateAction)
	}
	if err == nil {
		var next string
		next, err = p.service.BeginDeviceOAuth(r.Context(), grant, provider, browser, fresh)
		if err == nil {
			setPageHeaders(w)
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}
	}
	p.render(w, p.activationPage(grant, provider, fresh, err), err)
}

func (p signInPages) activationPage(grant store.DeviceGrant, provider string, fresh bool, err error) signInPage {
	if err != nil && err != auth.ErrTurnstile {
		page := signInPage{Code: grant.UserCode, Installation: grant.ClientName}
		if f, ok := err.(*auth.Failure); ok {
			page.CodeUsed = f.Code == "device_code_used"
			page.EnterCode = f.Code == "user_code_invalid"
		}
		return page
	}
	if grant.Provider != "" {
		provider = grant.Provider
	}
	if fresh && provider == "" {
		provider = auth.Google
	}
	page := signInPage{
		Title: signInTitle, Action: activatePath, SiteKey: p.service.TurnstileSiteKey(), Fresh: fresh,
		Code: grant.UserCode, Installation: grant.ClientName, Platform: grant.ClientPlatform, Location: grant.Location,
		Providers: providerButtons(provider), TurnstileAction: auth.ActivateAction,
	}
	if !fresh && provider != auth.GitHub {
		page.RecoveryURL = activatePath + "?" + auth.ActivationQuery(grant.UserCode, auth.Google, true)
	}
	return page
}

func (p signInPages) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	provider := r.PathValue("provider")
	result, err := p.service.CompleteOAuth(r.Context(), auth.Callback{
		Provider: provider, State: query.Get("state"), Code: query.Get("code"), Denied: query.Get("error") != "",
		Cookie: readCookie(r, activationCookie), BrowserBinding: readCookie(r, browserCookie), UserAgent: r.UserAgent(),
	})
	if result.WebSession != "" {
		setCookie(w, webSessionCookie, result.WebSession, auth.WebSessionTTL)
	}
	if result.Browser.Redirect != "" {
		redirectBrowser(w, r, result.Browser)
		return
	}
	if err != nil {
		p.render(w, callbackError(result.Grant, provider, err), err)
		return
	}
	p.render(w, signInPage{
		Title: "You're signed in. Return to CompozyOS.", SignedIn: true,
		Installation: result.Grant.ClientName, AccountEmail: result.Account.Email,
	}, nil)
}

func callbackError(grant store.DeviceGrant, provider string, err error) signInPage {
	page := signInPage{Code: grant.UserCode, Installation: grant.ClientName}
	if f, ok := err.(*auth.Failure); ok {
		page.CodeUsed = f.Code == "device_code_used"
		switch f.Code {
		case "sign_in_email_unverified":
			page.Description = "Your " + providerButtons(provider)[0].Name +
				" account has no verified email. Verify one and try again."
			if grant.UserCode != "" {
				page.RetryURL = activatePath + "?" + auth.ActivationQuery(grant.UserCode, provider, false)
			}
		case "fresh_sign_in_required":
			if grant.UserCode != "" {
				page.RetryURL = activatePath + "?" + auth.ActivationQuery(grant.UserCode, provider, true)
			}
		}
	}
	return page
}
