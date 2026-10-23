package workspace

func (r *Resolver) freshSnapshot(key string, ws Workspace, workspaceID, profileID string) (ResolvedWorkspace, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	cached := r.cache[key]
	if cached == nil || r.snapshotTTL <= 0 || now.Before(cached.validatedAt) ||
		now.Sub(cached.validatedAt) >= r.snapshotTTL || now.Sub(cached.lastAccess) >= r.cacheTTL ||
		!sameWorkspaceRuntimeInputs(cached.workspace, ws) {
		return ResolvedWorkspace{}, false
	}
	cached.lastAccess = now
	resolved := cloneResolvedWorkspace(&cached.resolved)
	resolved.Workspace = cloneWorkspace(ws)
	resolved.WorkspaceID, resolved.ProfileID = workspaceID, profileID
	return resolved, true
}
