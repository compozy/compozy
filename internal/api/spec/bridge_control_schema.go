package spec

import bridgepkg "github.com/compozy/compozy/internal/bridges"

func bridgeCheckStatusValues() []string {
	return []string{
		string(bridgepkg.BridgeCheckStatusPass),
		string(bridgepkg.BridgeCheckStatusWarn),
		string(bridgepkg.BridgeCheckStatusFail),
		string(bridgepkg.BridgeCheckStatusSkipped),
	}
}
