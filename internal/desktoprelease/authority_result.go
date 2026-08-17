package desktoprelease

func resultFromExisting(operation string, commit *ChannelCommit, verified []Artifact) OperatorResult {
	return OperatorResult{
		Operation: operation, ChannelRefBefore: commit.SHA, ChannelRefAfter: commit.SHA,
		VerifiedInventory: verified, AuditCommit: commit.SHA, Outcome: "already_completed",
	}
}

func cloneFiles(source map[string][]byte) map[string][]byte {
	cloned := make(map[string][]byte, len(source))
	for name, contents := range source {
		cloned[name] = append([]byte(nil), contents...)
	}
	return cloned
}
