package profile

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/oklog/ulid"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func formatTimestamp(value time.Time) string { return storepkg.FormatTimestamp(value) }

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := storepkg.ParseTimestamp(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("profile: parse timestamp %q: %w", value, err)
	}
	return parsed, nil
}

func (m *Manager) newProfileID() (string, error) {
	id, err := ulid.New(ulid.Timestamp(m.now().UTC()), m.entropy)
	if err != nil {
		return "", fmt.Errorf("profile: generate profile id: %w", err)
	}
	return id.String(), nil
}

func (m *Manager) newOperationID() (string, error) {
	id, err := ulid.New(ulid.Timestamp(m.now().UTC()), m.entropy)
	if err != nil {
		return "", fmt.Errorf("profile: generate lifecycle operation id: %w", err)
	}
	return "op_" + id.String(), nil
}

func fingerprint(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("profile: encode plan revision input: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func scanProfile(scanner interface{ Scan(...any) error }) (Profile, error) {
	var profile Profile
	var state, createdAt string
	var icon, emoji, archivedAt sql.NullString
	if err := scanner.Scan(
		&profile.ID,
		&profile.Name,
		&profile.Color,
		&icon,
		&emoji,
		&state,
		&createdAt,
		&archivedAt,
	); err != nil {
		return Profile{}, err
	}
	profile.Icon = icon.String
	profile.Emoji = emoji.String
	profile.State = State(state)
	parsedCreatedAt, err := parseTimestamp(createdAt)
	if err != nil {
		return Profile{}, err
	}
	profile.CreatedAt = parsedCreatedAt
	if archivedAt.Valid {
		parsedArchivedAt, err := parseTimestamp(archivedAt.String)
		if err != nil {
			return Profile{}, err
		}
		profile.ArchivedAt = &parsedArchivedAt
	}
	return profile, nil
}

func profileSelectSQL() string {
	return `SELECT id, name, color, icon, emoji, state, created_at, archived_at FROM profiles`
}

func getProfileByName(ctx context.Context, q queryer, name string) (Profile, error) {
	profile, err := scanProfile(q.QueryRowContext(ctx, profileSelectSQL()+` WHERE name = ?`, name))
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, domainError(
			"profile_not_found", fmt.Sprintf("profile %q was not found", name), "run compozy profile list", ErrNotFound,
		)
	}
	if err != nil {
		return Profile{}, fmt.Errorf("profile: get %q: %w", name, err)
	}
	return profile, nil
}

func getProfileByID(ctx context.Context, q queryer, id string) (Profile, error) {
	profile, err := scanProfile(q.QueryRowContext(ctx, profileSelectSQL()+` WHERE id = ?`, strings.TrimSpace(id)))
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, domainError(
			"profile_not_found",
			fmt.Sprintf("profile id %q was not found", id),
			"run compozy profile list",
			ErrNotFound,
		)
	}
	if err != nil {
		return Profile{}, fmt.Errorf("profile: get id %q: %w", id, err)
	}
	return profile, nil
}

func (m *Manager) write(
	ctx context.Context,
	action string,
	run func(globaldb.ProfileWriteExecutor) error,
) error {
	if ctx == nil {
		return fmt.Errorf("profile: %s context is required", action)
	}
	return m.store.ExecuteProfileWrite(ctx, action, run)
}

func requireContext(ctx context.Context, action string) error {
	if ctx == nil {
		return fmt.Errorf("profile: %s context is required", action)
	}
	return nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
