//go:build integration

package api_test

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/cloudflare"
)

type hostedFakeCloudflare struct {
	t       *testing.T
	mu      sync.Mutex
	created int
	deleted int
	tunnels map[string]fakeCloudflareTunnel
	dns     map[string]fakeCloudflareDNS
	client  *cloudflare.Client
}

type fakeCloudflareTunnel struct {
	ID   string              `json:"id"`
	Name string              `json:"name"`
	Edge hostedFakeConnector `json:"-"`
}

type fakeCloudflareDNS struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

type hostedFakeConnector struct {
	Hostname string `json:"hostname"`
	Origin   string `json:"origin"`
}

func newHostedFakeCloudflare(t *testing.T) *hostedFakeCloudflare {
	t.Helper()
	fake := &hostedFakeCloudflare{
		t: t, tunnels: make(map[string]fakeCloudflareTunnel), dns: make(map[string]fakeCloudflareDNS),
	}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(server.Close)
	client, err := cloudflare.New(cloudflare.Config{
		AccountID: "fake-account", ZoneID: "fake-zone", Token: "fake-cloudflare-key",
		Domain: cloudflare.ProductionDomain, BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	fake.client = client
	return fake
}

func (f *hostedFakeCloudflare) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer fake-cloudflare-key" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if path, matched := strings.CutPrefix(r.URL.Path, "/accounts/fake-account/cfd_tunnel"); matched {
		f.tunnel(w, r, path)
		return
	}
	if path, matched := strings.CutPrefix(r.URL.Path, "/zones/fake-zone/dns_records"); matched {
		f.record(w, r, path)
		return
	}
	f.t.Errorf("unexpected Cloudflare request: %s %s", r.Method, r.URL)
	w.WriteHeader(http.StatusBadRequest)
}

func (f *hostedFakeCloudflare) tunnel(w http.ResponseWriter, r *http.Request, path string) {
	if path == "" {
		f.tunnelCollection(w, r)
		return
	}
	id, action, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	tunnel, ok := f.tunnels[id]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method + " " + action {
	case "GET ":
		hostedFakeReply(w, tunnel)
	case "PUT configurations":
		var cfg struct {
			Config struct {
				Ingress []struct {
					Hostname string `json:"hostname"`
					Service  string `json:"service"`
				} `json:"ingress"`
			} `json:"config"`
		}
		if err := json.UnmarshalRead(r.Body, &cfg); err != nil || len(cfg.Config.Ingress) != 2 ||
			cfg.Config.Ingress[1].Service != "http_status:404" || cfg.Config.Ingress[1].Hostname != "" {
			f.t.Errorf("unexpected tunnel ingress configuration: %+v, %v", cfg, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		first := cfg.Config.Ingress[0]
		tunnel.Edge = hostedFakeConnector{Hostname: first.Hostname, Origin: first.Service}
		f.tunnels[id] = tunnel
		hostedFakeReply(w, nil)
	case "GET token":
		token, err := json.Marshal(tunnel.Edge)
		if err != nil {
			f.t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		hostedFakeReply(w, string(token))
	case "DELETE connections":
		hostedFakeReply(w, nil)
	case "DELETE ":
		delete(f.tunnels, id)
		f.deleted++
		hostedFakeReply(w, tunnel)
	default:
		f.t.Errorf("unexpected Cloudflare tunnel request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (f *hostedFakeCloudflare) tunnelCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows := []fakeCloudflareTunnel{}
		for _, tunnel := range f.tunnels {
			if tunnel.Name == r.URL.Query().Get("name") {
				rows = append(rows, tunnel)
			}
		}
		hostedFakeReply(w, rows)
	case http.MethodPost:
		var request struct {
			Name string `json:"name"`
			Mode string `json:"config_src"`
		}
		if err := json.UnmarshalRead(
			r.Body,
			&request,
		); err != nil || request.Mode != "cloudflare" ||
			request.Name == "" {
			f.t.Errorf("unexpected tunnel creation: %+v, %v", request, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.created++
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", f.created)
		tunnel := fakeCloudflareTunnel{ID: id, Name: request.Name}
		f.tunnels[id] = tunnel
		hostedFakeReply(w, tunnel)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *hostedFakeCloudflare) record(w http.ResponseWriter, r *http.Request, path string) {
	switch r.Method {
	case http.MethodGet:
		rows := []fakeCloudflareDNS{}
		for _, record := range f.dns {
			if record.Name == r.URL.Query().Get("name.exact") {
				rows = append(rows, record)
			}
		}
		hostedFakeReply(w, rows)
	case http.MethodPost:
		var record fakeCloudflareDNS
		if err := json.UnmarshalRead(r.Body, &record); err != nil || !record.Proxied || record.Type != "CNAME" {
			f.t.Errorf("unexpected DNS creation: %+v, %v", record, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		record.ID = fmt.Sprintf("record-%d", f.created)
		f.dns[record.ID] = record
		hostedFakeReply(w, record)
	case http.MethodDelete:
		delete(f.dns, strings.TrimPrefix(path, "/"))
		hostedFakeReply(w, nil)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *hostedFakeCloudflare) counts() (created, deleted int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.created, f.deleted
}

func (f *hostedFakeCloudflare) target(hostname string) hostedFakeConnector {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, tunnel := range f.tunnels {
		if tunnel.Edge.Hostname == hostname {
			return tunnel.Edge
		}
	}
	return hostedFakeConnector{}
}

func hostedFakeReply(w http.ResponseWriter, result any) {
	api.WriteJSON(w, http.StatusOK, struct {
		Success bool `json:"success"`
		Result  any  `json:"result"`
	}{Success: true, Result: result})
}
