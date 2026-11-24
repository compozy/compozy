package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.ComputerStore = (*ComputersRepo)(nil)

func (r *ComputersRepo) ControllerComputers(ctx context.Context, owner string) (_ []store.Computer, err error) {
	tx, err := r.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { joinCleanupError(&err, rollbackTx(tx, "controller computers")) }()
	q := sqlcgen.New(tx)
	rows, err := q.ListControllerComputers(ctx, owner)
	if err != nil {
		return nil, err
	}
	routes, err := q.ListControllerRoutes(ctx, owner)
	if err != nil {
		return nil, err
	}
	byComputer := make(map[string][]store.ComputerRoute)
	for _, route := range routes {
		byComputer[route.ComputerID] = append(byComputer[route.ComputerID], computerRouteFromRow(route))
	}
	result := make([]store.Computer, 0, len(rows))
	for _, row := range rows {
		item, err := computerFromRow(row)
		if err != nil {
			return nil, err
		}
		item.Routes = byComputer[row.ID]
		result = append(result, item)
	}
	return result, nil
}

func computerFromRow(row sqlcgen.Computer) (store.Computer, error) {
	item := store.Computer{
		ID: remoteproto.ComputerID(row.ID), AccountID: row.AccountID, Name: row.Name, Enabled: row.Enabled != 0,
		Platform: row.Platform.String, Version: row.Version.String, TLSRootSHA: row.TlsRootSha256,
		Weight: int(row.Weight),
	}
	var err error
	item.AddedAt, err = store.ParseTimestamp(row.AddedAt)
	if err != nil {
		return store.Computer{}, err
	}
	if row.LastConnectedAt.Valid {
		at, err := store.ParseTimestamp(row.LastConnectedAt.String)
		if err != nil {
			return store.Computer{}, err
		}
		item.LastConnAt = &at
	}
	return item, nil
}

func computerRouteFromRow(row sqlcgen.ComputerRoute) store.ComputerRoute {
	route := store.ComputerRoute{
		ID: row.ID, Kind: remoteproto.RouteKind(row.Kind), Address: row.Address,
		Position: int(row.Position), Learned: row.Learned != 0,
	}
	if route.Kind == remoteproto.RouteSSH {
		route.SSH = &store.ComputerSSHTarget{
			User:  row.SshUser.String,
			Port:  int(row.SshPort.Int64),
			Alias: row.SshAlias.String,
		}
	}
	return route
}

