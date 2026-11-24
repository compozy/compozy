//go:build integration

package remotetest

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/email"
)

type ResendFixture struct {
	Client *email.Resend

	mu             sync.Mutex
	contacts       map[string]resendContact
	messages       map[string]ResendMessage
	contactDeletes int
	failDeletes    int
}

type resendContact struct {
	Email      string            `json:"email"`
	Name       string            `json:"first_name"`
	Properties map[string]string `json:"properties"`
}

type ResendMessage struct {
	Recipients []string `json:"to"`
	Subject    string   `json:"subject"`
	Text       string   `json:"text"`
}

func StartResend(t Fixture) *ResendFixture {
	t.Helper()
	f := &ResendFixture{contacts: make(map[string]resendContact), messages: make(map[string]ResendMessage)}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	client, err := email.NewResend(email.ResendConfig{APIKey: "fixture-resend", SegmentID: "news", Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	f.Client = client
	return f
}

func (f *ResendFixture) FailContactDeletes(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failDeletes = count
}

func (f *ResendFixture) ContactDeletes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.contactDeletes
}

func (f *ResendFixture) UnsubscribeURL(address string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.contacts[address].Properties["compozy_unsubscribe_url"]
}

func (f *ResendFixture) Messages() []ResendMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ResendMessage, 0, len(f.messages))
	for _, message := range f.messages {
		message.Recipients = slices.Clone(message.Recipients)
		out = append(out, message)
	}
	return out
}

func (f *ResendFixture) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer fixture-resend" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/contacts":
		var contact resendContact
		if err := json.UnmarshalRead(r.Body, &contact); err != nil || contact.Email == "" ||
			contact.Name == "" || contact.Properties["compozy_unsubscribe_url"] == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.contacts[contact.Email] = contact
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/contacts/"):
		f.contactDeletes++
		if f.failDeletes > 0 {
			f.failDeletes--
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		address := strings.TrimPrefix(r.URL.Path, "/contacts/")
		if _, ok := f.contacts[address]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(f.contacts, address)
	case r.Method == http.MethodPost && r.URL.Path == "/emails":
		var message ResendMessage
		key := r.Header.Get("Idempotency-Key")
		if err := json.UnmarshalRead(r.Body, &message); err != nil || key == "" || len(message.Recipients) != 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, sent := f.messages[key]; !sent {
			f.messages[key] = message
		}
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"id": "fixture"})
}
