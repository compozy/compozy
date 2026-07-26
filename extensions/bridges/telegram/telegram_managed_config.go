package main

import (
	"context"

	"errors"
	"fmt"

	"net/http"

	"os"

	"strings"

	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *telegramProvider) prepareTelegramManagedConfigs(
	_ context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	if len(configs) == 0 {
		return configs, nil
	}
	requestedListen := strings.TrimSpace(os.Getenv(telegramListenAddrEnv))
	usedPaths := make(map[string]string, len(configs))

	for idx := range configs {
		requestedListen = updateTelegramRequestedListen(&configs[idx], requestedListen)
		markDuplicateTelegramWebhookPath(&configs[idx], usedPaths)
	}
	if p.lifecycle.Stopped() {
		closeTelegramBatchers(configs)
		return nil, bridgesdk.ErrProviderStopped
	}
	applyTelegramListenErrors(configs, requestedListen, p.startServer)
	p.mu.Lock()
	p.listenAddr = requestedListen
	p.mu.Unlock()
	return configs, nil
}

func updateTelegramRequestedListen(cfg *resolvedInstanceConfig, requestedListen string) string {
	if cfg == nil || cfg.listenAddr == "" {
		return requestedListen
	}
	if requestedListen == "" {
		return cfg.listenAddr
	}
	if requestedListen != cfg.listenAddr && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"telegram: instance %q configured incompatible listen_addr %q (runtime uses %q)",
			cfg.instanceID,
			cfg.listenAddr,
			requestedListen,
		)
	}
	return requestedListen
}

func markDuplicateTelegramWebhookPath(cfg *resolvedInstanceConfig, usedPaths map[string]string) {
	if cfg == nil || cfg.webhookPath == "" {
		return
	}
	if owner, ok := usedPaths[cfg.webhookPath]; ok && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"telegram: webhook path %q is shared by %q and %q",
			cfg.webhookPath,
			owner,
			cfg.instanceID,
		)
	}
	usedPaths[cfg.webhookPath] = cfg.instanceID
}

func closeTelegramBatchers(configs []resolvedInstanceConfig) {
	for idx := range configs {
		if configs[idx].batcher != nil {
			configs[idx].batcher.Close()
			configs[idx].batcher = nil
		}
	}
}

func applyTelegramListenErrors(
	configs []resolvedInstanceConfig,
	requestedListen string,
	startServer func(string) error,
) {
	if requestedListen == "" {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = errors.New(
					"telegram: webhook listen address is required",
				)
			}
		}
		return
	}

	if err := startServer(requestedListen); err != nil {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = err
			}
		}
	}
}

func (p *telegramProvider) populateTelegramInitialStates(
	ctx context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	for idx := range configs {
		status, degradation, err := p.determineInitialState(ctx, &configs[idx])
		if err != nil {
			p.setLastError(err)
		}
		configs[idx].initialStatus = status
		configs[idx].initialDegradation = degradation
	}
	return configs, nil
}

func writeTelegramWebhookOK(w http.ResponseWriter) error {
	if w == nil {
		return nil
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	return err
}
