package extensionpkg

import bridgepkg "github.com/compozy/compozy/internal/bridges"

type hostAPIBridgeIngressContext struct {
	params     bridgepkg.InboundMessageEnvelope
	instance   *bridgepkg.BridgeInstance
	routingKey bridgepkg.RoutingKey
	lockKey    string
}
