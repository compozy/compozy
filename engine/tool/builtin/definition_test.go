package builtin

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinTool(t *testing.T) {
	t.Run("Should call handler and return encoded output", func(t *testing.T) {
		called := false
		definition := BuiltinDefinition{
			ID:          "cp__echo",
			Description: "echo",
			Handler: func(_ context.Context, input map[string]any) (core.Output, error) {
				called = true
				return core.Output{"input": input["value"]}, nil
			},
		}
		tool, err := NewBuiltinTool(definition)
		require.NoError(t, err)
		result, err := tool.Call(context.Background(), `{"value":"hello"}`)
		require.NoError(t, err)
		assert.True(t, called)
		assert.JSONEq(t, `{"input":"hello"}`, result)
	})

	t.Run("Should expose args prototype", func(t *testing.T) {
		type args struct {
			Path string `json:"path"`
		}
		definition := BuiltinDefinition{
			ID:            "cp__args",
			Description:   "args",
			ArgsPrototype: args{},
			Handler: func(_ context.Context, _ map[string]any) (core.Output, error) {
				return core.Output{}, nil
			},
		}
		tool, err := NewBuiltinTool(definition)
		require.NoError(t, err)
		assert.IsType(t, args{}, tool.ArgsType())
	})

	t.Run("Should fail validation when handler missing", func(t *testing.T) {
		_, err := NewBuiltinTool(BuiltinDefinition{ID: "cp__bad"})
		require.Error(t, err)
	})
}

func TestValidationHelpers(t *testing.T) {
	t.Run("Should normalize relative root", func(t *testing.T) {
		root, err := NormalizeRoot(".")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(root))
	})

	t.Run("Should resolve path within root", func(t *testing.T) {
		root, err := NormalizeRoot(".")
		require.NoError(t, err)
		resolved, err := ResolvePath(root, "sub", "file.txt")
		require.NoError(t, err)
		assert.Contains(t, resolved, "sub")
	})

	t.Run("Should reject escaping paths", func(t *testing.T) {
		root, err := NormalizeRoot(".")
		require.NoError(t, err)
		_, err = ResolvePath(root, "..", "evil")
		require.Error(t, err)
	})

	t.Run("Should reject symlink info", func(t *testing.T) {
		info := mockFileInfo{mode: fs.ModeSymlink}
		require.Error(t, RejectSymlink(info))
	})

	t.Run("Should detect canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.Error(t, CheckContext(ctx))
	})

	t.Run("Should error on nil context", func(t *testing.T) {
		require.Error(t, CheckContext(nil)) //nolint:staticcheck // Validate explicit nil handling path.
	})
}

type mockFileInfo struct {
	name string
	mode fs.FileMode
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() any           { return nil }

func TestErrorConstructors(t *testing.T) {
	t.Run("Should wrap errors with canonical codes", func(t *testing.T) {
		err := errors.New("boom")
		assert.Equal(t, CodeInvalidArgument, InvalidArgument(err, nil).Code)
		assert.Equal(t, CodePermissionDenied, PermissionDenied(err, nil).Code)
		assert.Equal(t, CodeFileNotFound, FileNotFound(err, nil).Code)
		assert.Equal(t, CodeCommandNotAllowed, CommandNotAllowed(err, nil).Code)
		assert.Equal(t, CodeInternal, Internal(err, nil).Code)
	})
}
