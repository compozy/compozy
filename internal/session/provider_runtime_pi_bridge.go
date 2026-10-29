package session

import (
	"context"
	_ "embed"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/providerenv"
)

//go:embed pi_native_tools.js
var piNativeToolsExtension []byte

const (
	piBridgeTimeout = 10 * time.Second
	piBridgeWindows = "windows"
)

type piBridgeStatus struct {
	Nonce string `json:"nonce"`
	Code  string `json:"code"`
	Count int    `json:"count"`
}

type piBridgeServer struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

type piBridgeConfig struct {
	Nonce     string           `json:"nonce"`
	Status    string           `json:"status"`
	TimeoutMS int64            `json:"timeout_ms"`
	Servers   []piBridgeServer `json:"servers"`
}

func preparePiNativeTools(session *Session, opts acp.StartOpts) (_ acp.StartOpts, _ string, err error) {
	if session == nil || session.sessionDir == "" {
		return opts, "", errors.New("session: pi bridge requires session storage")
	}
	root := filepath.Join(session.sessionDir, "provider-runtime", "pi-bridge")
	if err := providerenv.EnsurePrivateDirUnder(session.sessionDir, root); err != nil {
		return opts, "", err
	}
	dir, err := os.MkdirTemp(root, "launch-")
	if err != nil {
		return opts, "", err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(dir))
		}
	}()
	config := piBridgeConfig{
		Nonce:     filepath.Base(dir),
		Status:    filepath.Join(dir, "status.json"),
		TimeoutMS: piBridgeTimeout.Milliseconds(),
	}
	for i, server := range opts.MCPServers {
		if server.EffectiveTransport() != compozyconfig.MCPServerTransportStdio || server.Command == "" {
			return opts, "", errors.New("session: pi bridge requires stdio MCP servers")
		}
		cwd := server.CWD
		if cwd == "" {
			cwd = opts.Cwd
		}
		config.Servers = append(config.Servers, piBridgeServer{
			Name: "compozy_" + strings.ReplaceAll(config.Nonce, "-", "_") + "_" + strconv.Itoa(i),
			Config: map[string]any{
				"command":  server.Command,
				"args":     server.Args,
				"env":      server.Env,
				"cwd":      cwd,
				"exposure": "codemode",
			},
		})
	}
	configPath := filepath.Join(dir, "bridge.json")
	if err := writeProviderJSON(configPath, config); err != nil {
		return opts, "", err
	}
	extensionPath := filepath.Join(dir, "native-tools.js")
	if err := fileutil.AtomicWriteFile(extensionPath, piNativeToolsExtension, 0o600); err != nil {
		return opts, "", err
	}
	command, _ := providerLookupEnv(opts.Env)("PI_ACP_PI_COMMAND")
	if command == "" {
		command = "pi"
		if runtime.GOOS == piBridgeWindows {
			command = "pi.cmd"
		}
	}
	wrapper := filepath.Join(dir, "pi")
	script := "#!/bin/sh\nexec \"$COMPOZY_PI_COMMAND\" --extension \"$COMPOZY_PI_EXTENSION\" \"$@\"\n"
	if runtime.GOOS == piBridgeWindows {
		wrapper += ".cmd"
		script = "@\"%COMPOZY_PI_COMMAND%\" --extension \"%COMPOZY_PI_EXTENSION%\" %*\r\n"
	}
	if err := fileutil.AtomicWriteFile(wrapper, []byte(script), 0o700); err != nil {
		return opts, "", err
	}
	for key, value := range map[string]string{
		"PI_ACP_PI_COMMAND":    wrapper,
		"COMPOZY_PI_COMMAND":   command,
		"COMPOZY_PI_EXTENSION": extensionPath,
		"COMPOZY_PI_BRIDGE":    configPath,
	} {
		opts.Env = setSessionStartEnvValue(opts.Env, key, value)
	}
	// The extension owns this launch even if a future ACP adapter adds MCP passthrough.
	opts.MCPServers = nil
	return opts, config.Status, nil
}

func readPiBridgeStatus(ctx context.Context, path string) string {
	deadline := time.NewTimer(piBridgeTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var status piBridgeStatus
			if json.Unmarshal(data, &status) != nil || status.Nonce != filepath.Base(filepath.Dir(path)) {
				return "pi_native_tools_status_invalid"
			}
			switch status.Code {
			case "ready":
				if status.Count > 0 {
					return ""
				}
			case "pi_native_tools_unsupported",
				"pi_native_tools_registration_failed",
				"pi_native_tools_connection_failed":
				return status.Code
			}
			return "pi_native_tools_status_invalid"
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "pi_native_tools_status_unavailable"
		}
		select {
		case <-ctx.Done():
			return "pi_native_tools_startup_canceled"
		case <-deadline.C:
			return "pi_native_tools_extension_unavailable"
		case <-ticker.C:
		}
	}
}
