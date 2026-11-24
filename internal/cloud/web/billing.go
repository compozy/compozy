package web

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/billing"
	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
)

const purchaseTimeout = 25 * time.Second

type billingPages struct {
	auth     *auth.Service
	billing  *billing.Service
	limiter  *api.AccountLimiter
	template *template.Template
}

func registerBilling(
	mux *http.ServeMux, service *auth.Service, bills *billing.Service, limiter *api.AccountLimiter,
) (billingPages, error) {
	t, err := template.ParseFS(pages, "billing.html")
	if err != nil {
		return billingPages{}, err
	}
	p := billingPages{auth: service, billing: bills, limiter: limiter, template: t}
	mux.HandleFunc("GET "+billing.UpgradePath, p.upgrade)
	mux.HandleFunc("GET "+billing.SuccessPath, p.success)
	mux.HandleFunc("POST "+billing.UpgradePath+"/sign-in", p.signIn)
	mux.HandleFunc("POST "+billing.UpgradePath+"/sign-out", p.signOut)
	mux.HandleFunc("POST "+billing.UpgradePath+"/checkout", p.checkout)
	mux.HandleFunc("POST "+billing.UpgradePath+"/portal", p.portal)
	return p, nil
}

type billingPage struct {
	Target                                                        billing.Target
	Overview                                                      billing.Overview
	Providers                                                     []providerButton
	Title, Code, Message, Email, CSRF, SiteKey, NextURL, RetryURL string
	RetryAfter                                                    int
	Success, CheckoutCanceled, Redirecting, ShowPlans, CanManage  bool
	AlreadyPro, OpeningCheckout                                   bool
}

const billingIntervalYear = "year"

func (p billingPage) Monthly() bool { return p.Target.Interval == "month" }

func (p billingPage) PlanLabel() string {
	interval := p.Target.Interval
	if p.AlreadyPro || p.Success {
		sub := p.Overview.Billing.Subscription
		if sub.Source == "complimentary" {
			return "Pro · complimentary"
		}
		interval = sub.Interval
	}
	switch interval {
	case billingIntervalYear:
		return "Pro annual"
	case "month":
		return "Pro monthly"
	default:
		return "Pro"
	}
}

func (p billingPage) PeriodEnd() string {
	subscription := p.Overview.Billing.AccountSubscription()
	if subscription == nil || subscription.CurrentPeriodEnd == nil {
		return ""
	}
	return subscription.CurrentPeriodEnd.UTC().Format("Jan 2, 2006")
}

func (p billingPage) PeriodEndLabel() string {
	subscription := p.Overview.Billing.Subscription
	if subscription.Source == "complimentary" || subscription.CancelAtPeriodEnd {
		return "Ends"
	}
	return "Renews"
}

func purchasePage(target billing.Target) billingPage {
	title := "Get Compozy Pro"
	if target.Hostname != "" {
		title = "Upgrade to Compozy Pro"
	}
	return billingPage{Target: target, Title: title, ShowPlans: true}
}

