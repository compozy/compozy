package local

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
)

func TestLocalProviderToolHostAdditionalDirsContract(t *testing.T) {
	t.Run("Should authorize prepared tool host operations inside additional directories", func(t *testing.T) {
		t.Parallel()

		req := newTestPrepareRequest(t)
		provider := NewProvider(WithPermissionMode(aghconfig.PermissionModeApproveAll))
		prepared, err := provider.Prepare(context.Background(), req)
		if err != nil {
			t.Fatalf("Prepare() error = %v", err)
		}
		closePreparedToolHost(t, prepared)

		additionalFile := filepath.Join(req.LocalAdditionalDirs[0], "nested", "file.txt")
		if err := prepared.ToolHost.WriteTextFile(
			context.Background(),
			additionalFile,
			"additional content",
		); err != nil {
			t.Fatalf("WriteTextFile(additional dir) error = %v", err)
		}
		content, err := prepared.ToolHost.ReadTextFile(context.Background(), additionalFile)
		if err != nil {
			t.Fatalf("ReadTextFile(additional dir) error = %v", err)
		}
		if content != "additional content" {
			t.Fatalf("ReadTextFile(additional dir) = %q, want additional content", content)
		}

		outsideFile := filepath.Join(t.TempDir(), "outside.txt")
		if err := prepared.ToolHost.WriteTextFile(context.Background(), outsideFile, "outside"); !errors.Is(
			err,
			acp.ErrPathOutsideWorkspace,
		) {
			t.Fatalf("WriteTextFile(outside roots) error = %v, want ErrPathOutsideWorkspace", err)
		}
	})
}
