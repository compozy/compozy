package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func executionProfileFromGenerated(row sqlcgen.GetTaskExecutionProfileRow) (taskpkg.ExecutionProfile, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf("store: parse task execution profile created_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf("store: parse task execution profile updated_at: %w", err)
	}
	networkParticipation, err := executionProfileParticipationFromGenerated(
		row.NetworkMode,
		row.NetworkChannelStrategy,
		row.NetworkChannel,
		row.NetworkBoundsJson,
	)
	if err != nil {
		return taskpkg.ExecutionProfile{}, err
	}
	workerACPOptions, err := decodeTaskProfileACPOptions(row.WorkerAcpOptionsJson)
	if err != nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf("store: decode task worker ACP options: %w", err)
	}
	reviewACPOptions, err := decodeTaskProfileACPOptions(row.ReviewAcpOptionsJson)
	if err != nil {
		return taskpkg.ExecutionProfile{}, fmt.Errorf("store: decode task review ACP options: %w", err)
	}
	return taskpkg.ExecutionProfile{
		TaskID: row.TaskID,
		Coordinator: taskpkg.CoordinatorProfile{
			Mode: taskpkg.CoordinatorMode(row.CoordinatorMode), AgentName: row.CoordinatorAgentName,
			Provider: row.CoordinatorProvider, Model: row.CoordinatorModel, Guidance: row.CoordinatorGuidance,
		},
		Worker: taskpkg.WorkerProfile{
			Mode: taskpkg.WorkerMode(row.WorkerMode), AgentName: row.WorkerAgentName,
			Provider: row.WorkerProvider, Model: row.WorkerModel,
			ReasoningEffort: row.WorkerReasoningEffort,
			Speed:           speedpkg.Speed(row.WorkerSpeed),
			ACPOptions:      workerACPOptions,
		},
		Review: taskpkg.ReviewProfile{
			AgentName: row.ReviewAgentName, Provider: row.ReviewProvider, Model: row.ReviewModel,
			ReasoningEffort: row.ReviewReasoningEffort,
			Speed:           speedpkg.Speed(row.ReviewSpeed),
			ACPOptions:      reviewACPOptions,
		},
		Sandbox: taskpkg.SandboxPolicy{
			Mode: taskpkg.SandboxMode(row.SandboxMode), SandboxRef: row.SandboxRef,
		},
		Worktree: taskpkg.WorktreePolicy{
			Mode: taskpkg.WorktreeMode(row.WorktreeMode), WorktreeRef: row.WorktreeRef,
		},
		Runtime:              taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeMode(row.RuntimeMode)},
		NetworkParticipation: networkParticipation,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}, nil
}

func encodeTaskProfileACPOptions(selections []taskpkg.ACPOptionSelection) (string, error) {
	if len(selections) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(selections)
	if err != nil {
		return "", fmt.Errorf("marshal ACP option selections: %w", err)
	}
	return string(encoded), nil
}

func decodeTaskProfileACPOptions(encoded string) ([]taskpkg.ACPOptionSelection, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, nil
	}
	var selections []taskpkg.ACPOptionSelection
	if err := json.Unmarshal([]byte(encoded), &selections); err != nil {
		return nil, fmt.Errorf("unmarshal ACP option selections: %w", err)
	}
	return selections, nil
}

func executionProfileParticipationFromGenerated(
	mode string,
	strategy sql.NullString,
	channel sql.NullString,
	boundsJSON sql.NullString,
) (*participation.Request, error) {
	if strings.TrimSpace(mode) == "" && !strategy.Valid && !channel.Valid && !boundsJSON.Valid {
		return nil, nil
	}
	request := participation.Request{}
	if trimmedMode := strings.TrimSpace(mode); trimmedMode != "" {
		value := participation.Mode(trimmedMode)
		request.Mode = &value
	}
	if strategy.Valid {
		value := participation.ChannelStrategy(strings.TrimSpace(strategy.String))
		request.ChannelStrategy = &value
	}
	if channel.Valid {
		value := strings.TrimSpace(channel.String)
		request.ChannelID = &value
	}
	if boundsJSON.Valid {
		var bounds participation.BoundsRequest
		if err := json.Unmarshal([]byte(boundsJSON.String), &bounds); err != nil {
			return nil, fmt.Errorf("store: decode task execution profile network bounds: %w", err)
		}
		request.Bounds = &bounds
	}
	normalized, err := participation.NormalizeIntent(request)
	if err != nil {
		return nil, fmt.Errorf("store: validate task execution profile network participation: %w", err)
	}
	return &normalized, nil
}

func executionProfileParticipationColumns(
	request *participation.Request,
) (string, sql.NullString, sql.NullString, sql.NullString, error) {
	if request == nil {
		return "", sql.NullString{}, sql.NullString{}, sql.NullString{}, nil
	}
	normalized, err := participation.NormalizeIntent(*request)
	if err != nil {
		return "", sql.NullString{}, sql.NullString{}, sql.NullString{}, err
	}
	mode := ""
	if normalized.Mode != nil {
		mode = string(*normalized.Mode)
	}
	strategy := sql.NullString{}
	if normalized.ChannelStrategy != nil {
		strategy = sql.NullString{String: string(*normalized.ChannelStrategy), Valid: true}
	}
	channel := sql.NullString{}
	if normalized.ChannelID != nil {
		channel = sql.NullString{String: *normalized.ChannelID, Valid: true}
	}
	bounds := sql.NullString{}
	if normalized.Bounds != nil {
		encoded, marshalErr := json.Marshal(normalized.Bounds)
		if marshalErr != nil {
			return "", sql.NullString{}, sql.NullString{}, sql.NullString{}, fmt.Errorf(
				"store: encode task execution profile network bounds: %w",
				marshalErr,
			)
		}
		bounds = sql.NullString{String: string(encoded), Valid: true}
	}
	return mode, strategy, channel, bounds, nil
}
