//go:build integration

package remotetest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/billing"
)

const (
	StripeAnnual  = "price_pro_annual"
	StripeMonthly = "price_pro_monthly"
	stripeKey     = "sk_test_fixture"
	stripeSigner  = "whsec_fixture"
)

type StripeInvoice struct {
	ID, Reason, Price string
	PaidAt            time.Time
}

type StripeSubscription struct {
	ID, Account, Status, Price, Interval string
	PeriodEnd                            time.Time
	CancellationReason                   string
	Invoices                             []StripeInvoice
}

type StripeCheckout struct {
	Form   url.Values
	ID     string
	Status string
	Sub    string
}

type StripeFixture struct {
	Client  *billing.Stripe
	Origin  string
	now     func() time.Time
	browser bool

	mu            sync.Mutex
	subscriptions map[string]StripeSubscription
	checkouts     []StripeCheckout
	reads         map[string]int
	onRead        func(*http.Request, StripeSubscription)
	onComplete    func(StripeCheckout)
	refunds       map[string]url.Values
	deleteFailure bool
	deletes       []string
	redacted      []string
	replayEvents  []map[string]any
	replayFails   bool
}

func StartStripe(t Fixture, now func() time.Time, browser bool) *StripeFixture {
	t.Helper()
	f := &StripeFixture{
		now: now, browser: browser, subscriptions: make(map[string]StripeSubscription),
		reads: make(map[string]int), refunds: make(map[string]url.Values),
	}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	client, err := billing.NewStripe(billing.StripeConfig{
		Endpoint: server.URL, SecretKey: stripeKey, WebhookSecret: stripeSigner,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.Client, f.Origin = client, server.URL
	return f
}

func StripeSignature(body []byte, at time.Time) string {
	digest := hmac.New(sha256.New, []byte(stripeSigner))
	fmt.Fprintf(digest, "%d.%s", at.Unix(), body)
	return fmt.Sprintf("t=%d,v1=%s", at.Unix(), hex.EncodeToString(digest.Sum(nil)))
}

func (f *StripeFixture) Put(s StripeSubscription) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s.Invoices = slices.Clone(s.Invoices)
	f.subscriptions[s.ID] = s
}

func (f *StripeFixture) SetReadBarrier(afterRead func(*http.Request, StripeSubscription)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onRead = afterRead
}

func (f *StripeFixture) SetOnComplete(afterComplete func(StripeCheckout)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onComplete = afterComplete
}

func (f *StripeFixture) SetReplayEvents(events []map[string]any, fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replayEvents, f.replayFails = slices.Clone(events), fail
}

func (f *StripeFixture) Reads(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads[id]
}

func (f *StripeFixture) Checkouts() []StripeCheckout {
	f.mu.Lock()
	defer f.mu.Unlock()
	checkouts := slices.Clone(f.checkouts)
	for i := range checkouts {
		checkouts[i].Form = checkouts[i].Form.Clone()
	}
	return checkouts
}

func (f *StripeFixture) Expire(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index := f.checkoutIndex(id); index >= 0 {
		f.checkouts[index].Status = "expired"
	}
}

func (f *StripeFixture) Complete(id string) StripeCheckout {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := f.checkoutIndex(id)
	if index < 0 || f.checkouts[index].Status != "open" {
		return StripeCheckout{}
	}
	checkout := &f.checkouts[index]
	price, interval, period := checkout.Form.Get("line_items[0][price]"), "month", 30*24*time.Hour
	if price == StripeAnnual {
		interval, period = "year", 365*24*time.Hour
	}
	checkout.Status, checkout.Sub = "complete", "sub_checkout_"+strings.TrimPrefix(id, "cs_fixture_")
	f.subscriptions[checkout.Sub] = StripeSubscription{
		ID:        checkout.Sub,
		Account:   checkout.Form.Get("client_reference_id"),
		Status:    "active",
		Price:     price,
		Interval:  interval,
		PeriodEnd: f.now().Add(period),
		Invoices: []StripeInvoice{
			{ID: "in_" + checkout.Sub, Reason: "subscription_create", Price: price, PaidAt: f.now()},
		},
	}
	return *checkout
}

func (f *StripeFixture) FailDeletion(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteFailure = fail
}

func (f *StripeFixture) DeletionCalls() ([]string, map[string]url.Values) {
	f.mu.Lock()
	defer f.mu.Unlock()
	refunds := make(map[string]url.Values, len(f.refunds))
	for key, form := range f.refunds {
		refunds[key] = form.Clone()
	}
	return slices.Clone(f.deletes), refunds
}

func (f *StripeFixture) RedactedCustomers() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.redacted)
}

