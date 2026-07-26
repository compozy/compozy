package daemon

import toolspkg "github.com/compozy/agh/internal/tools"

func (n *daemonNativeTools) windowManagerAvailability() toolspkg.NativeAvailabilityFunc {
	return n.dependencyAvailability(func() bool {
		return n.deps.WindowManager != nil && n.deps.Workspaces != nil
	})
}

func (n *daemonNativeTools) windowManagerToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDDesktopList:    {call: n.desktopList, availability: availability},
		toolspkg.ToolIDDesktopCreate:  {call: n.desktopCreate, availability: availability},
		toolspkg.ToolIDDesktopUpdate:  {call: n.desktopUpdate, availability: availability},
		toolspkg.ToolIDDesktopReorder: {call: n.desktopReorder, availability: availability},
		toolspkg.ToolIDDesktopSwitch:  {call: n.desktopSwitch, availability: availability},
		toolspkg.ToolIDDesktopDelete:  {call: n.desktopDelete, availability: availability},
		toolspkg.ToolIDDesktopClients: {call: n.desktopClients, availability: availability},
		toolspkg.ToolIDWindowList:     {call: n.windowList, availability: availability},
		toolspkg.ToolIDWindowOpen:     {call: n.windowOpen, availability: availability},
		toolspkg.ToolIDWindowNavigate: {call: n.windowNavigate, availability: availability},
		toolspkg.ToolIDWindowClose:    {call: n.windowClose, availability: availability},
		toolspkg.ToolIDWindowFocus:    {call: n.windowFocus, availability: availability},
		toolspkg.ToolIDWindowMove:     {call: n.windowMove, availability: availability},
		toolspkg.ToolIDWindowSwap:     {call: n.windowSwap, availability: availability},
		toolspkg.ToolIDWindowFloat:    {call: n.windowFloat, availability: availability},
		toolspkg.ToolIDWindowZoom:     {call: n.windowZoom, availability: availability},
		toolspkg.ToolIDLayoutGet:      {call: n.layoutGet, availability: availability},
		toolspkg.ToolIDLayoutPreview:  {call: n.layoutPreview, availability: availability},
		toolspkg.ToolIDLayoutArrange:  {call: n.layoutArrange, availability: availability},
		toolspkg.ToolIDLayoutResize:   {call: n.layoutResize, availability: availability},
		toolspkg.ToolIDLayoutBalance:  {call: n.layoutBalance, availability: availability},
		toolspkg.ToolIDLayoutUndo:     {call: n.layoutUndo, availability: availability},
		toolspkg.ToolIDLayoutRedo:     {call: n.layoutRedo, availability: availability},
		toolspkg.ToolIDLayoutExport:   {call: n.layoutExport, availability: availability},
		toolspkg.ToolIDLayoutValidate: {call: n.layoutValidate, availability: availability},
		toolspkg.ToolIDLayoutApply:    {call: n.layoutApply, availability: availability},
	}
}
