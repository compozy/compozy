package spec

func marketplaceOperations() []OperationSpec {
	return []OperationSpec{
		marketplaceSearchOperation(),
		marketplaceKindOperation(),
		marketplaceEntryOperation(),
		marketplaceRefreshOperation(),
	}
}
