package extensionpkg

import (
	"context"
	"errors"
	"fmt"

	profilepkg "github.com/compozy/compozy/internal/profile"
)

type declaredProfileManager interface {
	CreateDeclared(context.Context, profilepkg.DeclaredInput) (profilepkg.Profile, error)
	GetByName(context.Context, string) (profilepkg.Profile, error)
	HasDeclaredMarker(context.Context, string, string) (bool, error)
}

var _ declaredProfileManager = (*profilepkg.Manager)(nil)

// DeclaredProfilePlan is the install/update summary presented before mutation.
type DeclaredProfilePlan struct {
	Extension string                     `json:"extension"`
	Profiles  []DeclaredProfilePlanEntry `json:"profiles"`
}

// DeclaredProfilePlanEntry describes one declaration's bind/create outcome.
type DeclaredProfilePlanEntry struct {
	Name       string                      `json:"name"`
	Create     bool                        `json:"create"`
	NeedsSetup []ManifestProfileCredential `json:"needs_setup,omitempty"`
}

// DeclaredProfileApplyResult records the mutation outcome without implying
// that a previously bound or marker-gated profile was created again.
type DeclaredProfileApplyResult struct {
	Profile profilepkg.Profile `json:"profile"`
	Created bool               `json:"created"`
	Bound   bool               `json:"bound"`
}

func BuildDeclaredProfilePlan(
	ctx context.Context,
	manager declaredProfileManager,
	manifest *Manifest,
) (DeclaredProfilePlan, error) {
	if manager == nil {
		return DeclaredProfilePlan{}, errors.New("extension: declared profile manager is required")
	}
	if manifest == nil {
		return DeclaredProfilePlan{}, errors.New("extension: manifest is required")
	}
	plan := DeclaredProfilePlan{Extension: manifest.Name, Profiles: make([]DeclaredProfilePlanEntry, 0, len(manifest.Profiles))}
	for _, declaration := range manifest.Profiles {
		marked, err := manager.HasDeclaredMarker(ctx, manifest.Name, declaration.Name)
		if err != nil {
			return DeclaredProfilePlan{}, fmt.Errorf("extension: inspect declared profile %q: %w", declaration.Name, err)
		}
		_, getErr := manager.GetByName(ctx, declaration.Name)
		exists := getErr == nil
		if getErr != nil && !errors.Is(getErr, profilepkg.ErrNotFound) {
			return DeclaredProfilePlan{}, fmt.Errorf("extension: inspect declared profile %q: %w", declaration.Name, getErr)
		}
		entry := DeclaredProfilePlanEntry{Name: declaration.Name, Create: !marked && !exists}
		if entry.Create {
			entry.NeedsSetup = append([]ManifestProfileCredential(nil), declaration.Credentials...)
		}
		plan.Profiles = append(plan.Profiles, entry)
	}
	return plan, nil
}

func ApplyDeclaredProfiles(
	ctx context.Context,
	manager declaredProfileManager,
	manifest *Manifest,
) ([]DeclaredProfileApplyResult, error) {
	plan, err := BuildDeclaredProfilePlan(ctx, manager, manifest)
	if err != nil {
		return nil, err
	}
	results := make([]DeclaredProfileApplyResult, 0, len(manifest.Profiles))
	for index, declaration := range manifest.Profiles {
		created, err := manager.CreateDeclared(ctx, declaredProfileInput(manifest.Name, declaration))
		if err != nil {
			return nil, fmt.Errorf("extension: apply declared profile %q: %w", declaration.Name, err)
		}
		results = append(results, DeclaredProfileApplyResult{
			Profile: created,
			Created: plan.Profiles[index].Create && !created.CreatedAt.IsZero(),
			Bound:   !plan.Profiles[index].Create,
		})
	}
	return results, nil
}

func declaredProfileInput(extension string, declaration ManifestProfile) profilepkg.DeclaredInput {
	credentials := make([]profilepkg.CredentialAsk, 0, len(declaration.Credentials))
	for _, credential := range declaration.Credentials {
		credentials = append(credentials, profilepkg.CredentialAsk{
			Provider: credential.Provider, Slot: credential.Slot,
		})
	}
	return profilepkg.DeclaredInput{
		Extension: extension,
		Name:      declaration.Name,
		Seed: profilepkg.DeclaredSeed{
			Color: declaration.Color, Icon: declaration.Icon, Emoji: declaration.Emoji,
			Defaults: profilepkg.PersonaDefaults{
				Agent: declaration.Defaults.Agent, Provider: declaration.Defaults.Provider,
				Sandbox: declaration.Defaults.Sandbox,
			},
			CredentialAsks: credentials,
		},
	}
}
