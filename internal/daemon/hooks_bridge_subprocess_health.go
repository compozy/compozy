package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

func (n *hooksNotifier) OnSubprocessHealth(
	ctx context.Context,
	observation session.SubprocessHealthSnapshot,
) {
	if runtime := n.subprocessHealthNotifier(); runtime != nil {
		runtime.OnSubprocessHealth(ctx, observation)
	}
}
