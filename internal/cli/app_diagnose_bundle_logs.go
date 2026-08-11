package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
)

const (
	appDiagnosticBundleLogMessageMaxBytes = 512
	appDiagnosticBundleInfoLogLevel       = "INFO"
	appDiagnosticBundleSecretTerm         = "SECRET"
	appDiagnosticBundleTokenTerm          = "TOKEN"
	appDiagnosticBundleConfigTerm         = "config"
	appDiagnosticBundleSessionTerm        = "session"
)

type appDiagnosticBundleLogRecord struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	BootID    string `json:"boot_id"`
	Message   string `json:"message"`
}

func collectAppDiagnosticBundleEntries(
	homePaths compozyconfig.HomePaths,
	report appDiagnosticReport,
	manifest []byte,
) ([]appDiagnosticBundleEntry, error) {
	if int64(len(manifest)) > appDiagnosticBundleContentMaxBytes {
		return nil, errors.New("app diagnose: diagnostic bundle manifest exceeds its size limit")
	}
	entries := []appDiagnosticBundleEntry{{Name: appDiagnosticBundleManifestFile, Data: manifest}}
	totalBytes := int64(len(manifest))
	for _, source := range []appDiagnosticBundleLogSource{
		{
			Name: appDiagnosticBundleDesktopLogFile,
			Path: filepath.Join(homePaths.LogsDir, appDiagnosticBundleDesktopLogFile),
		},
		{
			Name: appDiagnosticBundleBootstrapLogFile,
			Path: filepath.Join(homePaths.LogsDir, appDiagnosticBundleBootstrapLogFile),
		},
	} {
		entry, err := collectAppDiagnosticBundleLogEntry(source, report, homePaths)
		if err != nil {
			slog.Debug("skip optional desktop diagnostic log tail", "entry", source.Name, automationErrorKey, err)
			continue
		}
		if entry == nil || totalBytes+int64(len(entry.Data)) > appDiagnosticBundleContentMaxBytes {
			continue
		}
		totalBytes += int64(len(entry.Data))
		entries = append(entries, *entry)
	}
	return entries, nil
}

type appDiagnosticBundleLogSource struct {
	Name string
	Path string
}

func collectAppDiagnosticBundleLogEntry(
	source appDiagnosticBundleLogSource,
	report appDiagnosticReport,
	homePaths compozyconfig.HomePaths,
) (*appDiagnosticBundleEntry, error) {
	tail, err := readAppDiagnosticBundleLogTail(source.Path)
	if err != nil || len(tail) == 0 {
		return nil, err
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("app diagnose: resolve user home for log redaction: %w", err)
	}
	redacted := redactAppDiagnosticBundleLogTail(tail, report, homePaths, userHome)
	if len(redacted) == 0 {
		return nil, nil
	}
	return &appDiagnosticBundleEntry{Name: source.Name, Data: redacted}, nil
}

func readAppDiagnosticBundleLogTail(path string) (tail []byte, err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) || info != nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("app diagnose: inspect diagnostic log: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("app diagnose: open diagnostic log: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			closeErr = fmt.Errorf("app diagnose: close diagnostic log: %w", closeErr)
			if err == nil {
				err = closeErr
			} else {
				err = errors.Join(err, closeErr)
			}
		}
	}()
	if info.Size() > appDiagnosticBundleLogTailMaxBytes {
		if _, err := file.Seek(-appDiagnosticBundleLogTailMaxBytes, io.SeekEnd); err != nil {
			return nil, fmt.Errorf("app diagnose: seek diagnostic log tail: %w", err)
		}
	}
	tail, err = io.ReadAll(io.LimitReader(file, appDiagnosticBundleLogTailMaxBytes))
	if err != nil {
		return nil, fmt.Errorf("app diagnose: read diagnostic log tail: %w", err)
	}
	if info.Size() > appDiagnosticBundleLogTailMaxBytes {
		if index := bytes.IndexByte(tail, '\n'); index >= 0 {
			tail = tail[index+1:]
		} else {
			tail = nil
		}
	}
	return tail, nil
}

func redactAppDiagnosticBundleLogTail(
	tail []byte,
	report appDiagnosticReport,
	homePaths compozyconfig.HomePaths,
	userHome string,
) []byte {
	var output bytes.Buffer
	for line := range bytes.SplitSeq(tail, []byte{'\n'}) {
		redacted, ok := redactAppDiagnosticBundleLogLine(line, report, homePaths, userHome)
		if !ok || output.Len()+len(redacted) >= int(appDiagnosticBundleLogTailMaxBytes) {
			if ok {
				break
			}
			continue
		}
		if _, err := output.WriteString(redacted); err != nil {
			return nil
		}
		if err := output.WriteByte('\n'); err != nil {
			return nil
		}
	}
	return output.Bytes()
}

func redactAppDiagnosticBundleLogLine(
	line []byte,
	report appDiagnosticReport,
	homePaths compozyconfig.HomePaths,
	userHome string,
) (string, bool) {
	var record appDiagnosticBundleLogRecord
	if err := json.Unmarshal(line, &record); err != nil || record.BootID != report.BootID {
		return "", false
	}
	timestamp, err := time.Parse(time.RFC3339, record.Timestamp)
	if err != nil || !isAppDiagnosticBundleLogLevelAllowed(record.Level) ||
		containsAppDiagnosticBundleNonShareableContent(record.Message) {
		return "", false
	}
	message := redactAppDiagnosticBundleLogMessage(record.Message, homePaths, userHome)
	if message == "" {
		return "", false
	}
	encoded, err := json.Marshal(appDiagnosticBundleLogRecord{
		Timestamp: timestamp.UTC().Format(time.RFC3339),
		Level:     strings.ToLower(record.Level),
		BootID:    report.BootID,
		Message:   message,
	})
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func isAppDiagnosticBundleLogLevelAllowed(level string) bool {
	level = strings.ToLower(level)
	return level == automationErrorKey || level == "warn" ||
		strings.EqualFold(level, appDiagnosticBundleInfoLogLevel) || level == "debug"
}

func containsAppDiagnosticBundleNonShareableContent(message string) bool {
	message = strings.ToLower(message)
	for _, term := range []string{
		appDiagnosticBundleConfigTerm,
		appDiagnosticBundleSessionTerm,
		"transcript",
		"credential",
		"password",
		strings.ToLower(appDiagnosticBundleSecretTerm),
		strings.ToLower(appDiagnosticBundleTokenTerm),
		"authorization",
		"compozy.db",
		"sqlite",
	} {
		if strings.Contains(message, term) {
			return true
		}
	}
	return false
}

func redactAppDiagnosticBundleLogMessage(
	message string,
	homePaths compozyconfig.HomePaths,
	userHome string,
) string {
	redacted := message
	for _, path := range []string{
		homePaths.HomeDir,
		homePaths.LogsDir,
		filepath.Join(homePaths.LogsDir, appDiagnosticBundleDesktopLogFile),
	} {
		if path != "" && path != string(filepath.Separator) {
			redacted = strings.ReplaceAll(redacted, path, "[redacted]")
		}
	}
	if userHome != "" && userHome != string(filepath.Separator) {
		redacted = strings.ReplaceAll(redacted, userHome, "[redacted]")
	}
	return diagnosticspkg.RedactAndBound(redacted, appDiagnosticBundleLogMessageMaxBytes)
}
