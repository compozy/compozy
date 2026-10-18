package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/testutil/acpmock"
)

func (a *mockAgent) writeProtocolDiagnostics(
	method string,
	sessionID string,
	configOptionID string,
	configOptionValue string,
) error {
	return a.writeDiagnostics(acpmock.DiagnosticsRecord{
		AgentName:         a.agent.Name,
		SessionID:         strings.TrimSpace(sessionID),
		ProtocolMethod:    strings.TrimSpace(method),
		ConfigOptionID:    strings.TrimSpace(configOptionID),
		ConfigOptionValue: strings.TrimSpace(configOptionValue),
	})
}

func (a *mockAgent) writeDiagnostics(record acpmock.DiagnosticsRecord) (err error) {
	if strings.TrimSpace(a.diagnosticsPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(a.diagnosticsPath), 0o755); err != nil {
		return fmt.Errorf("create diagnostics directory %q: %w", filepath.Dir(a.diagnosticsPath), err)
	}
	file, err := os.OpenFile(a.diagnosticsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open diagnostics %q: %w", a.diagnosticsPath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close diagnostics %q: %w", a.diagnosticsPath, closeErr))
		}
	}()

	record.CompozySessionID = strings.TrimSpace(os.Getenv("COMPOZY_SESSION_ID"))
	record.Provider = strings.TrimSpace(a.provider)
	if record.Provider == "" {
		record.Provider = strings.TrimSpace(a.agent.Provider)
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode diagnostics: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write diagnostics %q: %w", a.diagnosticsPath, err)
	}
	return nil
}

func (a *mockAgent) HandleExtensionMethod(_ context.Context, method string, params json.RawMessage) (any, error) {
	if method != "_session/steering" || a.agent.SteerOutcome == "" {
		return nil, &acpsdk.RequestError{Code: -32601, Message: "Method not found"}
	}
	var request acpsdk.PromptRequest
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, err
	}
	if err := a.writeDiagnostics(
		acpmock.DiagnosticsRecord{
			AgentName:      a.agent.Name,
			SessionID:      string(request.SessionId),
			ProtocolMethod: method,
			Prompt:         extractPromptText(request.Prompt),
		},
	); err != nil {
		return nil, err
	}
	a.mu.Lock()
	if session := a.sessions[string(request.SessionId)]; session != nil && session.steerAccepted != nil {
		select {
		case <-session.steerAccepted:
		default:
			close(session.steerAccepted)
		}
	}
	a.mu.Unlock()
	return map[string]string{"outcome": a.agent.SteerOutcome}, nil
}
