package daemon

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func instanceCeilingReader(repo *globaldb.InstanceRepo) session.InstanceCeilingReader {
	return func(ctx context.Context, sessionID, instanceID string) (*session.InstanceCeiling, error) {
		var caller *agentinstance.Caller
		starting := instanceID != ""
		if instanceID == "" {
			resolved, err := repo.InstanceCaller(ctx, sessionID)
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			if resolved.InstanceID == "" {
				return nil, nil
			}
			instanceID, caller = resolved.InstanceID, &resolved
		}
		instance, err := repo.Get(ctx, instanceID)
		if err != nil {
			return nil, err
		}
		var ceiling store.SessionPermissionPolicy
		if err := json.Unmarshal([]byte(instance.PermissionCeiling), &ceiling); err != nil {
			return nil, err
		}
		grant := instance.GrantCeiling
		if caller != nil {
			ceiling = agentinstance.IntersectPermissions(ceiling, caller.PermissionPolicy)
			grant = workspaceaccess.NarrowGrant(grant, caller.Grant)
		}
		return &session.InstanceCeiling{
			Policy: ceiling,
			Grant:  grant,
			Main:   starting || sessionID == instance.MainSessionID,
		}, nil
	}
}
