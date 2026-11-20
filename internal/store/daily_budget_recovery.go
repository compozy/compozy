package store

import "time"

type DailyBudgetSessionOwnership struct {
	InstanceID string
	Unresolved bool
}

type DailyBudgetReceiptState struct {
	Day                 string
	Present             bool
	OwnershipUnresolved bool
}

type DailyBudgetPotentialOwner struct {
	InstanceID string
	CreatedAt  time.Time
}

// DailyBudgetReconcileSession identifies a ledger and its potential budget scope.
// A historical ancestry gap requires a check for unsettled current-day spend
// before a cap can use the ledger, without assigning the gap to that owner.
type DailyBudgetReconcileSession struct {
	SessionInfo
	UnresolvedOwner   bool
	InstanceCreatedAt time.Time
}
