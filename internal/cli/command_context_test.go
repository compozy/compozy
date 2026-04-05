package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestSignalCommandContextPropagatesParentCancellation(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancelCause(context.Background())
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(parent)

	ctx, stop := signalCommandContext(cmd)
	t.Cleanup(stop)

	want := errors.New("parent canceled")
	cancel(want)

	<-ctx.Done()
	if !errors.Is(context.Cause(ctx), want) {
		t.Fatalf("expected signal command context to preserve parent cause %v, got %v", want, context.Cause(ctx))
	}
}
