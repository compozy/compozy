package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func (m *Manager) GetByName(ctx context.Context, name string) (Profile, error) {
	if ctx == nil {
		return Profile{}, errors.New("profile: get context is required")
	}
	return getProfileByName(ctx, m.store.DB(), strings.TrimSpace(name))
}

// GetWithCounts returns one profile with its ownership and credential summary.
func (m *Manager) GetWithCounts(ctx context.Context, name string) (WithCounts, error) {
	if ctx == nil {
		return WithCounts{}, errors.New("profile: get detail context is required")
	}
	profile, err := getProfileByName(ctx, m.store.DB(), strings.TrimSpace(name))
	if err != nil {
		return WithCounts{}, err
	}
	counts, err := m.profileCounts(ctx, m.store.DB(), profile.ID)
	if err != nil {
		return WithCounts{}, err
	}
	requirements, err := listCredentialRequirements(ctx, m.store.DB(), profile.ID)
	if err != nil {
		return WithCounts{}, err
	}
	return WithCounts{
		Profile:                profile,
		WorkItems:              counts.workItems,
		NeedsSetup:             len(requirements) > 0,
		CredentialRequirements: requirements,
	}, nil
}

// ListNames returns the profile names needed by lightweight ownership diagnostics.
func (m *Manager) ListNames(ctx context.Context) (names []string, err error) {
	if ctx == nil {
		return nil, errors.New("profile: list names context is required")
	}
	rows, err := m.store.DB().QueryContext(ctx, `SELECT name FROM profiles ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("profile: list names: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("profile: close name rows: %w", closeErr))
		}
	}()
	names = make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("profile: scan name row: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profile: iterate name rows: %w", err)
	}
	return names, nil
}

// ProfileName resolves the current profile name for one stable profile id.
func (m *Manager) ProfileName(ctx context.Context, profileID string) (string, error) {
	if ctx == nil {
		return "", errors.New("profile: get name context is required")
	}
	profile, err := getProfileByID(ctx, m.store.DB(), strings.TrimSpace(profileID))
	if err != nil {
		return "", err
	}
	if err := ensureAvailable(ctx, m.store.DB(), profile, true); err != nil {
		return "", err
	}
	return profile.Name, nil
}

func (m *Manager) List(ctx context.Context) (profiles []WithCounts, err error) {
	if ctx == nil {
		return nil, errors.New("profile: list context is required")
	}
	rows, err := m.store.DB().QueryContext(ctx, profileSelectSQL()+` ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("profile: list: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("profile: close list rows: %w", closeErr))
		}
	}()
	profiles = make([]WithCounts, 0)
	profilesByID := make([]Profile, 0)
	for rows.Next() {
		profile, scanErr := scanProfile(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("profile: scan list row: %w", scanErr)
		}
		profilesByID = append(profilesByID, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profile: iterate list: %w", err)
	}
	profileIDs := make([]string, 0, len(profilesByID))
	for _, profile := range profilesByID {
		profileIDs = append(profileIDs, profile.ID)
	}
	counts, err := m.profileCountsForProfiles(ctx, m.store.DB(), profileIDs)
	if err != nil {
		return nil, err
	}
	requirements, err := listCredentialRequirementsForProfiles(ctx, m.store.DB(), profileIDs)
	if err != nil {
		return nil, err
	}
	for _, profile := range profilesByID {
		profileCounts := counts[profile.ID]
		profileRequirements := requirements[profile.ID]
		profiles = append(profiles, WithCounts{
			Profile: profile, WorkItems: profileCounts.workItems, NeedsSetup: len(profileRequirements) > 0,
			CredentialRequirements: profileRequirements,
		})
	}
	return profiles, nil
}

func listCredentialRequirements(
	ctx context.Context,
	q queryer,
	profileID string,
) (result []CredentialRequirement, err error) {
	rows, err := q.QueryContext(ctx, `
		SELECT provider, slot, source_extension
		FROM profile_credential_requirements
		WHERE profile_id = ?
		ORDER BY provider ASC, slot ASC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("profile: list credential requirements: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("profile: close credential requirement rows: %w", closeErr))
		}
	}()
	result = make([]CredentialRequirement, 0)
	for rows.Next() {
		var item CredentialRequirement
		if err := rows.Scan(&item.Provider, &item.Slot, &item.SourceExtension); err != nil {
			return nil, fmt.Errorf("profile: scan credential requirement: %w", err)
		}
		item.Missing = true
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profile: iterate credential requirements: %w", err)
	}
	return result, nil
}

func listCredentialRequirementsForProfiles(
	ctx context.Context,
	q queryer,
	profileIDs []string,
) (requirements map[string][]CredentialRequirement, err error) {
	requirements = make(map[string][]CredentialRequirement, len(profileIDs))
	if len(profileIDs) == 0 {
		return requirements, nil
	}
	for _, profileID := range profileIDs {
		requirements[profileID] = make([]CredentialRequirement, 0)
	}
	placeholders := makePlaceholders(len(profileIDs))
	args := make([]any, 0, len(profileIDs))
	for _, profileID := range profileIDs {
		args = append(args, profileID)
	}
	rows, err := q.QueryContext(ctx, `
		SELECT profile_id, provider, slot, source_extension
		FROM profile_credential_requirements
		WHERE profile_id IN (`+placeholders+`)
		ORDER BY profile_id ASC, provider ASC, slot ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("profile: list credential requirements: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("profile: close credential requirement rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		var profileID string
		item := CredentialRequirement{Missing: true}
		if err := rows.Scan(&profileID, &item.Provider, &item.Slot, &item.SourceExtension); err != nil {
			return nil, fmt.Errorf("profile: scan credential requirement: %w", err)
		}
		requirements[profileID] = append(requirements[profileID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profile: iterate credential requirements: %w", err)
	}
	return requirements, nil
}

type profileCountsResult struct {
	workItems  int
	needsSetup bool
}

var ownedWorkTables = []string{
	"sessions", "tasks", "loop_runs", "automation_jobs", "automation_triggers",
	"automation_suggestions", "bridge_instances", "worktrees", "network_channels",
	"network_direct_rooms", "network_threads", "network_work", "notification_cursors",
	"tool_approval_grants", "dead_entities", "token_usage_daily",
	"calls", "call_messages",
}

func (m *Manager) profileCounts(ctx context.Context, q queryer, profileID string) (profileCountsResult, error) {
	var result profileCountsResult
	for _, table := range ownedWorkTables {
		var count int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE profile_id = ?`, profileID).
			Scan(&count); err != nil {
			return profileCountsResult{}, fmt.Errorf("profile: count %s ownership: %w", table, err)
		}
		result.workItems += count
	}
	var requirements int
	if err := q.QueryRowContext(
		ctx, `SELECT COUNT(*) FROM profile_credential_requirements WHERE profile_id = ?`, profileID,
	).Scan(&requirements); err != nil {
		return profileCountsResult{}, fmt.Errorf("profile: count credential requirements: %w", err)
	}
	result.needsSetup = requirements > 0
	return result, nil
}

func (m *Manager) profileCountsForProfiles(
	ctx context.Context,
	q queryer,
	profileIDs []string,
) (map[string]profileCountsResult, error) {
	counts := make(map[string]profileCountsResult, len(profileIDs))
	if len(profileIDs) == 0 {
		return counts, nil
	}
	for _, profileID := range profileIDs {
		counts[profileID] = profileCountsResult{}
	}
	placeholders := makePlaceholders(len(profileIDs))
	args := make([]any, 0, len(profileIDs))
	for _, profileID := range profileIDs {
		args = append(args, profileID)
	}
	for _, table := range ownedWorkTables {
		rows, err := q.QueryContext(
			ctx,
			`SELECT profile_id, COUNT(*) FROM `+table+` WHERE profile_id IN (`+placeholders+`) GROUP BY profile_id`,
			args...)
		if err != nil {
			return nil, fmt.Errorf("profile: count %s ownership: %w", table, err)
		}
		for rows.Next() {
			var profileID string
			var count int
			if err := rows.Scan(&profileID, &count); err != nil {
				closeErr := rows.Close()
				return nil, errors.Join(fmt.Errorf("profile: scan %s ownership count: %w", table, err), closeErr)
			}
			counts[profileID] = profileCountsResult{
				workItems: counts[profileID].workItems + count,
			}
		}
		if err := rows.Err(); err != nil {
			closeErr := rows.Close()
			return nil, errors.Join(fmt.Errorf("profile: iterate %s ownership counts: %w", table, err), closeErr)
		}
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("profile: close %s ownership rows: %w", table, err)
		}
	}
	return counts, nil
}

func makePlaceholders(count int) string {
	placeholders := make([]string, count)
	for index := range placeholders {
		placeholders[index] = "?"
	}
	return strings.Join(placeholders, ",")
}

func (m *Manager) UpdateIdentity(
	ctx context.Context,
	name string,
	patch IdentityPatch,
) (Profile, error) {
	name = strings.TrimSpace(name)
	var updated Profile
	err := m.write(ctx, "update profile identity", func(exec globaldb.ProfileWriteExecutor) error {
		current, err := getProfileByName(ctx, exec, name)
		if err != nil {
			return err
		}
		color, icon, emoji := current.Color, current.Icon, current.Emoji
		if patch.Color != nil {
			color = *patch.Color
		}
		if patch.Icon != nil {
			icon, emoji = *patch.Icon, ""
		}
		if patch.Emoji != nil {
			emoji, icon = *patch.Emoji, ""
		}
		color, icon, emoji, err = normalizeIdentity(color, icon, emoji)
		if err != nil {
			return err
		}
		result, err := exec.ExecContext(
			ctx, `UPDATE profiles SET color = ?, icon = ?, emoji = ? WHERE id = ?`,
			color, nullableString(icon), nullableString(emoji), current.ID,
		)
		if err != nil {
			return fmt.Errorf("profile: update identity for %q: %w", name, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("profile: inspect identity update for %q: %w", name, err)
		}
		if affected != 1 {
			return fmt.Errorf("profile: update identity for %q affected %d rows", name, affected)
		}
		updated, err = getProfileByName(ctx, exec, name)
		return err
	})
	if err != nil {
		return Profile{}, err
	}
	m.recordEvent(eventspkg.ProfileIdentityUpdated, updated, "")
	return updated, nil
}

func mapNameConstraint(err error, name string) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique") || strings.Contains(message, "profile_name_reserved") {
		return domainError(
			"profile_name_taken",
			fmt.Sprintf("profile name %q is already held", name),
			"choose another profile name",
			ErrNameTaken,
		)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domainError("profile_not_found", "profile was not found", "run compozy profile list", ErrNotFound)
	}
	return err
}