func (r *ComputersRepo) AddComputer(ctx context.Context, item store.Computer) error {
	return r.executeWriteQuery(ctx, "globaldb.AddComputer", func(ctx context.Context, q *sqlcgen.Queries) error {
		err := q.AddControllerComputer(ctx, sqlcgen.AddControllerComputerParams{
			ID: string(item.ID), AccountID: item.AccountID, Name: item.Name,
			Enabled: boolToInt64(item.Enabled), Platform: nullableSessionStringPointer(&item.Platform),
			Version: nullableSessionStringPointer(&item.Version), TlsRootSha256: item.TLSRootSHA,
			Weight: int64(item.Weight), AddedAt: store.FormatTimestamp(item.AddedAt),
		})
		if err != nil {
			return err
		}
		for _, route := range item.Routes {
			if err := addComputerRoute(ctx, q, item.AccountID, item.ID, route); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ComputersRepo) UpdateComputer(
	ctx context.Context, owner string, id remoteproto.ComputerID, patch store.ComputerPatch,
) error {
	return r.executeWriteQuery(ctx, "globaldb.UpdateComputer", func(ctx context.Context, q *sqlcgen.Queries) error {
		count, err := q.UpdateControllerComputer(ctx, sqlcgen.UpdateControllerComputerParams{
			AccountID: owner, ID: string(id), Name: nullableSessionStringPointer(patch.Name),
			Enabled: nullableBool(patch.Enabled), Weight: nullableInt(patch.Weight),
		})
		return computerRowsChanged(count, err, store.ErrComputerNotFound)
	})
}

func (r *ComputersRepo) RemoveComputer(ctx context.Context, owner string, id remoteproto.ComputerID) error {
	return r.executeWriteQuery(ctx, "globaldb.RemoveComputer", func(ctx context.Context, q *sqlcgen.Queries) error {
		count, err := q.DeleteControllerComputer(ctx, sqlcgen.DeleteControllerComputerParams{
			AccountID: owner, ID: string(id),
		})
		return computerRowsChanged(count, err, store.ErrComputerNotFound)
	})
}

func (r *ComputersRepo) AddComputerRoute(
	ctx context.Context, owner string, id remoteproto.ComputerID, route store.ComputerRoute,
) error {
	return r.executeWriteQuery(ctx, "globaldb.AddComputerRoute", func(ctx context.Context, q *sqlcgen.Queries) error {
		return addComputerRoute(ctx, q, owner, id, route)
	})
}

func addComputerRoute(
	ctx context.Context, q *sqlcgen.Queries, owner string, id remoteproto.ComputerID, route store.ComputerRoute,
) error {
	input := sqlcgen.AddControllerRouteParams{
		RouteID: route.ID, ComputerID: string(id), AccountID: owner, Kind: string(route.Kind),
		Address: route.Address, Position: int64(route.Position), Learned: boolToInt64(route.Learned),
	}
	if ssh := route.SSH; ssh != nil {
		input.SshUser = nullableSessionStringPointer(&ssh.User)
		input.SshPort = nullableInt(&ssh.Port)
		input.SshAlias = nullableSessionStringPointer(&ssh.Alias)
	}
	return q.AddControllerRoute(ctx, input)
}

func (r *ComputersRepo) ForgetComputerRoute(
	ctx context.Context, owner string, id remoteproto.ComputerID, routeID string,
) error {
	return r.executeWriteQuery(
		ctx,
		"globaldb.ForgetComputerRoute",
		func(ctx context.Context, q *sqlcgen.Queries) error {
			count, err := q.ForgetControllerRoute(ctx, sqlcgen.ForgetControllerRouteParams{
				ComputerID: string(id), AccountID: owner, RouteID: routeID,
			})
			return computerRowsChanged(count, err, store.ErrComputerRouteNotFound)
		},
	)
}

func computerRowsChanged(count int64, err, notFound error) error {
	if err == nil && count == 0 {
		return notFound
	}
	return err
}

func (r *ComputersRepo) LearnComputerRoutes(
	ctx context.Context, owner string, id remoteproto.ComputerID, routes []store.ComputerRoute,
) ([]store.ComputerRoute, error) {
	return executeWriteQueryResult(ctx, r.repoBase, "globaldb.LearnComputerRoutes",
		func(ctx context.Context, q *sqlcgen.Queries) ([]store.ComputerRoute, error) {
			for _, route := range routes {
				if err := addComputerRoute(ctx, q, owner, id, route); err != nil {
					return nil, err
				}
			}
			if err := pruneComputerRoutes(ctx, q, owner, id, routes); err != nil {
				return nil, err
			}
			rows, err := q.GetControllerRoutes(ctx, sqlcgen.GetControllerRoutesParams{
				AccountID: owner, ComputerID: string(id),
			})
			if err != nil {
				return nil, err
			}
			result := make([]store.ComputerRoute, 0, len(rows))
			for _, row := range rows {
				result = append(result, computerRouteFromRow(row))
			}
			return result, nil
		})
}

func pruneComputerRoutes(
	ctx context.Context, q *sqlcgen.Queries, owner string, id remoteproto.ComputerID, routes []store.ComputerRoute,
) error {
	for _, kind := range []remoteproto.RouteKind{remoteproto.RouteDirect, remoteproto.RouteCloud} {
		addresses := make([]string, 0, len(routes))
		for _, route := range routes {
			if route.Kind == kind {
				addresses = append(addresses, route.Address)
			}
		}
		data, err := json.Marshal(addresses)
		if err != nil {
			return err
		}
		err = q.PruneControllerLearnedRoutes(ctx, sqlcgen.PruneControllerLearnedRoutesParams{
			ComputerID: string(id), AccountID: owner, Kind: string(kind), Addresses: string(data),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ComputersRepo) RecordComputerConnection(
	ctx context.Context, owner string, descriptor remoteproto.Descriptor, at time.Time,
) error {
	return r.executeWriteQuery(
		ctx,
		"globaldb.RecordComputerConnection",
		func(ctx context.Context, q *sqlcgen.Queries) error {
			return q.RecordControllerConnection(ctx, sqlcgen.RecordControllerConnectionParams{
				AccountID: owner, ID: string(descriptor.ComputerID),
				LastConnectedAt: sql.NullString{String: store.FormatTimestamp(at), Valid: true},
				Platform:        nullableSessionStringPointer(&descriptor.Platform),
				Version:         nullableSessionStringPointer(&descriptor.Version),
			})
		},
	)
}
