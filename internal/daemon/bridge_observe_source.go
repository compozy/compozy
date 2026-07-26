package daemon

import (
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/observe"
)

func bridgeObserveSource(service core.BridgeService) observe.BridgeSource {
	if service == nil {
		return nil
	}
	source, ok := service.(observe.BridgeSource)
	if !ok {
		return nil
	}
	return source
}
