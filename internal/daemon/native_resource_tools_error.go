package daemon

import (
	core "github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeResourceToolError(id toolspkg.ToolID, err error) error {
	return nativeHTTPStatusToolError(id, err, core.StatusForResourceError(err))
}
