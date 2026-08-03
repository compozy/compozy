package daemon

import (
	"fmt"

	"github.com/compozy/compozy/internal/resources"
)

func resolveDaemonResourceStore[T any](
	state *bootState,
	kind resources.ResourceKind,
	label string,
) (resources.KindCodec[T], resources.Store[T], error) {
	codec, err := resources.ResolveCodec[T](state.resourceCodecs, kind)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve %s codec: %w", label, err)
	}
	store, err := resources.NewStore(state.resourceKernel, codec)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: create %s resource store: %w", label, err)
	}
	return codec, store, nil
}
