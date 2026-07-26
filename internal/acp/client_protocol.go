package acp

import (
	"errors"
	"fmt"

	"sort"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
	shellquote "github.com/kballard/go-shellquote"
)

func parseCommandString(command string) (string, []string, error) {
	parts, err := shellquote.Split(command)
	if err != nil {
		return "", nil, fmt.Errorf("acp: parse command %q: %w", command, err)
	}
	if len(parts) == 0 {
		return "", nil, errors.New("acp: command is empty")
	}
	return parts[0], parts[1:], nil
}

func toSDKMCPServers(servers []aghconfig.MCPServer) []acpsdk.McpServer {
	if len(servers) == 0 {
		return []acpsdk.McpServer{}
	}

	converted := make([]acpsdk.McpServer, 0, len(servers))
	for _, server := range servers {
		if server.EffectiveTransport() != aghconfig.MCPServerTransportStdio {
			continue
		}
		if strings.TrimSpace(server.Command) == "" {
			continue
		}
		envKeys := make([]string, 0, len(server.Env))
		for key := range server.Env {
			envKeys = append(envKeys, key)
		}
		sort.Strings(envKeys)

		env := make([]acpsdk.EnvVariable, 0, len(server.Env))
		for _, key := range envKeys {
			env = append(env, acpsdk.EnvVariable{Name: key, Value: server.Env[key]})
		}

		converted = append(converted, acpsdk.McpServer{
			Stdio: &acpsdk.McpServerStdio{
				Name:    server.Name,
				Command: server.Command,
				Args:    append([]string(nil), server.Args...),
				Env:     env,
			},
		})
	}
	return converted
}

func captureCaps(
	loadSession bool,
	modes *acpsdk.SessionModeState,
	configOptions []acpsdk.SessionConfigOption,
) Caps {
	caps := Caps{SupportsLoadSession: loadSession}
	if modes != nil {
		caps.SupportedModes = make([]string, 0, len(modes.AvailableModes))
		for _, mode := range modes.AvailableModes {
			caps.SupportedModes = append(caps.SupportedModes, string(mode.Id))
		}
	}
	caps.ConfigOptions = sessionConfigOptionsFromSDK(configOptions)
	return caps
}

func attachStderr(err error, stderr string) error {
	if strings.TrimSpace(stderr) == "" {
		return err
	}
	return fmt.Errorf("%w: stderr=%s", err, strings.TrimSpace(stderr))
}

func (d *Driver) waitForPromptQuiescence(active *activePromptState) {
	if active == nil || d.promptDrainWait <= 0 {
		return
	}
	timer := time.NewTimer(d.promptDrainWait)
	maxTimer := time.NewTimer(2 * d.promptDrainWait)
	defer timer.Stop()
	defer maxTimer.Stop()

	for {
		select {
		case <-active.activity:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(d.promptDrainWait)
		case <-timer.C:
			return
		case <-maxTimer.C:
			return
		}
	}
}
