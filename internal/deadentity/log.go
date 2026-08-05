package deadentity

import "github.com/compozy/compozy/internal/store"

func (s *Service) logStoreFailure(operation string, key store.DeadEntityKey, err error) {
	s.logger.Warn(
		"deadentity: durable transition failed open",
		"operation", operation,
		"workspace_id", key.WorkspaceID,
		"kind", key.Kind,
		"entity_id", key.EntityID,
		"error", err,
	)
}
