package memory

import (
	"context"
	"errors"
)

// OpenCatalog opens and migrates the optional catalog database. Composition
// roots call this explicitly after acquiring their database lifecycle lock.
func (s *Store) OpenCatalog(ctx context.Context) error {
	if s == nil || s.catalog == nil {
		return nil
	}
	if err := s.catalog.open(ctx); err != nil {
		return err
	}
	if err := s.reconcileProfileMemoryMaintenance(ctx); err != nil {
		return errors.Join(err, s.catalog.close(context.WithoutCancel(ctx)))
	}
	return nil
}

// CloseCatalog checkpoints and closes the optional catalog database.
func (s *Store) CloseCatalog(ctx context.Context) error {
	if s == nil || s.catalog == nil {
		return nil
	}
	return s.catalog.close(ctx)
}
