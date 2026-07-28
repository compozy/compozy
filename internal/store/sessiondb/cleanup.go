package sessiondb

import "errors"

func joinSessionCleanupError(target *error, cleanupErr error) {
	if target == nil || cleanupErr == nil {
		return
	}
	*target = errors.Join(*target, cleanupErr)
}
