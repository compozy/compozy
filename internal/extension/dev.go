package extensionpkg

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// DevLinkRequest identifies one immutable workspace-local extension generation.
type DevLinkRequest struct {
	Name                     string
	WorkspaceID              string
	OriginPath               string
	GenerationHash           string
	NetworkRequirementDigest string
}

// DevLink is one workspace-local overlay row.
type DevLink struct {
	ExtensionName            string
	WorkspaceID              string
	OriginPath               string
	BundleGeneration         string
	LinkedAt                 time.Time
	NetworkRequirementDigest string
	NetworkConfirmedBy       string
	NetworkConfirmedAt       time.Time
}

// ActiveExtension is the registry resolution result for one instance key.
type ActiveExtension struct {
	Published          *ExtensionInfo
	DevLink            *DevLink
	OverridesPublished bool
}

// LinkDev upserts one workspace-local overlay without modifying a published row.
func (r *Registry) LinkDev(req DevLinkRequest) (*DevLink, error) {
	if err := r.checkReady("link development extension"); err != nil {
		return nil, err
	}
	name, err := normalizeExtensionName(req.Name)
	if err != nil {
		return nil, err
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return nil, errors.New("extension: workspace id is required for a development link")
	}
	originPath := strings.TrimSpace(req.OriginPath)
	if originPath == "" {
		return nil, errors.New("extension: development origin path is required")
	}
	generation := strings.TrimSpace(req.GenerationHash)
	if !generationHashPattern.MatchString(generation) {
		return nil, fmt.Errorf(
			"%w: generation hash must be 64 lowercase hexadecimal characters",
			ErrExtensionGenerationInvalid,
		)
	}
	linkedAt := r.now().UTC()
	networkRequirementDigest := strings.TrimSpace(req.NetworkRequirementDigest)
	_, err = r.db.ExecContext(registryContext(), `
		INSERT INTO extension_dev_links (
			extension_name, workspace_id, origin_path, bundle_generation, linked_at,
			network_requirement_digest, network_confirmed_by, network_confirmed_at
		) VALUES (?, ?, ?, ?, ?, ?, NULL, NULL)
		ON CONFLICT(extension_name, workspace_id) DO UPDATE SET
			origin_path = excluded.origin_path,
			bundle_generation = excluded.bundle_generation,
			linked_at = excluded.linked_at,
			network_confirmed_by = CASE
				WHEN extension_dev_links.network_requirement_digest = excluded.network_requirement_digest
				THEN extension_dev_links.network_confirmed_by
				ELSE NULL
			END,
			network_confirmed_at = CASE
				WHEN extension_dev_links.network_requirement_digest = excluded.network_requirement_digest
				THEN extension_dev_links.network_confirmed_at
				ELSE NULL
			END,
			network_requirement_digest = excluded.network_requirement_digest
	`, name, workspaceID, originPath, generation, linkedAt, networkRequirementDigest)
	if err != nil {
		return nil, fmt.Errorf("extension: link development extension %q: %w", name, err)
	}
	return r.GetDevLink(name, workspaceID)
}

