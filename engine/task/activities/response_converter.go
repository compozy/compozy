package activities

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/logger"
)

// ResponseConverter provides utilities for converting response outputs to specific response types.
// This utility centralizes response conversion logic to eliminate code duplication across
// exec_basic.go, collection_resp.go, and response_helpers.go.
//
// The converter handles safe conversion with nil checks and provides consistent behavior
// for converting shared.ResponseOutput to various task response types.
type ResponseConverter struct{}

// NewResponseConverter creates a new ResponseConverter instance.
// The converter is stateless and can be reused across multiple conversions.
func NewResponseConverter() *ResponseConverter {
	return &ResponseConverter{}
}

// ConvertToMainTaskResponse converts shared.ResponseOutput to task.MainTaskResponse
// This eliminates the duplication found in exec_basic.go:142-157
func (rc *ResponseConverter) ConvertToMainTaskResponse(result *shared.ResponseOutput) *task.MainTaskResponse {
	if result == nil {
		return &task.MainTaskResponse{}
	}
	var mainTaskResponse *task.MainTaskResponse
	if result.Response != nil {
		if mtr, ok := result.Response.(*task.MainTaskResponse); ok {
			mainTaskResponse = mtr
		}
	}
	if mainTaskResponse == nil {
		mainTaskResponse = &task.MainTaskResponse{
			State: result.State,
		}
	}
	return mainTaskResponse
}

// ConvertToCollectionResponse converts shared.ResponseOutput to task.CollectionResponse with metadata
// This eliminates the duplication found in collection_resp.go:117-158
func (rc *ResponseConverter) ConvertToCollectionResponse(
	ctx context.Context,
	result *shared.ResponseOutput,
	configStore task2core.ConfigStore,
	task2Factory interface {
		CreateTaskConfigRepository(store task2core.ConfigStore, cwd *core.PathCWD) (shared.TaskConfigRepository, error)
	},
	cwd *core.PathCWD,
) *task.CollectionResponse {
	if result == nil {
		return &task.CollectionResponse{
			MainTaskResponse: &task.MainTaskResponse{},
			ItemCount:        0,
			SkippedCount:     0,
		}
	}
	mainTaskResponse := rc.ConvertToMainTaskResponse(result)
	response := &task.CollectionResponse{
		MainTaskResponse: mainTaskResponse,
		ItemCount:        0,
		SkippedCount:     0,
	}
	if result.State != nil && result.State.Output != nil {
		if metadata, exists := (*result.State.Output)["collection_metadata"]; exists {
			if metadataMap, ok := metadata.(map[string]any); ok {
				response.ItemCount = core.AnyToInt(metadataMap["item_count"])
				response.SkippedCount = core.AnyToInt(metadataMap["skipped_count"])
			}
		}
	}
	if configStore != nil && task2Factory != nil && result.State != nil {
		configRepo, err := task2Factory.CreateTaskConfigRepository(configStore, cwd)
		if err != nil {
			logger.FromContext(ctx).Error("failed to create task config repository", "error", err)
		} else {
			metadata, err := configRepo.LoadCollectionMetadata(ctx, result.State.TaskExecID)
			if err == nil && metadata != nil {
				if collectionMetadata, ok := metadata.(*task2core.CollectionTaskMetadata); ok {
					response.ItemCount = collectionMetadata.ItemCount
					response.SkippedCount = collectionMetadata.SkippedCount
				}
			}
		}
	}
	return response
}
