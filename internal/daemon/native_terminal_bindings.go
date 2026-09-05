package daemon

import toolspkg "github.com/compozy/compozy/internal/tools"

func (n *daemonNativeTools) terminalToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDTerminalExec:         {call: n.terminalExec, availability: availability},
		toolspkg.ToolIDTerminalOpen:         {call: n.terminalOpen, availability: availability},
		toolspkg.ToolIDTerminalWrite:        {call: n.terminalWrite, availability: availability},
		toolspkg.ToolIDTerminalRead:         {call: n.terminalRead, availability: availability},
		toolspkg.ToolIDTerminalWait:         {call: n.terminalWait, availability: availability},
		toolspkg.ToolIDTerminalSignal:       {call: n.terminalSignal, availability: availability},
		toolspkg.ToolIDTerminalClose:        {call: n.terminalClose, availability: availability},
		toolspkg.ToolIDTerminalList:         {call: n.terminalList, availability: availability},
		toolspkg.ToolIDTerminalRequestInput: {call: n.terminalRequestInput, availability: availability},
	}
}
