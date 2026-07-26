package hooks

func (payload WindowManagerPayload) cloneForAsync() WindowManagerPayload {
	payload.Changes.DesktopIDs = append([]string(nil), payload.Changes.DesktopIDs...)
	payload.Changes.WindowIDs = append([]string(nil), payload.Changes.WindowIDs...)
	payload.Changes.GroupIDs = append([]string(nil), payload.Changes.GroupIDs...)
	payload.Changes.NodeIDs = append([]string(nil), payload.Changes.NodeIDs...)
	payload.Changes.ClientIDs = append([]string(nil), payload.Changes.ClientIDs...)
	return payload
}
