package store

import (
	"context"
	"errors"
)

var ErrComputerSSHSetupNotFound = errors.New("store: SSH setup not found")

type ComputerSSHSetup struct {
	ID, AccountID, HostKey, TargetAlias, Name string
	Kind, Step, ReleaseVersion                string
	Platform, BinaryPath, ServiceWarning      string
	ReservationID, LinkTokenRef               string
	LinkTokenExpiresAt, ComputerID            string
	ErrorCode, ErrorDetail, UpdatedAt         string
}

type ComputerSSHSetupStore interface {
	GetComputerSSHSetup(ctx context.Context, account, id string) (ComputerSSHSetup, error)
	FindPendingComputerSSHSetup(ctx context.Context, account, host string) (ComputerSSHSetup, error)
	CreateComputerSSHSetup(context.Context, ComputerSSHSetup) error
	WriteComputerSSHSetup(context.Context, ComputerSSHSetup) error
}
