package core

import (
	"context"

	compozyupdate "github.com/compozy/compozy/internal/update"
)

// SettingsUpdateApply describes an accepted update operation.
type SettingsUpdateApply struct {
	Targets     []compozyupdate.Target
	Status      compozyupdate.ApplyStatus
	OperationID string
	Message     string
	Holder      *compozyupdate.Holder
}

// SettingsUpdateCancel describes a canceled update operation.
type SettingsUpdateCancel struct {
	Status      compozyupdate.Status
	OperationID string
	Message     string
	Holder      *compozyupdate.Holder
}

// SettingsUpdateController exposes the daemon-owned update status surface to settings transports.
type SettingsUpdateController interface {
	GetUpdate(ctx context.Context) (compozyupdate.MultiState, error)
	ApplyUpdate(ctx context.Context, targets []compozyupdate.Target) (SettingsUpdateApply, error)
	CancelUpdate(ctx context.Context) (SettingsUpdateCancel, error)
}