// UnlinkDev removes only the workspace overlay row.
func (r *Registry) UnlinkDev(name, workspaceID string) error {
	if err := r.checkReady("unlink development extension"); err != nil {
		return err
	}
	key, err := normalizeDevInstanceKey(name, workspaceID)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(
		registryContext(),
		`DELETE FROM extension_dev_links WHERE extension_name = ? AND workspace_id = ?`,
		key.Name,
		key.WorkspaceID,
	)
	if err != nil {
		return fmt.Errorf("extension: unlink development extension %q: %w", key.Name, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("extension: inspect development unlink %q: %w", key.Name, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	return nil
}

// GetDevLink returns one workspace-local overlay row.
func (r *Registry) GetDevLink(name, workspaceID string) (*DevLink, error) {
	if err := r.checkReady("get development extension"); err != nil {
		return nil, err
	}
	key, err := normalizeDevInstanceKey(name, workspaceID)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(registryContext(), `
		SELECT extension_name, workspace_id, origin_path, bundle_generation, linked_at,
		       network_requirement_digest, network_confirmed_by, network_confirmed_at
		FROM extension_dev_links
		WHERE extension_name = ? AND workspace_id = ?
	`, key.Name, key.WorkspaceID)
	link, err := scanDevLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	return link, err
}

// ListDevLinks returns every dev overlay in stable instance-key order.
func (r *Registry) ListDevLinks() (links []DevLink, resultErr error) {
	if err := r.checkReady("list development extensions"); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(registryContext(), `
		SELECT extension_name, workspace_id, origin_path, bundle_generation, linked_at,
		       network_requirement_digest, network_confirmed_by, network_confirmed_at
		FROM extension_dev_links
		ORDER BY extension_name ASC, workspace_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("extension: list development extensions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("extension: close development link rows: %w", closeErr))
		}
	}()
	links = make([]DevLink, 0)
	for rows.Next() {
		link, scanErr := scanDevLink(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		links = append(links, *link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("extension: iterate development links: %w", err)
	}
	return links, nil
}

// DevLinkNamesForWorkspace returns the development source owners trusted by one workspace.
func (r *Registry) DevLinkNamesForWorkspace(workspaceID string) (names []string, resultErr error) {
	if err := r.checkReady("list workspace development extensions"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("extension: workspace id is required to list development links")
	}
	rows, err := r.db.QueryContext(
		registryContext(),
		`SELECT extension_name
		 FROM extension_dev_links
		 WHERE workspace_id = ?
		 ORDER BY extension_name ASC`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("extension: list workspace development extensions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("extension: close workspace development links: %w", closeErr))
		}
	}()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("extension: scan workspace development extension: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("extension: iterate workspace development extensions: %w", err)
	}
	return names, nil
}

// ResolveActive prefers a workspace dev overlay and falls back to the published row.
func (r *Registry) ResolveActive(name, workspaceID string) (*ActiveExtension, error) {
	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return nil, err
	}
	var published *ExtensionInfo
	published, err = r.Get(trimmedName)
	if err != nil && !errors.Is(err, ErrExtensionNotFound) {
		return nil, err
	}
	if errors.Is(err, ErrExtensionNotFound) {
		published = nil
	}
	if strings.TrimSpace(workspaceID) != "" {
		link, linkErr := r.GetDevLink(trimmedName, workspaceID)
		if linkErr == nil {
			return &ActiveExtension{
				Published:          published,
				DevLink:            link,
				OverridesPublished: published != nil,
			}, nil
		}
		if !errors.Is(linkErr, ErrExtensionNotDevLinked) {
			return nil, linkErr
		}
	}
	if published == nil {
		return nil, &ExtensionNotFoundError{Name: trimmedName}
	}
	return &ActiveExtension{Published: published}, nil
}

func normalizeDevInstanceKey(name, workspaceID string) (InstanceKey, error) {
	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return InstanceKey{}, err
	}
	key := InstanceKey{Name: trimmedName, WorkspaceID: strings.TrimSpace(workspaceID)}
	if key.WorkspaceID == "" {
		return InstanceKey{}, errors.New("extension: workspace id is required for a development instance")
	}
	return key, nil
}

func scanDevLink(scanner interface{ Scan(dest ...any) error }) (*DevLink, error) {
	var link DevLink
	var confirmedBy sql.NullString
	var confirmedAt sql.NullString
	if err := scanner.Scan(
		&link.ExtensionName,
		&link.WorkspaceID,
		&link.OriginPath,
		&link.BundleGeneration,
		&link.LinkedAt,
		&link.NetworkRequirementDigest,
		&confirmedBy,
		&confirmedAt,
	); err != nil {
		return nil, err
	}
	link.LinkedAt = link.LinkedAt.UTC()
	link.NetworkRequirementDigest = strings.TrimSpace(link.NetworkRequirementDigest)
	link.NetworkConfirmedBy = strings.TrimSpace(confirmedBy.String)
	if confirmedAt.Valid {
		parsed, err := store.ParseTimestamp(confirmedAt.String)
		if err != nil {
			return nil, fmt.Errorf("extension: parse development network_confirmed_at: %w", err)
		}
		link.NetworkConfirmedAt = parsed
	}
	return &link, nil
}
