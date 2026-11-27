package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/zalando/go-keyring"
)

const gatewayUpgradeURL = "https://compozy.com/blog/compozy-0-8-remote-computers"

func (d *Daemon) migrateRemovedGateway(ctx context.Context, state *bootState) {
	notices, ok := state.registry.(interface {
		RecordGatewayRemovalProfiles(context.Context, int, time.Time) error
	})
	if !ok {
		return
	}
	keys, err := config.MigrateRemovedGateway(d.homePaths.ConfigFile, func(count int) error {
		return notices.RecordGatewayRemovalProfiles(ctx, count, d.now())
	})
	if err != nil {
		state.logger.WarnContext(ctx, "config_keys_removal_failed", "error", err)
		return
	}
	if len(keys) == 0 {
		return
	}
	state.logger.WarnContext(ctx, "config_keys_removed", "keys", keys, "release_note", gatewayUpgradeURL)
	dir := filepath.Join(d.homePaths.HomeDir, "gateway")
	if err := purgeRemovedGatewayCredentials(dir); err != nil {
		state.logger.WarnContext(ctx, "gateway_credentials_cleanup_failed", "error", err)
	}
}

func purgeRemovedGatewayCredentials(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() {
		return err
	}
	credentials := filepath.Join(dir, "credentials")
	info, err = os.Lstat(credentials)
	if errors.Is(err, os.ErrNotExist) {
		return removeEmptyGatewayDir(dir)
	}
	if err != nil || !info.IsDir() {
		return err
	}
	return errors.Join(
		deleteGatewayCredentials(credentials),
		removeEmptyGatewayDir(credentials),
		removeEmptyGatewayDir(dir),
	)
}

func removeEmptyGatewayDir(dir string) error {
	err := os.Remove(dir)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
		return nil
	}
	return err
}

func deleteGatewayCredentials(dir string) error {
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	scope := sha256.Sum256([]byte(canonical))
	suffix := "@" + hex.EncodeToString(scope[:16])
	var cleanupErr error
	for _, entry := range entries {
		profile, ok := strings.CutSuffix(entry.Name(), ".cred")
		if !ok || !entry.Type().IsRegular() {
			continue
		}
		if err := keyring.Delete(
			"compozy-gateway",
			profile+suffix,
		); err != nil &&
			!errors.Is(err, keyring.ErrNotFound) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}
