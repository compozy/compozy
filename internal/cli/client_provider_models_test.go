package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
)

func TestProviderModelClientRoutesGlobalActions(t *testing.T) {
	t.Parallel()

	t.Run("Should route refresh without a provider to the global endpoint", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.Path != "/api/model-catalog/models/refresh" {
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				return newHTTPResponse(http.StatusOK, `{"sources":[]}`), nil
			})},
		}
		if _, err := client.RefreshProviderModels(
			context.Background(),
			"  ",
			ProviderModelRefreshRequest{},
		); err != nil {
			t.Fatalf("RefreshProviderModels(blank provider) error = %v", err)
		}
	})

	t.Run("Should route status without a provider to the global endpoint", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/api/model-catalog/sources/status" {
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				return newHTTPResponse(http.StatusOK, `{"sources":[]}`), nil
			})},
		}
		if _, err := client.ProviderModelStatus(context.Background(), " "); err != nil {
			t.Fatalf("ProviderModelStatus(blank provider) error = %v", err)
		}
	})
}

func TestProviderModelClientRequiresProviderIDForCuration(t *testing.T) {
	t.Parallel()

	client := &unixSocketClient{}

	t.Run("Should reject blank provider IDs for curation", func(t *testing.T) {
		t.Parallel()

		_, err := client.CurateProviderModel(context.Background(), " ", ProviderModelCurationRequest{})
		if err == nil {
			t.Fatal("CurateProviderModel(blank provider) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "provider_id is required") {
			t.Fatalf("CurateProviderModel(blank provider) error = %v, want provider_id validation", err)
		}
	})

	t.Run("Should POST curation request and decode response", func(t *testing.T) {
		t.Parallel()

		hidden := false
		featured := true
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost ||
					req.URL.Path != "/api/model-catalog/providers/codex/models/curate" {
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, fmt.Errorf("read curation body: %w", err)
				}
				var payload ProviderModelCurationRequest
				if err := json.Unmarshal(body, &payload); err != nil {
					return nil, fmt.Errorf("decode curation body: %w", err)
				}
				if payload.ModelID != "gpt-5.6-sol" ||
					payload.Hidden == nil || *payload.Hidden ||
					payload.Featured == nil || !*payload.Featured {
					return nil, fmt.Errorf("unexpected curation payload: %#v", payload)
				}
				return newHTTPResponse(http.StatusOK, `{
					"model":{
						"provider_id":"codex",
						"model_id":"gpt-5.6-sol",
						"available":true,
						"availability_state":"available",
						"stale":false,
						"sources":[],
						"curated":true,
						"deprecated":false,
						"hidden":false,
						"featured":true
					},
					"apply":{
						"applied":true,
						"lifecycle":"live",
						"apply_record_id":"apply-1",
						"active_generation":1,
						"active_config_hash":"hash-1",
						"next_action":"none"
					}
				}`), nil
			})},
		}
		record, err := client.CurateProviderModel(
			context.Background(),
			"codex",
			ProviderModelCurationRequest{
				ModelID:  "gpt-5.6-sol",
				Hidden:   &hidden,
				Featured: &featured,
			},
		)
		if err != nil {
			t.Fatalf("CurateProviderModel() error = %v", err)
		}
		if record.Model.ProviderID != "codex" ||
			record.Model.ModelID != "gpt-5.6-sol" ||
			!record.Model.Curated ||
			!record.Model.Featured ||
			!record.Apply.Applied {
			t.Fatalf("CurateProviderModel() record = %#v, want curated featured applied model", record)
		}
		if record.Apply.Lifecycle != contract.SettingsApplyLifecycleLive {
			t.Fatalf("apply lifecycle = %q, want live", record.Apply.Lifecycle)
		}
	})
}

func TestProviderModelListValues(t *testing.T) {
	t.Parallel()

	t.Run("Should encode view filter into query values", func(t *testing.T) {
		t.Parallel()

		values := providerModelListValues(ProviderModelListQuery{View: providerModelViewAll})
		if got, want := values.Get("view"), providerModelViewAll; got != want {
			t.Fatalf("view query = %q, want %q", got, want)
		}
	})
}
