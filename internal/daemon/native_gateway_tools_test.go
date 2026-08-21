package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestDaemonNativeGatewayTool(t *testing.T) {
	t.Parallel()

	t.Run("Should report an unavailable gateway dependency", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{}, nativeApproveAllPolicyInputs())
		_, err := registry.Call(t.Context(), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDGateway, Input: json.RawMessage(`{"action":"status"}`),
		})
		if !errors.Is(err, toolspkg.ErrToolUnavailable) {
			t.Fatalf("Registry.Call(gateway status) error = %v, want ErrToolUnavailable", err)
		}
	})

	t.Run("Should allow redacted reads in approve-reads mode", func(t *testing.T) {
		t.Parallel()

		service := &nativeGatewayService{
			status: gateway.Status{
				Enabled: true,
				Devices: []gateway.DeviceSession{{ID: "device-1", Name: "Browser"}},
			},
			audit: gateway.AuditReport{Ran: true, NoFindings: true, LocalOnly: true},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Gateway:  func() core.GatewayService { return service },
			Sessions: nativeNetworkTestSessionManager("workspace-a"),
			GatewayPermissionMode: func(context.Context, string) (string, error) {
				return string(toolspkg.PermissionModeDenyAll), nil
			},
		}, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    true,
		})

		for _, action := range []string{gatewayActionStatus, gatewayActionAudit, gatewayActionDeviceList} {
			result, err := registry.Call(t.Context(), toolspkg.Scope{
				SessionID: "session-reads", WorkspaceID: "workspace-a",
			}, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDGateway, Input: json.RawMessage(`{"action":"` + action + `"}`),
			})
			if err != nil {
				t.Fatalf("Registry.Call(gateway %s) error = %v", action, err)
			}
			var payload nativeGatewayResult
			if err := json.Unmarshal(result.Structured, &payload); err != nil {
				t.Fatalf("Unmarshal(gateway %s) error = %v", action, err)
			}
			if payload.Action != action || !payload.Redacted {
				t.Fatalf("gateway payload = %#v, want action %q and redacted=true", payload, action)
			}
			switch action {
			case gatewayActionStatus:
				if payload.Status == nil || !payload.Status.Enabled {
					t.Fatalf("gateway status payload = %#v, want enabled", payload.Status)
				}
			case gatewayActionAudit:
				if payload.Audit == nil || !payload.Audit.Ran || !payload.Audit.NoFindings || !payload.Audit.LocalOnly {
					t.Fatalf("gateway audit payload = %#v, want local-only no-findings report", payload.Audit)
				}
			case gatewayActionDeviceList:
				if payload.Devices == nil || len(*payload.Devices) != 1 || (*payload.Devices)[0].ID != "device-1" {
					t.Fatalf("gateway device payload = %#v, want device-1", payload.Devices)
				}
			}
		}
	})

	t.Run("Should deny session mutations below approve-all", func(t *testing.T) {
		t.Parallel()

		service := &nativeGatewayService{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Gateway:  func() core.GatewayService { return service },
			Sessions: nativeNetworkTestSessionManager(""),
			GatewayPermissionMode: func(context.Context, string) (string, error) {
				return string(toolspkg.PermissionModeApproveReads), nil
			},
		}, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    true,
		})
		_, err := registry.Call(t.Context(), toolspkg.Scope{SessionID: "session-reads"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDGateway,
			Input: json.RawMessage(
				`{"action":"surface_set","surface":"operator_ui","tier":"private","desired":"enabled"}`,
			),
		})
		if !errors.Is(err, toolspkg.ErrToolDenied) {
			t.Fatalf("Registry.Call(gateway surface_set) error = %v, want ErrToolDenied", err)
		}
		if service.surfaceSetCalls != 0 {
			t.Fatalf("SetSurfaceExposure calls = %d, want 0", service.surfaceSetCalls)
		}
	})

	t.Run("Should allow approve-all session and operator mutations", func(t *testing.T) {
		t.Parallel()

		service := &nativeGatewayService{status: gateway.Status{Enabled: true}}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Gateway:  func() core.GatewayService { return service },
			Sessions: nativeNetworkTestSessionManager(""),
			GatewayPermissionMode: func(_ context.Context, sessionID string) (string, error) {
				if strings.TrimSpace(sessionID) != "session-all" {
					t.Fatalf("permission session id = %q, want session-all", sessionID)
				}
				return string(toolspkg.PermissionModeApproveAll), nil
			},
		}, nativeApproveAllPolicyInputs())
		input := json.RawMessage(
			`{"action":"surface_set","surface":"operator_ui","tier":"private","desired":"enabled"}`,
		)
		for _, scope := range []toolspkg.Scope{{SessionID: "session-all"}, {Operator: true}} {
			if _, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDGateway, Input: input,
			}); err != nil {
				t.Fatalf("Registry.Call(gateway surface_set) error = %v", err)
			}
		}
		if service.surfaceSetCalls != 2 {
			t.Fatalf("SetSurfaceExposure calls = %d, want 2", service.surfaceSetCalls)
		}
	})

	// Invariant: native gateway calls resolve the service created after tool-registry boot.
	// Owning layer: daemon composition root.
	// Canonical suite: TestDaemonNativeGatewayTool.
	t.Run("Should resolve the gateway service lazily after registry construction", func(t *testing.T) {
		t.Parallel()

		var service core.GatewayService
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Gateway: func() core.GatewayService { return service },
		}, nativeApproveAllPolicyInputs())
		service = &nativeGatewayService{
			audit: gateway.AuditReport{Ran: true, NoFindings: true, LocalOnly: true},
		}
		result, err := registry.Call(t.Context(), toolspkg.Scope{Operator: true}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDGateway,
			Input:  json.RawMessage(`{"action":"audit"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(gateway audit) error = %v", err)
		}
		var payload nativeGatewayResult
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("Unmarshal(gateway audit) error = %v", err)
		}
		if payload.Audit == nil || !payload.Audit.Ran || !payload.Audit.NoFindings || !payload.Audit.LocalOnly {
			t.Fatalf("gateway audit payload = %#v, want local-only no-findings report", payload.Audit)
		}
	})
}

type nativeGatewayService struct {
	core.GatewayService
	status          gateway.Status
	audit           gateway.AuditReport
	surfaceSetCalls int
}

func (s *nativeGatewayService) Status(context.Context) (gateway.Status, error) {
	return s.status, nil
}

func (s *nativeGatewayService) Audit(context.Context) (gateway.AuditReport, error) {
	return s.audit, nil
}

func (s *nativeGatewayService) ListDevices(context.Context) ([]gateway.DeviceSession, error) {
	return s.status.Devices, nil
}

func (s *nativeGatewayService) SetSurfaceExposure(
	_ context.Context,
	_ gateway.SurfaceExposureRequest,
) (gateway.Status, error) {
	s.surfaceSetCalls++
	return s.status, nil
}
