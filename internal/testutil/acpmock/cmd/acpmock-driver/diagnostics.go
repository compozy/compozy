package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/testutil/acpmock"
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

	record.AGHSessionID = strings.TrimSpace(os.Getenv("AGH_SESSION_ID"))
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode diagnostics: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write diagnostics %q: %w", a.diagnosticsPath, err)
	}
	return nil
}