func (p billingPages) upgrade(w http.ResponseWriter, r *http.Request) {
	target, err := p.billing.Target(r.URL.Query())
	page := purchasePage(target)
	if err != nil {
		p.render(w, page, err)
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	page.CheckoutCanceled = r.URL.Query().Get("checkout") == billing.CheckoutCancelled
	page.CSRF = activationBrowser(w, r)
	session, err := p.auth.WebSession(r.Context(), readCookie(r, webSessionCookie))
	page.Email = session.Email
	switch {
	case errors.Is(err, auth.ErrWebSession):
		page.Providers, page.SiteKey = providerButtons(""), p.auth.TurnstileSiteKey()
		p.render(w, page, nil)
	case err != nil:
		p.render(w, page, err)
	default:
		p.renderOverview(w, r, page, session.AccountID)
	}
}

func (p billingPages) renderOverview(w http.ResponseWriter, r *http.Request, page billingPage, accountID string) {
	overview, err := p.billing.Overview(r.Context(), accountID, page.Target, p.webAccess(r, accountID))
	if err == nil {
		page.Overview = overview
		if overview.Billing.Plan == plan.Pro {
			page.Title, page.ShowPlans = "You're already on Pro.", false
			page.AlreadyPro = true
			page.CanManage = overview.Billing.Subscription.CustomerID != ""
		}
	}
	p.render(w, page, err)
}

func (p billingPages) signIn(w http.ResponseWriter, r *http.Request) {
	target, cookie, ok := p.purchaseForm(w, r)
	if !ok {
		return
	}
	page := purchasePage(target)
	page.CSRF, page.Providers, page.SiteKey = cookie, providerButtons(""), p.auth.TurnstileSiteKey()
	if err := p.auth.VerifyBrowser(r, r.PostForm.Get("cf-turnstile-response"), auth.BrowserAction); err != nil {
		p.render(w, page, err)
		return
	}
	next, err := p.auth.BeginPurchaseOAuth(r.Context(), r.PostForm.Get("provider"), cookie, target.URL())
	if err != nil {
		p.render(w, page, err)
		return
	}
	setPageHeaders(w)
	// #nosec G710 -- OAuth chooses its configured provider endpoint.
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (p billingPages) afterOAuth(w http.ResponseWriter, r *http.Request, next string, err error) {
	if err != nil {
		page := purchasePage(billing.Target{Interval: billingIntervalYear})
		page.RetryURL = next
		p.render(w, page, err)
		return
	}
	setPageHeaders(w)
	// #nosec G710 -- The persisted purchase return is a validated cloud-relative URL.
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (p billingPages) signOut(w http.ResponseWriter, r *http.Request) {
	target, _, ok := p.purchaseForm(w, r)
	if !ok {
		return
	}
	if err := p.auth.EndWebSession(r.Context(), readCookie(r, webSessionCookie)); err != nil {
		p.render(w, purchasePage(target), err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: webSessionCookie, Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	redirectPurchase(w, r, target)
}

func (p billingPages) checkout(w http.ResponseWriter, r *http.Request) {
	p.mutate(w, r, false)
}

func (p billingPages) portal(w http.ResponseWriter, r *http.Request) {
	p.mutate(w, r, true)
}

func (p billingPages) mutate(w http.ResponseWriter, r *http.Request, portal bool) {
	target, cookie, ok := p.purchaseForm(w, r)
	if !ok {
		return
	}
	page := purchasePage(target)
	page.CSRF = cookie
	session, err := p.auth.WebSession(r.Context(), readCookie(r, webSessionCookie))
	if errors.Is(err, auth.ErrWebSession) {
		redirectPurchase(w, r, target)
		return
	}
	if err != nil {
		p.render(w, page, err)
		return
	}
	page.Email = session.Email
	if retry := p.limiter.RetryAfter(session.AccountID); retry > 0 {
		page.RetryAfter = retry
		p.render(w, page, &auth.Failure{Status: http.StatusTooManyRequests, Code: api.CodeRateLimited})
		return
	}
	p.openStripe(w, r, page, session.AccountID, portal)
}

func (p billingPages) openStripe(
	w http.ResponseWriter,
	r *http.Request,
	page billingPage,
	accountID string,
	portal bool,
) {
	ctx, cancel := context.WithTimeout(r.Context(), purchaseTimeout)
	defer cancel()
	var next string
	var err error
	if portal {
		next, err = p.billing.Portal(ctx, accountID, p.webAccess(r, accountID))
	} else {
		next, err = p.billing.Checkout(ctx, accountID, page.Target, p.webAccess(r, accountID))
	}
	if errors.Is(err, billing.ErrAlreadyPro) || errors.Is(err, auth.ErrWebSession) {
		redirectPurchase(w, r, page.Target)
		return
	}
	if err == nil {
		page.Redirecting, page.ShowPlans, page.NextURL = true, false, next
		page.OpeningCheckout = !portal
		page.Title = "Opening Stripe Checkout…"
		if portal {
			page.Title = "Opening the Stripe Customer Portal…"
		}
	}
	p.render(w, page, err)
}

func (p billingPages) success(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	page := purchasePage(billing.Target{Interval: billingIntervalYear})
	page.ShowPlans = false
	session, err := p.auth.WebSession(r.Context(), readCookie(r, webSessionCookie))
	if errors.Is(err, auth.ErrWebSession) {
		redirectPurchase(w, r, page.Target)
		return
	}
	if err != nil {
		p.render(w, page, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), purchaseTimeout)
	defer cancel()
	id := r.URL.Query().Get("session_id")
	overview, err := p.billing.CompleteCheckout(ctx, session.AccountID, id, p.webAccess(r, session.AccountID))
	if err == nil && overview.HostedURL != "" {
		setPageHeaders(w)
		// #nosec G710 -- The hostname came from the retained Checkout and was verified against its owner.
		http.Redirect(w, r, overview.HostedURL, http.StatusSeeOther)
		return
	}
	page.Title, page.Success, page.Email = "You're on Pro. Open CompozyOS to use your computers.", err == nil, session.Email
	if err == nil {
		page.Overview = overview
	}
	if errors.Is(err, billing.ErrPaymentPending) {
		page.RetryURL = billing.SuccessPath + "?" + url.Values{"session_id": {id}}.Encode()
	}
	p.render(w, page, err)
}

func (p billingPages) purchaseForm(w http.ResponseWriter, r *http.Request) (billing.Target, string, bool) {
	cookie := readCookie(r, activationCookie)
	if !parseForm(w, r) || cookie == "" ||
		subtle.ConstantTimeCompare([]byte(cookie), []byte(r.PostForm.Get("csrf"))) != 1 {
		p.render(w, purchasePage(billing.Target{}), auth.ErrInvalidRequest)
		return billing.Target{}, "", false
	}
	target, err := p.billing.Target(r.PostForm)
	if err != nil {
		p.render(w, purchasePage(target), err)
	}
	return target, cookie, err == nil
}

func (p billingPages) webAccess(r *http.Request, accountID string) billing.Authorize {
	token := readCookie(r, webSessionCookie)
	return func(ctx context.Context, q *store.Queries) error {
		return p.auth.CheckWebSession(ctx, q, token, accountID)
	}
}

func redirectPurchase(w http.ResponseWriter, r *http.Request, target billing.Target) {
	setPageHeaders(w)
	// #nosec G710 -- target.URL() builds a validated relative /billing/upgrade path.
	http.Redirect(w, r, target.URL(), http.StatusSeeOther)
}

func (p billingPages) render(w http.ResponseWriter, page billingPage, err error) {
	setPageHeaders(w, p.auth.OAuthFormOrigins()...)
	status := http.StatusOK
	switch {
	case err == nil:
	case errors.Is(err, auth.ErrTurnstile):
		status, page.Code, page.Message = auth.ErrTurnstile.Status, auth.ErrTurnstile.Code, auth.ErrTurnstile.Message
	default:
		status = purchaseError(&page, err)
	}
	if page.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(page.RetryAfter))
	}
	var body bytes.Buffer
	if err := p.template.Execute(&body, page); err != nil {
		http.Error(w, billing.ErrUnavailable.Message, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.Debug("Cloud purchase response interrupted", "status", status)
	}
}

func purchaseError(page *billingPage, err error) int {
	status := http.StatusServiceUnavailable
	page.Code, page.Title = billing.ErrUnavailable.Code, billing.ErrUnavailable.Message
	page.ShowPlans = false
	if failure, ok := errors.AsType[*billing.Failure](err); ok {
		status, page.Code, page.Title = failure.Status, failure.Code, failure.Message
	}
	if failure, ok := errors.AsType[*auth.Failure](err); ok {
		status, page.Code, page.Title = failure.Status, failure.Code, failure.Message
		if status == http.StatusTooManyRequests {
			page.Title, page.ShowPlans = purchasePage(page.Target).Title, true
			page.Message = "Too many requests. Try again in " + strconv.Itoa(page.RetryAfter) + " seconds."
			page.RetryURL = page.Target.URL()
		}
	}
	if status == http.StatusServiceUnavailable {
		page.RetryAfter, page.RetryURL = 1, page.Target.URL()
	}
	return status
}
