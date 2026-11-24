// SPDX-License-Identifier: FSL-1.1-MIT

package admin

import (
	"bytes"
	"context"
	"embed"
	"encoding/json/v2"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/billing"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

//go:embed console.html console_account.html console.css console.js
var assets embed.FS

type Config struct {
	Access  *AccessConfig
	Now     func() time.Time
	Logger  *slog.Logger
	Metrics func(context.Context) ([]observe.Sample, error)
}

type console struct {
	db       *store.Store
	accounts *control.Service
	billing  *billing.Service
	access   *accessVerifier
	csrf     *http.CrossOriginProtection
	view     *template.Template
	config   Config
}

type operatorKey struct{}

func Register(
	root *http.ServeMux, db *store.Store, accounts *control.Service, bills *billing.Service, cfg Config,
) error {
	access, err := newAccessVerifier(cfg.Access)
	if err != nil {
		return err
	}
	view, err := template.New("console.html").Funcs(template.FuncMap{
		"timestamp": timestamp, "optionalTime": optionalTime,
	}).ParseFS(assets, "console.html", "console_account.html")
	if err != nil {
		return err
	}
	c := &console{db: db, accounts: accounts, billing: bills, access: access,
		csrf: http.NewCrossOriginProtection(), view: view, config: cfg}
	pages := http.NewServeMux()
	pages.HandleFunc("GET /admin", c.overview)
	pages.HandleFunc("GET /admin/{$}", c.overview)
	pages.HandleFunc("GET /admin/search", c.search)
	pages.HandleFunc("GET /admin/accounts/{id}", c.account)
	pages.HandleFunc("GET /admin/confirm/{action}", c.confirm)
	pages.HandleFunc("POST /admin/actions/{action}", c.apply)
	pages.HandleFunc("POST /admin/accounts/{id}/export.json", c.exportAccount)
	pages.HandleFunc("POST /admin/contacts.csv", c.exportContacts)
	pages.HandleFunc("GET /admin/events/{id}", c.receipt)
	for _, asset := range []string{"console.css", "console.js"} {
		pages.HandleFunc("GET /admin/"+asset, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, assets, asset)
		})
	}
	secured := c.authorize(pages)
	root.Handle("/admin", secured)
	root.Handle("/admin/", secured)
	return nil
}

func (c *console) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; base-uri 'none'; frame-ancestors 'none'; style-src 'self'; "+
				"form-action 'self'; script-src 'self'; connect-src 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		now := c.config.Now()
		address, reason := c.access.operator(r, now)
		if reason == "" && c.csrf.Check(r) != nil {
			reason = "cross_origin"
		}
		if reason != "" {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
			defer cancel()
			if err := c.db.AppendSecurityEvent(ctx, store.SecurityEvent{
				Type: "admin_access_denied", Actor: "system", At: now, Details: map[string]any{"reason": reason},
			}); err != nil {
				c.config.Logger.WarnContext(ctx, "Could not record Cloud admin access denial")
			}
			http.Error(w, "Admin access denied.", http.StatusForbidden)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, operatorKey{}, address)))
	})
}

func operator(r *http.Request) string {
	address, _ := r.Context().Value(operatorKey{}).(string)
	return address
}

type page struct {
	Title, Error, Query, Back string
	Overview                  *overviewView
	Account                   *accountView
	Lookup                    []store.AdminAccount
	Tunnel                    *store.AdminTunnel
	Confirm                   *confirmation
	Event                     *store.AdminEvent
}

func (c *console) render(w http.ResponseWriter, status int, data page) {
	var body bytes.Buffer
	if err := c.view.Execute(&body, data); err != nil {
		c.config.Logger.Error("Could not render Cloud admin page", "error", err)
		http.Error(w, "Compozy Cloud is temporarily unavailable.", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body.Bytes()); err != nil {
		c.config.Logger.Debug("Cloud admin response interrupted")
	}
}

func (c *console) fail(w http.ResponseWriter, err error) {
	if failure, ok := errors.AsType[*control.Error](err); ok {
		c.render(w, failure.Status, page{Title: "Action unavailable", Error: failure.Message})
		return
	}
	switch {
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrAuthNotFound):
		c.render(w, http.StatusNotFound, page{Title: "Resource not found"})
	case errors.Is(err, remoteproto.ErrGatePolicyInvalid), errors.Is(err, billing.ErrComplimentaryEnd):
		c.render(w, http.StatusBadRequest, page{
			Title: "Invalid date or policy", Error: "Choose a future end time in UTC.",
		})
	default:
		c.render(w, http.StatusServiceUnavailable, page{Title: "Compozy Cloud is temporarily unavailable."})
	}
}

func timestamp(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 UTC") }

func optionalTime(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return timestamp(*t)
}

func (c *console) search(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if term == "" || len(term) > 254 {
		c.render(
			w,
			http.StatusBadRequest,
			page{Title: "Search", Error: "Enter an account, email, Stripe customer or hostname."},
		)
		return
	}
	found, err := c.db.SearchAdminAccounts(r.Context(), term)
	if err != nil {
		c.fail(w, err)
		return
	}
	var tunnel *store.AdminTunnel
	target, err := c.db.AdminTunnel(r.Context(), strings.ToLower(term))
	switch {
	case err == nil:
		tunnel = &target
	case !errors.Is(err, store.ErrNotFound):
		c.fail(w, err)
		return
	}
	c.render(w, http.StatusOK, page{Title: "Search", Query: term, Lookup: found, Tunnel: tunnel})
}

func (c *console) account(w http.ResponseWriter, r *http.Request) {
	content, err := c.db.AdminAccountData(r.Context(), r.PathValue("id"), 50)
	if err != nil {
		c.fail(w, err)
		return
	}
	var data accountView
	if err := json.Unmarshal(content, &data); err != nil {
		c.fail(w, err)
		return
	}
	c.render(w, http.StatusOK, page{Title: "Account", Account: &data})
}

func (c *console) receipt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		c.fail(w, store.ErrNotFound)
		return
	}
	event, err := c.db.AdminSecurityEvent(r.Context(), id)
	if err != nil {
		c.fail(w, err)
		return
	}
	data := page{Title: "Security event", Event: &event, Back: "/admin"}
	if event.AccountID != "" {
		data.Back = "/admin/accounts/" + event.AccountID
	}
	c.render(w, http.StatusOK, data)
}
