package daemon

import toolspkg "github.com/compozy/compozy/internal/tools"

func (n *daemonNativeTools) cmdPaletteToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDCmdPaletteList: {
			call: n.cmdPaletteList, availability: availability,
		},
		toolspkg.ToolIDCmdPaletteInvoke: {
			call: n.cmdPaletteInvoke, availability: availability,
		},
	}
}
