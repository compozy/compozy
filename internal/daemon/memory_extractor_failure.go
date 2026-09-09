package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	"github.com/compozy/compozy/internal/redact"
	"github.com/google/uuid"
)

func (e *forkedMemoryExtractor) recordExtractionFailure(turn memcontract.TurnRecord, output string, cause error) error {
	if e.failuresDir == "" {
		return nil
	}
	if err := os.MkdirAll(e.failuresDir, 0o700); err != nil {
		return fmt.Errorf("daemon: create extraction failure directory: %w", err)
	}
	now := e.nowUTC()
	report := extractorFailureReport{
		Stage:       "extract",
		Source:      "memory extractor child",
		Error:       redact.String(cause.Error()),
		Content:     redact.String(output),
		RecordedAt:  now.Format(time.RFC3339Nano),
		SessionID:   turn.SessionID,
		WorkspaceID: turn.WorkspaceID,
		AgentName:   turn.AgentID,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("daemon: encode extraction failure: %w", err)
	}
	path := filepath.Join(e.failuresDir, "extraction-"+uuid.NewString()+".json")
	if err := fileutil.AtomicWriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("daemon: write extraction failure: %w", err)
	}
	return nil
}
