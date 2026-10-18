package profile

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

func enabledProfileAutomations(ctx context.Context, q queryer, profileID string) ([]string, error) {
	return stringColumn(ctx, q, "enabled profile automations", `
		SELECT SUBSTR(r.kind, LENGTH('automation.') + 1) || ':' || r.id AS identity
		FROM resource_records r
		LEFT JOIN automation_job_overlays j ON r.kind = 'automation.job' AND j.job_id = r.id
		LEFT JOIN automation_trigger_overlays t ON r.kind = 'automation.trigger' AND t.trigger_id = r.id
		WHERE r.kind IN ('automation.job', 'automation.trigger')
		  AND json_extract(r.spec_json, '$.profile_id') = ?
		  AND CASE WHEN json_extract(r.spec_json, '$.source') IN ('config', 'package')
		      THEN COALESCE(j.enabled_override, t.enabled_override, json_extract(r.spec_json, '$.enabled'))
		      ELSE json_extract(r.spec_json, '$.enabled') END = 1
		UNION
		SELECT 'job:' || j.id FROM automation_jobs j
		LEFT JOIN automation_job_overlays o ON o.job_id = j.id
		WHERE j.profile_id = ?
		  AND CASE WHEN j.source IN ('config', 'package') THEN COALESCE(o.enabled_override, j.enabled)
		      ELSE j.enabled END = 1
		  AND NOT EXISTS (SELECT 1 FROM resource_records r WHERE r.kind = 'automation.job' AND r.id = j.id)
		UNION
		SELECT 'trigger:' || t.id FROM automation_triggers t
		LEFT JOIN automation_trigger_overlays o ON o.trigger_id = t.id
		WHERE t.profile_id = ?
		  AND CASE WHEN t.source IN ('config', 'package') THEN COALESCE(o.enabled_override, t.enabled)
		      ELSE t.enabled END = 1
		  AND NOT EXISTS (SELECT 1 FROM resource_records r WHERE r.kind = 'automation.trigger' AND r.id = t.id)
		ORDER BY identity`, profileID, profileID, profileID)
}

func pauseProfileAutomations(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	profileID string,
	automations []string,
	now string,
) error {
	for _, identity := range automations {
		kind, id, found := strings.Cut(identity, ":")
		if !found || (kind != "job" && kind != "trigger") {
			return fmt.Errorf("profile: invalid automation identity %q", identity)
		}
		if err := pauseProfileAutomation(ctx, exec, profileID, kind, id, now); err != nil {
			return fmt.Errorf("profile: pause %s %q: %w", kind, id, err)
		}
	}
	return nil
}

func pauseProfileAutomation(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	profileID, kind, id, now string,
) error {
	if _, err := exec.ExecContext(ctx, `
		UPDATE resource_records
		SET spec_json = json_set(spec_json, '$.enabled', json('false')), version = version + 1, updated_at = ?
		WHERE kind = ? AND id = ? AND json_extract(spec_json, '$.profile_id') = ?
		  AND json_extract(spec_json, '$.source') NOT IN ('config', 'package')`,
		now, "automation."+kind, id, profileID,
	); err != nil {
		return err
	}
	// dynamic-sql: the validated automation kind chooses its existing definition and overlay tables.
	table, overlay := "automation_"+kind+"s", "automation_"+kind+"_overlays"
	key, catalog := kind+"_id", "automation_"+kind+"_catalog_entries"
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO `+overlay+` (`+key+`, enabled_override, updated_at)
		SELECT id, 0, ? FROM resource_records
		WHERE kind = ? AND id = ? AND json_extract(spec_json, '$.profile_id') = ?
		  AND json_extract(spec_json, '$.source') IN ('config', 'package')
		UNION
		SELECT id, 0, ? FROM `+table+` WHERE id = ? AND profile_id = ? AND source IN ('config', 'package')
		ON CONFLICT (`+key+`) DO UPDATE SET enabled_override = 0, updated_at = excluded.updated_at`,
		now, "automation."+kind, id, profileID, now, id, profileID,
	); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `
		UPDATE `+table+` SET enabled = 0, updated_at = ?
		WHERE id = ? AND profile_id = ? AND source NOT IN ('config', 'package')`, now, id, profileID,
	); err != nil {
		return err
	}
	_, err := exec.ExecContext(ctx, `
		UPDATE `+catalog+` SET enabled = 0
		WHERE `+key+` = ? AND EXISTS (
			SELECT 1 FROM `+table+` WHERE id = ? AND profile_id = ? AND source NOT IN ('config', 'package')
		)`, id, id, profileID)
	return err
}

func (m *Manager) reconcileProfileAutomations(ctx context.Context, opID string) error {
	if m.automations == nil {
		return nil
	}
	var profileID string
	if err := m.store.DB().QueryRowContext(ctx,
		`SELECT profile_id FROM profile_lifecycle_ops WHERE id = ?`, opID,
	).Scan(&profileID); err != nil {
		return fmt.Errorf("profile: read automation owner for operation %s: %w", opID, err)
	}
	if err := m.automations.ReconcileProfileAutomations(ctx, profileID); err != nil {
		return fmt.Errorf("profile: reconcile archived automations: %w", err)
	}
	return nil
}
