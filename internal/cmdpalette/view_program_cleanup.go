package cmdpalette

import (
	"context"
	"time"
)

const viewProgramCleanupTimeout = 3 * time.Second

func (s *Service) closeViewProgram(
	ctx context.Context,
	profileLens ProfileLens,
	workspace WorkspaceID,
	extension string,
	request ViewCloseRequest,
) error {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), viewProgramCleanupTimeout)
	defer cancel()
	return s.viewPrograms.CloseProgram(closeCtx, profileLens, workspace, extension, request)
}
