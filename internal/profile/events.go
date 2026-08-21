package profile

import (
	"context"
	"database/sql"
	"errors"
)

func (m *Manager) recordOperationEvent(ctx context.Context, name, opID string, cause error) {
	if m.events == nil {
		return
	}
	var event Event
	event.Name = name
	event.OperationID = opID
	if cause != nil {
		event.Error = cause.Error()
	}
	err := m.store.DB().QueryRowContext(ctx, `
		SELECT o.profile_id, COALESCE(p.name, o.new_name, o.old_name, '')
		FROM profile_lifecycle_ops o
		LEFT JOIN profiles p ON p.id = o.profile_id
		WHERE o.id = ?`, opID).Scan(&event.ProfileID, &event.ProfileName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			m.logger.Warn("profile lifecycle event lookup failed", "operation_id", opID, "error", err)
		}
		return
	}
	m.events.RecordProfileEvent(event)
}
