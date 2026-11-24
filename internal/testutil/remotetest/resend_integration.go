//go:build integration

package remotetest

import (
	"encoding/json/v2"
	"maps"
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
	contactWrites  int
	onSend         func(ResendMessage)
}

const (
	resendSendingKey    = "fixture-resend"
	resendContactsKey   = "fixture-resend-contacts"
	ResendPostalAddress = "123 Test Lane, Example City"
)

type resendContact struct {
	Email      string            `json:"email"`
	Name       string            `json:"first_name"`
	Properties map[string]string `json:"properties"`
	Segments   []struct {
		ID string `json:"id"`
	} `json:"segments"`
	Unsubscribed bool `json:"unsubscribed"`
}

type ResendMessage struct {
	From       string            `json:"from"`
	Headers    map[string]string `json:"headers"`
	Recipients []string          `json:"to"`
	Subject    string            `json:"subject"`
	Text       string            `json:"text"`
}

func StartResend(t Fixture) *ResendFixture {
	t.Helper()
	f := &ResendFixture{contacts: make(map[string]resendContact), messages: make(map[string]ResendMessage)}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	client, err := email.NewResend(email.ResendConfig{
		APIKey: resendSendingKey, ContactAPIKey: resendContactsKey,
		SegmentID: "news", Endpoint: server.URL, PostalAddress: ResendPostalAddress,
	})
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

func (f *ResendFixture) ContactWrites() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.contactWrites
}

func (f *ResendFixture) ContactName(address string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.contacts[address].Name
}

func (f *ResendFixture) UnsubscribeURL(address string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.contacts[address].Properties["compozy_unsubscribe_url"]
}

func (f *ResendFixture) SetOnSend(fn func(ResendMessage)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onSend = fn
}

func (f *ResendFixture) Messages() []ResendMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ResendMessage, 0, len(f.messages))
	for _, message := range f.messages {
		message.Recipients = slices.Clone(message.Recipients)
		message.Headers = maps.Clone(message.Headers)
		out = append(out, message)
	}
	return out
}

func (f *ResendFixture) serve(w http.ResponseWriter, r *http.Request) {
	key := resendContactsKey
	if r.URL.Path == "/emails" {
		key = resendSendingKey
	}
	if r.Header.Get("Authorization") != "Bearer "+key {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/contacts":
		var contact resendContact
		if err := json.UnmarshalRead(r.Body, &contact); err != nil || contact.Email == "" ||
			!validResendContact(contact) || len(contact.Segments) != 1 || contact.Segments[0].ID != "news" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, exists := f.contacts[contact.Email]; exists {
			w.WriteHeader(http.StatusConflict)
			return
		}
		f.contactWrites++
		f.contacts[contact.Email] = contact
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/contacts/"):
		address := strings.TrimPrefix(r.URL.Path, "/contacts/")
		current, ok := f.contacts[address]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var contact resendContact
		if err := json.UnmarshalRead(r.Body, &contact); err != nil || !validResendContact(contact) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		contact.Email, contact.Segments = address, current.Segments
		f.contactWrites++
		f.contacts[address] = contact
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/segments/news"):
		address := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/contacts/"), "/segments/news")
		if _, ok := f.contacts[address]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
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
			if f.onSend != nil {
				f.onSend(message)
			}
			f.messages[key] = message
		}
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"id": "fixture"})
}

func validResendContact(contact resendContact) bool {
	return contact.Name != "" && !contact.Unsubscribed && contact.Properties["compozy_unsubscribe_url"] != "" &&
		contact.Properties["compozy_postal_address"] == ResendPostalAddress
}
