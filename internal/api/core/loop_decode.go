package core

import (
	"bytes"
	"fmt"
	"io"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/gin-gonic/gin"
)

func decodeStrictLoopJSONBody(c *gin.Context, target any) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return io.EOF
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Errorf("read loop request body: %w", err)
	}
	if path, found := looppkg.RetiredRuntimeKeyPath(body); found {
		return fmt.Errorf(
			"retired runtime key %s; see MIGRATION_GUIDE.md#per-task-runtime-selection",
			path,
		)
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return decodeStrictJSONBody(c, target)
}
