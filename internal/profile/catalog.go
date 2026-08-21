package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

func (m *Manager) GetByName(ctx context.Context, name string) (Profile, error) {
	if ctx == nil {
		return Profile{}, errors.New("profile: get context is required")
	}
	return getProfileByName(ctx, m.store.DB(), strings.TrimSpace(name))
}

func (m *Manager) List(ctx context.Context) (profiles []ProfileWithCounts, err error) {
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
	profiles = make([]ProfileWithCounts, 0)
	for rows.Next() {
		profile, scanErr := scanProfile(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("profile: scan list row: %w", scanErr)
		}
		counts, countErr := m.profileCounts(ctx, m.store.DB(), profile.ID)
		if countErr != nil {
			return nil, countErr
		}
		profiles = append(profiles, ProfileWithCounts{
			Profile: profile, WorkItems: counts.workItems, NeedsSetup: counts.needsSetup,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profile: iterate list: %w", err)
	}
	return profiles, nil
}

type profileCountsResult struct {
	workItems  int
	needsSetup bool
}

var ownedWorkTables = []string{
	"sessions", "tasks", "loop_runs", "automation_jobs", "automation_triggers",
	"automation_suggestions", "bridge_instances", "worktrees", "network_channels",
	"network_direct_rooms", "network_threads", "network_work", "notification_cursors",
	"tool_approval_grants", "event_summaries", "dead_entities", "token_usage_daily",
}

func (m *Manager) profileCounts(ctx context.Context, q queryer, profileID string) (profileCountsResult, error) {
	var result profileCountsResult
	for _, table := range ownedWorkTables {
		var count int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE profile_id = ?`, profileID).Scan(&count); err != nil {
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
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return fmt.Errorf("profile: update identity for %q affected %d rows: %w", name, affected, err)
		}
		updated, err = getProfileByName(ctx, exec, name)
		return err
	})
	if err != nil {
		return Profile{}, err
	}
	m.recordEvent("profile.identity_updated", updated, "")
	return updated, nil
}

func mapNameConstraint(err error, name string) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique") || strings.Contains(message, "profile_name_reserved") {
		return domainError(
			"profile_name_taken", fmt.Sprintf("profile name %q is already held", name), "choose another profile name", ErrNameTaken,
		)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domainError("profile_not_found", "profile was not found", "run compozy profile list", ErrNotFound)
	}
	return err
}
