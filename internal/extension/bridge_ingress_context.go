package extensionpkg

import bridgepkg "github.com/compozy/agh/internal/bridges"

type hostAPIBridgeIngressContext struct {
	params     bridgepkg.InboundMessageEnvelope
	instance   *bridgepkg.BridgeInstance
	routingKey bridgepkg.RoutingKey
	lockKey    string
}
