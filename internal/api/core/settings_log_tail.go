package core

import (
	"bufio"

	"errors"
	"fmt"
	"io"

	"os"

	"strings"
	"time"

	automationmodel "github.com/compozy/agh/internal/automation/model"

	settingspkg "github.com/compozy/agh/internal/settings"
)

func settingsRestartStatusURL(operationID string) string {
	return settingsRestartStatusPathPrefix + strings.TrimSpace(operationID)
}

func openSettingsLogTailFile(path string) (*os.File, os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open settings log tail file %q: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("stat settings log tail file %q: %w", path, err)
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("seek settings log tail file %q: %w", path, err)
	}
	return file, info, nil
}

func settingsLogTailPollInterval(interval time.Duration) time.Duration {
	if interval > 0 {
		return interval
	}
	return defaultPollInterval
}

func settingsLogTailRotated(path string, initial os.FileInfo, file *os.File) (bool, error) {
	current, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("stat settings log tail file %q: %w", path, err)
	}
	if initial != nil && !os.SameFile(initial, current) {
		return true, nil
	}
	if file != nil {
		offset, seekErr := file.Seek(0, io.SeekCurrent)
		if seekErr != nil {
			return false, fmt.Errorf("read settings log tail offset: %w", seekErr)
		}
		if current.Size() < offset {
			return true, nil
		}
	}
	return false, nil
}

func (h *BaseHandlers) drainSettingsLogTail(
	writer FlushWriter,
	reader *bufio.Reader,
	partial *string,
) error {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if partial != nil {
					*partial += line
				}
				return nil
			}
			return fmt.Errorf("read settings log tail: %w", err)
		}
		if partial != nil {
			line = *partial + line
			*partial = ""
		}
		h.writeSSEBestEffort(writer, SSEMessage{
			Name: "log",
			Data: SettingsLogTailEventPayload{Line: strings.TrimRight(line, "\r\n")},
		})
	}
}

func findSettingsProvider(values []settingspkg.ProviderItem, name string) (settingspkg.ProviderItem, bool) {
	for idx := range values {
		value := &values[idx]
		if strings.TrimSpace(value.Name) == name {
			return *value, true
		}
	}
	return settingspkg.ProviderItem{}, false
}

func findSettingsSandbox(values []settingspkg.SandboxItem, name string) (settingspkg.SandboxItem, bool) {
	for _, value := range values {
		if strings.TrimSpace(value.Name) == name {
			return value, true
		}
	}
	return settingspkg.SandboxItem{}, false
}

func automationFireLimitFromPayload(
	payload automationmodel.FireLimitConfig,
) automationmodel.FireLimitConfig {
	return payload
}
