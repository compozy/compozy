package memory

import "context"

// OpenCatalog opens and migrates the optional catalog database. Composition
// roots call this explicitly after acquiring their database lifecycle lock.
func (s *Store) OpenCatalog(ctx context.Context) error {
	if s == nil || s.catalog == nil {
		return nil
	}
	return s.catalog.open(ctx)
}

// CloseCatalog checkpoints and closes the optional catalog database.
func (s *Store) CloseCatalog(ctx context.Context) error {
	if s == nil || s.catalog == nil {
		return nil
	}
	return s.catalog.close(ctx)
}
