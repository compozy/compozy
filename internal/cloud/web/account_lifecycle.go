package web

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/email"
)

const (
	signInNoticePath   = "/account/sign-ins/"
	signInNoticeCookie = "__Host-compozy_notice"
	accountPageTimeout = 5 * time.Second
)

type accountPages struct {
	auth     *auth.Service
	accounts *control.Service
	links    *email.UnsubscribeLinks
	view     *template.Template
	csrf     *http.CrossOriginProtection
}

type accountPage struct {
	Title, Description, Action, Button string
	ErrorCode                          string
	Notice                             bool
}

func registerAccountPages(
	mux *http.ServeMux, authService *auth.Service, service *control.Service, links *email.UnsubscribeLinks,
) (accountPages, error) {
	view, err := template.ParseFS(pages, "account-lifecycle.html")
	if err != nil {
		return accountPages{}, err
	}
	p := accountPages{auth: authService, accounts: service, links: links, view: view, csrf: http.NewCrossOriginProtection()}
	mux.HandleFunc("GET "+email.UnsubscribePath, p.unsubscribe)
	mux.HandleFunc("POST "+email.UnsubscribePath, p.unsubscribe)
	mux.HandleFunc("GET "+signInNoticePath+"{event_id}", p.signInNotice)
	mux.HandleFunc("POST "+signInNoticePath+"{event_id}", p.signInNotice)
	return p, nil
}

func (p accountPages) unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("t")
	id, ok := p.links.AccountID(token)
	if !ok {
		p.render(w, accountPage{Title: "This unsubscribe link is invalid."}, http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		p.render(w, accountPage{
			Title: "Unsubscribe from Compozy product news?", Button: "Unsubscribe",
			Action: email.UnsubscribePath + "?" + url.Values{"t": {token}}.Encode(),
		}, http.StatusOK)
		return
	}
	if !parseUnsubscribeForm(w, r) {
		p.render(w, accountPage{Title: "This unsubscribe request is invalid."}, http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), accountPageTimeout)
	defer cancel()
	if err := p.accounts.Unsubscribe(ctx, id); err != nil {
		p.fail(w, err)
		return
	}
	p.render(w, accountPage{Title: "You won't get product news and offers anymore."}, http.StatusOK)
}

func parseUnsubscribeForm(w http.ResponseWriter, r *http.Request) bool {
	const formLimit = 4 << 10
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, formLimit)
	switch mediaType {
	case "application/x-www-form-urlencoded":
		err = r.ParseForm()
	case "multipart/form-data":
		err = r.ParseMultipartForm(formLimit)
		if r.MultipartForm != nil {
			err = errors.Join(err, r.MultipartForm.RemoveAll())
		}
	default:
		return false
	}
	values := r.PostForm["List-Unsubscribe"]
	return err == nil && len(values) == 1 && values[0] == "One-Click"
}

func (p accountPages) signInNotice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("event_id"), 10, 64)
	if err != nil || id <= 0 || r.Method == http.MethodHead {
		p.fail(w, auth.ErrSignInNotice)
		return
	}
	if r.Method == http.MethodPost && p.csrf.Check(r) != nil {
		p.fail(w, auth.ErrInvalidRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), accountPageTimeout)
	defer cancel()
	token := readCookie(r, signInNoticeCookie)
	accountID, notice, err := p.auth.SignInNotice(ctx, token, id)
	if errors.Is(err, auth.ErrWebSession) && r.Method == http.MethodGet {
		p.beginNoticeOAuth(ctx, w, r, id)
		return
	}
	if err != nil {
		p.fail(w, err)
		return
	}
	if r.Method == http.MethodPost {
		if err := p.accounts.RevokeSignInNotice(ctx, p.auth, token, accountID, id); err != nil {
			p.fail(w, err)
			return
		}
		p.render(w, accountPage{Title: notice.Name + " was removed from your account."}, http.StatusOK)
		return
	}
	if r.URL.Query().Get("confirmed") == "1" {
		p.render(w, accountPage{Title: "Sign-in confirmed", Description: "You can close this tab."}, http.StatusOK)
		return
	}
	details := notice.At.UTC().Format("Jan 2, 15:04 UTC")
	if notice.Location != "" {
		details = notice.Location + " · " + details
	}
	p.render(w, accountPage{
		Title: "New sign-in on " + notice.Name, Description: details, Action: noticePath(id),
		Button: "Revoke this device", Notice: true,
	}, http.StatusOK)
}

func (p accountPages) beginNoticeOAuth(ctx context.Context, w http.ResponseWriter, r *http.Request, eventID int64) {
	browser := activationBrowser(w, r)
	next, err := p.auth.BeginSignInNoticeOAuth(ctx, eventID, browser)
	if err != nil {
		p.fail(w, err)
		return
	}
	setPageHeaders(w)
	// #nosec G710 -- The OAuth service uses the configured provider origin.
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (p signInPages) afterNoticeOAuth(w http.ResponseWriter, r *http.Request, result auth.CallbackResult, err error) {
	if err != nil {
		p.lifecycle.fail(w, err)
		return
	}
	setCookie(w, signInNoticeCookie, result.WebSession, auth.NoticeSessionTTL)
	setPageHeaders(w)
	// #nosec G710 -- A fixed relative path followed by a formatted numeric ID.
	http.Redirect(w, r, noticePath(result.NoticeEventID), http.StatusSeeOther)
}

func noticePath(id int64) string {
	return signInNoticePath + strconv.FormatInt(id, 10)
}

func (p accountPages) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrWebSession):
		p.render(w, accountPage{Title: "Sign in again to remove this device."}, http.StatusUnauthorized)
	default:
		if f, ok := errors.AsType[*auth.Failure](err); ok {
			p.render(w, accountPage{Title: f.Message, ErrorCode: f.Code}, f.Status)
			return
		}
		if f, ok := errors.AsType[*control.Error](err); ok {
			p.render(w, accountPage{Title: f.Message, ErrorCode: f.Code}, f.Status)
			return
		}
		w.Header().Set("Retry-After", "1")
		p.render(w, accountPage{Title: "Compozy Cloud is temporarily unavailable."}, http.StatusServiceUnavailable)
	}
}

func (p accountPages) render(w http.ResponseWriter, page accountPage, status int) {
	setPageHeaders(w)
	var body bytes.Buffer
	if err := p.view.Execute(&body, page); err != nil {
		http.Error(w, "Compozy Cloud is temporarily unavailable.", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.Debug("Cloud account page response interrupted", "status", status)
	}
}
