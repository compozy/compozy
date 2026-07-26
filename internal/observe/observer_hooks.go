package observe

import (
	"context"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"strings"

	"time"

	"github.com/compozy/agh/internal/store"
)

// AttachHooks swaps in the live hook catalog source after the hook runtime is built.
func (o *Observer) AttachHooks(source HookCatalogSource) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.hookCatalogSource = source
}

func (o *Observer) hookDBPath(sessionID string) string {
	return store.SessionDBFile(filepath.Join(o.homePaths.SessionsDir, strings.TrimSpace(sessionID)))
}

func (o *Observer) openHookRunStore(ctx context.Context, sessionID string) (HookRunStore, func() error, error) {
	if o == nil {
		return nil, nil, errors.New("observe: observer is required")
	}
	if ctx == nil {
		return nil, nil, errors.New("observe: hook run context is required")
	}

	target, err := sanitizeHookSessionID(sessionID)
	if err != nil {
		return nil, nil, err
	}

	dbPath := o.hookDBPath(target)
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, os.ErrNotExist
		}
		return nil, nil, fmt.Errorf("observe: stat hook database for %q: %w", target, err)
	}

	openStore := o.openHookStore
	if openStore == nil {
		return nil, nil, errors.New("observe: hook store opener is required")
	}

	storeHandle, err := openStore(ctx, target, dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("observe: open hook database for %q: %w", target, err)
	}

	cleanup := func() error {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return storeHandle.Close(closeCtx)
	}
	return storeHandle, cleanup, nil
}

// Close flushes and closes the backing global registry.
func (o *Observer) Close(ctx context.Context) error {
	return o.registry.Close(ctx)
}
