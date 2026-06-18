package recovery

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

type persistedRunResult struct {
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	Status        RunStatus    `json:"status"`
	ArtifactsDir  string       `json:"artifacts_dir"`
	ResultPath    string       `json:"result_path,omitempty"`
	Jobs          []JobOutcome `json:"jobs"`
}

// ReadRunOutcome reads and validates the run's persisted result.json.
//
// A nil outcome plus a non-nil error is fail-safe: callers should skip
// recovery when the persisted result contract is missing, corrupt, or
// unsupported.
func ReadRunOutcome(artifacts model.RunArtifacts) (*RunOutcome, error) {
	resultPath := strings.TrimSpace(artifacts.ResultPath)
	if resultPath == "" {
		return nil, fmt.Errorf("read run outcome: result path is empty")
	}
	payload, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("read run result %q: %w", resultPath, err)
	}

	var result persistedRunResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("decode run result %q: %w", resultPath, err)
	}
	if result.SchemaVersion != ResultSchemaVersion {
		return nil, fmt.Errorf(
			"unsupported run result schema_version %d in %q: want %d",
			result.SchemaVersion,
			resultPath,
			ResultSchemaVersion,
		)
	}
	outcome := &RunOutcome{
		RunID:        result.RunID,
		Status:       result.Status,
		ArtifactsDir: result.ArtifactsDir,
		ResultPath:   result.ResultPath,
		Jobs:         append([]JobOutcome(nil), result.Jobs...),
	}
	return outcome, nil
}
