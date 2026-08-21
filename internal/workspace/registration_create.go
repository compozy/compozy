package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (r *Resolver) createWorkspaceRegistration(ctx context.Context, opts RegisterOptions) (Workspace, error) {
	rootDir, err := canonicalRoot(opts.RootDir)
	if err != nil {
		return Workspace{}, err
	}
	if err := r.rejectOperatorHomeRegistration(rootDir); err != nil {
		return Workspace{}, err
	}

	additionalDirs, err := normalizeAdditionalDirs(rootDir, opts.AdditionalDirs)
	if err != nil {
		return Workspace{}, err
	}

	if _, err := r.lookupWorkspaceBySameRoot(ctx, rootDir); err == nil {
		return Workspace{}, fmt.Errorf("workspace: register workspace %q: %w", rootDir, ErrWorkspacePathTaken)
	} else if !errors.Is(err, ErrWorkspaceNotFound) {
		return Workspace{}, err
	}

	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name, err = r.nextWorkspaceName(ctx, rootDir)
		if err != nil {
			return Workspace{}, err
		}
	}

	now := r.now()
	id, err := r.nextRegistrationID()
	if err != nil {
		return Workspace{}, err
	}
	ws := Workspace{
		ID:             id,
		RootDir:        rootDir,
		AdditionalDirs: additionalDirs,
		Name:           name,
		DefaultAgent:   strings.TrimSpace(opts.DefaultAgent),
		SandboxRef:     strings.TrimSpace(opts.SandboxRef),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	for {
		if err := checkContext(ctx); err != nil {
			return Workspace{}, err
		}

		if insertErr := r.store.InsertWorkspace(ctx, ws); insertErr != nil {
			switch {
			case errors.Is(insertErr, ErrWorkspaceNameTaken) && strings.TrimSpace(opts.Name) == "":
				name, err = r.nextWorkspaceName(ctx, rootDir)
				if err != nil {
					return Workspace{}, err
				}
				id, generateErr := r.nextRegistrationID()
				if generateErr != nil {
					return Workspace{}, generateErr
				}
				ws.Name = name
				ws.ID = id
				ws.CreatedAt = now
				ws.UpdatedAt = now
				continue
			default:
				return Workspace{}, fmt.Errorf("workspace: register workspace %q: %w", rootDir, insertErr)
			}
		}

		return ws, nil
	}
}
