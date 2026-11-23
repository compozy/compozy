package web

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/packages/brand"
)

const (
	activationCookie     = "__Host-compozy_act"
	webSessionCookie     = "__Host-compozy_ws"
	browserCookie        = "__Host-compozy_bd"
	activatePath         = "/activate"
	authorizePath        = "/v1/browser/authorize"
	revalidateBrowserTTL = 365 * 24 * time.Hour
)

//go:embed sign-in.html assets/*
var pages embed.FS

type signInPages struct {
	service  *auth.Service
	template *template.Template
}

func Register(mux *http.ServeMux, service *auth.Service) error {
	t, err := template.ParseFS(pages, "sign-in.html")
	if err != nil {
		return err
	}
	p := signInPages{service: service, template: t}
	mux.HandleFunc("GET "+activatePath, p.activate)
	mux.HandleFunc("POST "+activatePath, p.continueActivation)
	mux.HandleFunc("GET /v1/auth/callback/{provider}", p.callback)
	mux.HandleFunc("GET "+authorizePath, p.browserAuthorize)
	mux.HandleFunc("POST "+authorizePath, p.browserAuthorize)
	mux.Handle("GET /auth-assets/", http.StripPrefix("/auth-assets/", http.FileServerFS(assetFiles(pages, "assets"))))
	mux.Handle("GET /brand/", http.FileServerFS(assetFiles(brand.CloudAssets, "public")))
	return nil
}

func assetFiles(source fs.FS, path string) fs.FS {
	assets, err := fs.Sub(source, path)
	if err != nil {
		panic("invariant: invalid embedded cloud asset directory")
	}
	return assets
}

type signInPage struct {
	Title, Description, Status, ErrorCode          string
	Code, Installation, Platform, Location         string
	AccountEmail, SiteKey, Action, TurnstileAction string
	Providers                                      []providerButton
	Browser                                        auth.BrowserRequest
	Fresh, EnterCode, SignedIn, CodeUsed           bool
	RecoveryURL, RetryURL                          string
}

type providerButton struct{ ID, Name string }

func providerButtons(selected string) []providerButton {
	google := providerButton{auth.Google, "Google"}
	github := providerButton{auth.GitHub, "GitHub"}
	switch selected {
	case auth.Google:
		return []providerButton{google}
	case auth.GitHub:
		return []providerButton{github}
	default:
		return []providerButton{google, github}
	}
}

func setPageHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; frame-ancestors 'none'; "+
		"script-src 'self' https://challenges.cloudflare.com; frame-src https://challenges.cloudflare.com; "+
		"style-src 'self'; img-src 'self'; form-action 'self' https://accounts.google.com https://github.com "+
		"https://*.compozy.run https://*.staging.compozy.run")
}

func (p signInPages) render(w http.ResponseWriter, page signInPage, err error) {
	setPageHeaders(w)
	status := http.StatusOK
	switch f, ok := errors.AsType[*auth.Failure](err); {
	case ok:
		status, page.ErrorCode = f.Status, f.Code
		if page.Title == "" {
			page.Title = f.Message
		} else {
			page.Status = f.Message
		}
		if f.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(f.RetryAfter))
			minutes := (f.RetryAfter + 59) / 60
			page.Status = "Too many attempts. Try again in " + strconv.Itoa(minutes) + " minutes."
		}
	case err != nil:
		status, page.ErrorCode = http.StatusServiceUnavailable, api.CodeUnavailable
		page.Title = "Compozy Cloud is temporarily unavailable."
		w.Header().Set("Retry-After", "1")
	}
	var body bytes.Buffer
	if err := p.template.Execute(&body, page); err != nil {
		http.Error(w, "Compozy Cloud is temporarily unavailable.", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}

func readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil || !auth.ValidSecret(c.Value) {
		return ""
	}
	return c.Value
}

func setCookie(w http.ResponseWriter, name, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: int(ttl.Seconds()),
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

func activationBrowser(w http.ResponseWriter, r *http.Request) string {
	const browserTTL = 15 * time.Minute
	value := readCookie(r, activationCookie)
	if value == "" {
		value = auth.Secret()
	}
	setCookie(w, activationCookie, value, browserTTL)
	return value
}

func parseForm(w http.ResponseWriter, r *http.Request) bool {
	const formLimit = 12 << 10
	r.Body = http.MaxBytesReader(w, r.Body, formLimit)
	return r.ParseForm() == nil && strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
}
