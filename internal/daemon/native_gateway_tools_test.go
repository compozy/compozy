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
			status: gateway.Status{Enabled: true},
			audit:  gateway.AuditReport{Ran: true, NoFindings: true, LocalOnly: true},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Gateway: service,
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
		}
	})

	t.Run("Should deny session mutations below approve-all", func(t *testing.T) {
		t.Parallel()

		service := &nativeGatewayService{}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Gateway: service,
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
			Gateway: service,
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
