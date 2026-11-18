package hooks

func (payload WindowManagerPayload) cloneForAsync() WindowManagerPayload {
	payload.Changes.DesktopIDs = append([]string(nil), payload.Changes.DesktopIDs...)
	payload.Changes.WindowIDs = append([]string(nil), payload.Changes.WindowIDs...)
	payload.Changes.GroupIDs = append([]string(nil), payload.Changes.GroupIDs...)
	payload.Changes.NodeIDs = append([]string(nil), payload.Changes.NodeIDs...)
	payload.Changes.ClientIDs = append([]string(nil), payload.Changes.ClientIDs...)
	payload.Changes.TabIDs = append([]string(nil), payload.Changes.TabIDs...)
	payload.Changes.TabSplit = append([]string(nil), payload.Changes.TabSplit...)
	payload.Changes.TabUnsplit = append([]string(nil), payload.Changes.TabUnsplit...)
	payload.Changes.StackGrouped = append([]string(nil), payload.Changes.StackGrouped...)
	payload.Changes.StackUngrouped = append([]string(nil), payload.Changes.StackUngrouped...)
	return payload
}
