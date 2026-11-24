//go:build integration

package main

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/cloudflare"
)

type hostedFakeCloudflare struct {
	f       *fixture
	mu      sync.Mutex
	created int
	tunnels map[string]hostedFakeTunnel
	dns     map[string]hostedFakeDNS
	client  *cloudflare.Client
}

type hostedFakeTunnel struct {
	ID   string              `json:"id"`
	Name string              `json:"name"`
	Edge hostedFakeConnector `json:"-"`
}

type hostedFakeDNS struct {
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

func newHostedFakeCloudflare(f *fixture) *hostedFakeCloudflare {
	f.Helper()
	fake := &hostedFakeCloudflare{
		f:       f,
		tunnels: make(map[string]hostedFakeTunnel),
		dns:     make(map[string]hostedFakeDNS),
	}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	f.Cleanup(server.Close)
	client, err := cloudflare.New(cloudflare.Config{
		AccountID: "fake-account",
		ZoneID:    "fake-zone",
		Token:     "fake-cloudflare-key",
		Domain:    cloudflare.ProductionDomain,
		BaseURL:   server.URL,
	})
	if err != nil {
		f.Fatal(err)
	}
	fake.client = client
	return fake
}

func (c *hostedFakeCloudflare) serve(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer fake-cloudflare-key" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if path, matched := strings.CutPrefix(r.URL.Path, "/accounts/fake-account/cfd_tunnel"); matched {
		c.tunnel(w, r, path)
		return
	}
	if path, matched := strings.CutPrefix(r.URL.Path, "/zones/fake-zone/dns_records"); matched {
		c.record(w, r, path)
		return
	}
	c.f.Errorf("unexpected Cloudflare request: %s %s", r.Method, r.URL)
	w.WriteHeader(http.StatusBadRequest)
}

func (c *hostedFakeCloudflare) tunnel(w http.ResponseWriter, r *http.Request, path string) {
	if path == "" {
		c.tunnelCollection(w, r)
		return
	}
	id, action, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	tunnel, ok := c.tunnels[id]
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
			c.f.Errorf("unexpected tunnel ingress configuration: %+v, %v", cfg, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		first := cfg.Config.Ingress[0]
		tunnel.Edge = hostedFakeConnector{Hostname: first.Hostname, Origin: first.Service}
		c.tunnels[id] = tunnel
		hostedFakeReply(w, nil)
	case "GET token":
		hostedFakeReply(w, "fixture-connector")
	case "DELETE connections":
		hostedFakeReply(w, nil)
	case "DELETE ":
		delete(c.tunnels, id)
		hostedFakeReply(w, tunnel)
	default:
		c.f.Errorf("unexpected Cloudflare tunnel request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (c *hostedFakeCloudflare) tunnelCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows := []hostedFakeTunnel{}
		for _, tunnel := range c.tunnels {
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
		if err := json.UnmarshalRead(r.Body, &request); err != nil ||
			request.Mode != "cloudflare" || request.Name == "" {
			c.f.Errorf("unexpected tunnel creation: %+v, %v", request, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		c.created++
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", c.created)
		tunnel := hostedFakeTunnel{ID: id, Name: request.Name}
		c.tunnels[id] = tunnel
		hostedFakeReply(w, tunnel)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *hostedFakeCloudflare) record(w http.ResponseWriter, r *http.Request, path string) {
	switch r.Method {
	case http.MethodGet:
		rows := []hostedFakeDNS{}
		for _, record := range c.dns {
			if record.Name == r.URL.Query().Get("name.exact") {
				rows = append(rows, record)
			}
		}
		hostedFakeReply(w, rows)
	case http.MethodPost:
		var record hostedFakeDNS
		if err := json.UnmarshalRead(r.Body, &record); err != nil || !record.Proxied || record.Type != "CNAME" {
			c.f.Errorf("unexpected DNS creation: %+v, %v", record, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		record.ID = fmt.Sprintf("record-%d", c.created)
		c.dns[record.ID] = record
		hostedFakeReply(w, record)
	case http.MethodDelete:
		delete(c.dns, strings.TrimPrefix(path, "/"))
		hostedFakeReply(w, nil)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *hostedFakeCloudflare) target(hostname string) hostedFakeConnector {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, tunnel := range c.tunnels {
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
