package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrActivationNotFound = errors.New("bundles: activation not found")
)

type Scope string

const (
	ScopeGlobal    Scope = "global"
	ScopeWorkspace Scope = "workspace"
)

func (s Scope) Normalize() Scope {
	return Scope(strings.ToLower(strings.TrimSpace(string(s))))
}

func (s Scope) Validate(workspaceID string) error {
	normalized := s.Normalize()
	switch normalized {
	case ScopeGlobal:
		if strings.TrimSpace(workspaceID) != "" {
			return errors.New("bundles: global activation cannot include workspace id")
		}
		return nil
	case ScopeWorkspace:
		if strings.TrimSpace(workspaceID) == "" {
			return errors.New("bundles: workspace activation requires workspace id")
		}
		return nil
	case "":
		return errors.New("bundles: scope is required")
	default:
		return fmt.Errorf("bundles: unsupported scope %q", normalized)
	}
}

type Activation struct {
	ID                       string
	Version                  int64
	ExtensionName            string
	BundleName               string
	ProfileName              string
	Scope                    Scope
	WorkspaceID              string
	SpecContentHash          string
	NetworkRequirementDigest string
	ConfirmedBy              string
	ConfirmedAt              string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func (a Activation) Normalize() Activation {
	a.ID = strings.TrimSpace(a.ID)
	a.ExtensionName = strings.TrimSpace(a.ExtensionName)
	a.BundleName = strings.TrimSpace(a.BundleName)
	a.ProfileName = strings.TrimSpace(a.ProfileName)
	a.Scope = a.Scope.Normalize()
	a.WorkspaceID = strings.TrimSpace(a.WorkspaceID)
	a.SpecContentHash = strings.TrimSpace(a.SpecContentHash)
	a.NetworkRequirementDigest = strings.TrimSpace(a.NetworkRequirementDigest)
	a.ConfirmedBy = strings.TrimSpace(a.ConfirmedBy)
	a.ConfirmedAt = strings.TrimSpace(a.ConfirmedAt)
	return a
}

func (a Activation) Validate() error {
	a = a.Normalize()
	return a.validateNormalized()
}

func (a Activation) Validated() (Activation, error) {
	a = a.Normalize()
	if err := a.validateNormalized(); err != nil {
		return Activation{}, err
	}
	return a, nil
}

func (a Activation) validateNormalized() error {
	if a.Version < 0 {
		return errors.New("bundles: activation version cannot be negative")
	}
	if err := requireNonEmpty(a.ID, "bundles: activation id is required"); err != nil {
		return err
	}
	if err := requireNonEmpty(a.ExtensionName, "bundles: activation extension name is required"); err != nil {
		return err
	}
	if err := requireNonEmpty(a.BundleName, "bundles: activation bundle name is required"); err != nil {
		return err
	}
	if err := requireNonEmpty(a.ProfileName, "bundles: activation profile name is required"); err != nil {
		return err
	}
	return a.Scope.Validate(a.WorkspaceID)
}

type InventoryItem struct {
	ActivationID  string
	ResourceKind  string
	ResourceID    string
	ResourceName  string
	RecordedAtUTC time.Time
}

func (i InventoryItem) Normalize() InventoryItem {
	i.ActivationID = strings.TrimSpace(i.ActivationID)
	i.ResourceKind = strings.TrimSpace(i.ResourceKind)
	i.ResourceID = strings.TrimSpace(i.ResourceID)
	i.ResourceName = strings.TrimSpace(i.ResourceName)
	return i
}

func (i InventoryItem) Validate() error {
	i = i.Normalize()
	return i.validateNormalized()
}

func (i InventoryItem) Validated() (InventoryItem, error) {
	i = i.Normalize()
	if err := i.validateNormalized(); err != nil {
		return InventoryItem{}, err
	}
	return i, nil
}

func (i InventoryItem) validateNormalized() error {
	if err := requireNonEmpty(i.ActivationID, "bundles: inventory activation id is required"); err != nil {
		return err
	}
	if err := requireNonEmpty(i.ResourceKind, "bundles: inventory resource kind is required"); err != nil {
		return err
	}
	if err := requireNonEmpty(i.ResourceID, "bundles: inventory resource id is required"); err != nil {
		return err
	}
	if err := requireNonEmpty(i.ResourceName, "bundles: inventory resource name is required"); err != nil {
		return err
	}
	return nil
}

func requireNonEmpty(value string, message string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(message)
	}
	return nil
}
