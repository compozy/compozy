package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

const (
	mcpStdioHomeNamePrefix = "compozy-mcp-"
	// mcpStdioWaitDelay bounds Wait when a grandchild keeps the stderr pipe open.
	mcpStdioWaitDelay = 2 * time.Second
)

// mcpStdioSpec is the resolved, launch-ready identity of one stdio MCP server for one target.
type mcpStdioSpec struct {
	slot    string
	digest  string
	command string
	args    []string
	env     []string
	dir     string
	secrets []string
}

// mcpStdioLayout places isolated homes and shared package caches. The zero layout keeps
// per-launch temporary homes with caches inside them.
type mcpStdioLayout struct {
	homesDir string
	npmCache string
	uvCache  string
}

type mcpStdioLaunch struct {
	command *exec.Cmd
	cleanup func() error
}

func (e *CallExecutor) newMCPStdioLaunch(
	ctx context.Context,
	server compozyconfig.MCPServer,
	target mcpauth.Target,
) (mcpStdioLaunch, error) {
	spec, err := e.mcpStdioSpec(ctx, server, target)
	if err != nil {
		return mcpStdioLaunch{}, err
	}
	return newMCPStdioProcess(ctx, spec, mcpStdioLayout{}, nil)
}

func (e *CallExecutor) mcpStdioSpec(
	ctx context.Context,
	server compozyconfig.MCPServer,
	target mcpauth.Target,
) (mcpStdioSpec, error) {
	if err := e.prepareMCPDataDir(server); err != nil {
		return mcpStdioSpec{}, err
	}
	return e.resolveMCPStdioSpec(ctx, server, target)
}

// resolveMCPStdioSpec resolves the launch identity without touching the filesystem.
func (e *CallExecutor) resolveMCPStdioSpec(
	ctx context.Context,
	server compozyconfig.MCPServer,
	target mcpauth.Target,
) (mcpStdioSpec, error) {
	slot, err := target.Key()
	if err != nil {
		return mcpStdioSpec{}, fmt.Errorf("mcp: derive stdio target identity: %w", err)
	}
	env, secrets, err := e.mcpServerEnv(ctx, server)
	if err != nil {
		return mcpStdioSpec{}, err
	}
	spec := mcpStdioSpec{
		slot:    slot,
		command: strings.TrimSpace(server.Command),
		args:    trimStrings(server.Args),
		env:     env,
		dir:     strings.TrimSpace(server.CWD),
		secrets: secrets,
	}
	spec.digest = spec.identityDigest()
	return spec, nil
}

func (s mcpStdioSpec) identityDigest() string {
	hash := sha256.New()
	write := func(value string) {
		_, _ = fmt.Fprintf(hash, "%d:%s", len(value), value)
	}
	write(s.slot)
	write(s.command)
	write(s.dir)
	for _, arg := range s.args {
		write("arg=" + arg)
	}
	for _, value := range s.env {
		write("env=" + value)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// newMCPStdioProcess prepares the child command and its isolated home; it does not start it.
func newMCPStdioProcess(
	ctx context.Context,
	spec mcpStdioSpec,
	layout mcpStdioLayout,
	stderr io.Writer,
) (mcpStdioLaunch, error) {
	home, err := mcpStdioHome(layout.homesDir, spec.slot)
	if err != nil {
		return mcpStdioLaunch{}, err
	}
	cleanup := func() error {
		return os.RemoveAll(home)
	}
	npmCache := firstNonEmpty(layout.npmCache, filepath.Join(home, "npm-cache"))
	uvCache := firstNonEmpty(layout.uvCache, filepath.Join(home, "uv-cache"))
	env := append(slices.Clone(spec.env),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, "config"),
		"XDG_CACHE_HOME="+filepath.Join(home, "cache"),
		"npm_config_cache="+npmCache,
		"UV_CACHE_DIR="+uvCache,
	)
	command := mcpStdioCommandWithExactEnv(ctx, spec.command, env, spec.args)
	command.Dir = spec.dir
	command.WaitDelay = mcpStdioWaitDelay
	if stderr != nil {
		command.Stderr = stderr
	}
	return mcpStdioLaunch{command: command, cleanup: cleanup}, nil
}

func mcpStdioCommandWithExactEnv(ctx context.Context, command string, env []string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append([]string(nil), env...)
	return cmd
}

func mcpStdioHome(homesDir string, slot string) (string, error) {
	prefix := mcpStdioHomePrefix(slot)
	home, err := os.MkdirTemp(homesDir, prefix)
	if err != nil {
		return "", fmt.Errorf("mcp: create isolated stdio home: %w", err)
	}
	for _, path := range []string{
		filepath.Join(home, "config"),
		filepath.Join(home, "cache"),
		filepath.Join(home, "npm-cache"),
		filepath.Join(home, "uv-cache"),
	} {
		if err := os.Mkdir(path, 0o700); err != nil {
			return "", errors.Join(
				fmt.Errorf("mcp: create isolated stdio directory %q: %w", path, err),
				os.RemoveAll(home),
			)
		}
	}
	return home, nil
}

func mcpStdioHomePrefix(targetKey string) string {
	digest := sha256.Sum256([]byte(targetKey))
	return fmt.Sprintf("%s%x-", mcpStdioHomeNamePrefix, digest[:8])
}

func mcpServerEnv(env map[string]string) []string {
	merged := mcpStdioBaseEnv()
	for key, value := range env {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		merged[trimmedKey] = value
	}
	keys := slices.Sorted(maps.Keys(merged))
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) != "" {
			values = append(values, strings.TrimSpace(key)+"="+merged[key])
		}
	}
	return values
}

func mcpStdioBaseEnv() map[string]string {
	keys := []string{
		"PATH",
		"SystemRoot",
		"WINDIR",
		"COMSPEC",
		"PATHEXT",
		"SSL_CERT_FILE",
		"SSL_CERT_DIR",
	}
	base := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			base[key] = value
		}
	}
	return base
}

// mcpServerEnv resolves the child environment and returns the resolved secret values for redaction.
func (e *CallExecutor) mcpServerEnv(
	ctx context.Context,
	server compozyconfig.MCPServer,
) ([]string, []string, error) {
	env := cloneStringMap(server.Env)
	var secrets []string
	if len(server.SecretEnv) > 0 {
		if env == nil {
			env = make(map[string]string, len(server.SecretEnv))
		}
		for key, ref := range server.SecretEnv {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			value, err := e.resolveSecretRef(ctx, ref)
			if err != nil {
				return nil, nil, fmt.Errorf(
					"mcp: resolve secret_env %s for server %q: %w", trimmedKey, server.Name, err,
				)
			}
			env[trimmedKey] = value
			secrets = append(secrets, value)
		}
	}
	return mcpServerEnv(env), secrets, nil
}
