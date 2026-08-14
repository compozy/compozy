package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
)

const cursorACPDiscoveryAgentName = "compozy-model-catalog"

// CursorACPModelProbe reads model options advertised by a short-lived Cursor ACP session.
type CursorACPModelProbe interface {
	InspectCursorModels(context.Context, CursorACPModelProbeRequest) ([]acp.SessionConfigOption, error)
}

// CursorACPModelProbeRequest captures one Cursor ACP discovery invocation.
type CursorACPModelProbeRequest struct {
	ProviderID string
	Command    string
	Cwd        string
	Env        []string
	Timeout    time.Duration
}

// ACPModelProbe reads advertised model options through ACP.
type ACPModelProbe struct{}

var _ CursorACPModelProbe = ACPModelProbe{}

// InspectCursorModels creates a short-lived ACP session without changing its configuration.
func (ACPModelProbe) InspectCursorModels(
	ctx context.Context,
	req CursorACPModelProbeRequest,
) ([]acp.SessionConfigOption, error) {
	options, err := acp.InspectSessionConfigOptions(ctx, acp.SessionInspectionRequest{
		AgentName: cursorACPDiscoveryAgentName,
		Command:   strings.TrimSpace(req.Command),
		Cwd:       strings.TrimSpace(req.Cwd),
		Env:       append([]string(nil), req.Env...),
	})
	if err != nil {
		return nil, fmt.Errorf("model catalog: inspect Cursor ACP model options: %w", err)
	}
	return options, nil
}

func (s *LiveProviderSource) listCursorACP(
	ctx context.Context,
	command string,
	env []string,
	timeout time.Duration,
	now time.Time,
) ([]ModelRow, error) {
	if s.cursorACPProbe == nil {
		return nil, errors.New("model catalog: Cursor ACP model probe is required")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("model catalog: resolve Cursor ACP discovery working directory: %w", err)
	}
	options, err := s.cursorACPProbe.InspectCursorModels(ctx, CursorACPModelProbeRequest{
		ProviderID: s.providerID,
		Command:    strings.TrimSpace(command),
		Cwd:        cwd,
		Env:        append([]string(nil), env...),
		Timeout:    timeout,
	})
	if err != nil {
		return nil, err
	}
	modelOption, ok := acp.ModelConfigOption(options)
	if !ok {
		return nil, errors.New("model catalog: Cursor ACP session did not advertise a model select option")
	}
	rows := cursorACPModelRows(s.providerID, modelOption, now)
	if len(rows) == 0 {
		return nil, errors.New("model catalog: Cursor ACP model option did not advertise model values")
	}
	return rows, nil
}

func cursorACPModelRows(
	providerID string,
	modelOption acp.SessionConfigOption,
	now time.Time,
) []ModelRow {
	rows := make([]ModelRow, 0, len(modelOption.Values))
	seen := make(map[string]struct{}, len(modelOption.Values))
	for _, value := range modelOption.Values {
		modelID := strings.TrimSpace(value.Value)
		if modelID == "" {
			continue
		}
		if _, exists := seen[modelID]; exists {
			continue
		}
		seen[modelID] = struct{}{}
		displayName := strings.TrimSpace(value.Label)
		if displayName == "" {
			displayName = modelID
		}
		available := true
		rows = append(rows, ModelRow{
			ProviderID:  providerID,
			ModelID:     modelID,
			DisplayName: displayName,
			SourceID:    SourceKindProviderLiveID(providerID),
			SourceKind:  SourceKindProviderLive,
			Priority:    PriorityProviderLive,
			Available:   &available,
			RefreshedAt: now,
		})
	}
	sortModelRowsByID(rows)
	return rows
}