func (f *StripeFixture) serve(w http.ResponseWriter, r *http.Request) {
	if f.browser && strings.HasPrefix(r.URL.Path, "/fixture-checkout/") {
		f.browserCheckout(w, r)
		return
	}
	key, _, _ := r.BasicAuth()
	if key != stripeKey || r.Header.Get("Stripe-Version") == "" {
		http.Error(w, "missing Stripe key or version", http.StatusUnauthorized)
		return
	}
	switch {
	case r.URL.Path == "/v1/checkout/sessions" && r.Method == http.MethodPost:
		f.createCheckout(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/checkout/sessions/"):
		f.readCheckout(w, r)
	case r.URL.Path == "/v1/billing_portal/sessions" && r.Method == http.MethodPost:
		if err := r.ParseForm(); err != nil || r.Form.Get("customer") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{"url": "https://billing.stripe.com/p/session/fixture"})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/customers/"):
		f.redactCustomer(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/subscriptions/"):
		f.subscription(w, r)
	case r.URL.Path == "/v1/events":
		f.mu.Lock()
		events, fail := slices.Clone(f.replayEvents), f.replayFails
		f.mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]any{"data": events, "has_more": false})
	case r.URL.Path == "/v1/invoices":
		f.invoices(w, r)
	case r.URL.Path == "/v1/invoice_payments":
		invoice := r.URL.Query().Get("invoice")
		api.WriteJSON(w, http.StatusOK, map[string]any{"data": []map[string]any{{
			"id": "inpay_" + invoice, "amount_paid": 7999,
			"payment": map[string]string{"type": "payment_intent", "payment_intent": "pi_" + invoice},
		}}})
	case r.URL.Path == "/v1/refunds" && r.Method == http.MethodPost:
		f.refund(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *StripeFixture) redactCustomer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/customers/")
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, sub := range f.subscriptions {
		if "cus_"+sub.Account == id && sub.Status != "canceled" {
			w.WriteHeader(http.StatusConflict)
			return
		}
	}
	f.redacted = append(f.redacted, id)
	api.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

func (f *StripeFixture) createCheckout(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.Form.Get("mode") != "subscription" ||
		r.Header.Get("Stripe-Version") < "2025-03-31.basil" || r.Form.Get("managed_payments[enabled]") != "true" ||
		r.Form.Get("allow_promotion_codes") != "true" ||
		r.Form.Get("subscription_data[metadata][compozy_account_id]") != r.Form.Get("client_reference_id") ||
		!slices.Contains([]string{StripeAnnual, StripeMonthly}, r.Form.Get("line_items[0][price]")) {
		http.Error(w, "invalid Checkout parameters", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	id := fmt.Sprintf("cs_fixture_%d", len(f.checkouts)+1)
	target := "https://checkout.stripe.com/c/pay/" + id
	if f.browser {
		target = "https://checkout.stripe.com/fixture-checkout/" + id
	}
	f.checkouts = append(f.checkouts, StripeCheckout{ID: id, Form: r.Form.Clone(), Status: "open"})
	api.WriteJSON(w, http.StatusOK, map[string]any{
		"id": id, "url": target, "status": "open", "expires_at": f.now().Add(24 * time.Hour).Unix(),
	})
}

func (f *StripeFixture) checkoutIndex(id string) int {
	return slices.IndexFunc(f.checkouts, func(c StripeCheckout) bool { return c.ID == id })
}

func (f *StripeFixture) readCheckout(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tail := strings.TrimPrefix(r.URL.Path, "/v1/checkout/sessions/")
	id, action, _ := strings.Cut(tail, "/")
	index := f.checkoutIndex(id)
	if index < 0 {
		http.NotFound(w, r)
		return
	}
	checkout := &f.checkouts[index]
	if action == "expire" && r.Method == http.MethodPost && checkout.Status == "open" {
		checkout.Status = "expired"
	}
	payment := "unpaid"
	if checkout.Status == "complete" {
		payment = "paid"
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{
		"id": id, "status": checkout.Status, "client_reference_id": checkout.Form.Get("client_reference_id"),
		"payment_status": payment, "subscription": checkout.Sub,
	})
}

func (f *StripeFixture) subscription(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/subscriptions/")
	f.mu.Lock()
	s, ok := f.subscriptions[id]
	if r.Method == http.MethodDelete {
		f.deletes = append(f.deletes, id)
		if f.deleteFailure {
			f.mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if ok {
			s.Status = "canceled"
			f.subscriptions[id] = s
		}
	} else {
		f.reads[id]++
	}
	barrier := f.onRead
	f.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	if barrier != nil && r.Method == http.MethodGet {
		barrier(r, s)
	}
	latest := ""
	if len(s.Invoices) > 0 {
		latest = s.Invoices[0].ID
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{
		"id": s.ID, "status": s.Status, "customer": "cus_" + s.Account, "latest_invoice": latest,
		"metadata":             map[string]string{"compozy_account_id": s.Account},
		"cancellation_details": map[string]string{"reason": s.CancellationReason},
		"items": map[string]any{"data": []map[string]any{{
			"current_period_end": s.PeriodEnd.Unix(),
			"price":              map[string]any{"id": s.Price, "recurring": map[string]string{"interval": s.Interval}},
		}}},
	})
}

func (f *StripeFixture) invoices(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sub := f.subscriptions[r.URL.Query().Get("subscription")]
	rows := make([]map[string]any, 0, len(sub.Invoices))
	for _, invoice := range sub.Invoices {
		rows = append(rows, map[string]any{
			"id": invoice.ID, "amount_paid": 7999, "billing_reason": invoice.Reason,
			"status_transitions": map[string]int64{"paid_at": invoice.PaidAt.Unix()},
			"lines": map[string]any{"data": []map[string]any{{
				"pricing": map[string]any{"price_details": map[string]string{"price": invoice.Price}},
			}}},
		})
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{"data": rows})
}

func (f *StripeFixture) refund(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if err := r.ParseForm(); err != nil || key == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refunds[key] = r.Form.Clone()
	api.WriteJSON(w, http.StatusOK, map[string]string{"id": "re_fixture", "status": "succeeded"})
}

func (f *StripeFixture) browserCheckout(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/fixture-checkout/")
	if r.Method == http.MethodPost {
		checkout := f.Complete(id)
		if checkout.ID == "" {
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		callback := f.onComplete
		f.mu.Unlock()
		if callback != nil {
			callback(checkout)
		}
		next := strings.ReplaceAll(checkout.Form.Get("success_url"), "{CHECKOUT_SESSION_ID}", id)
		// #nosec G710 -- the fixture uses the return URL supplied by the real checkout service.
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.checkoutIndex(id) < 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><html lang="en"><title>Stripe Checkout fixture</title>
		<form method="post"><button>Complete payment</button></form></html>`)
}
