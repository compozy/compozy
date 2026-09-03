//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pty

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

type shellSetup struct {
	argv    []string
	env     map[string]string
	cleanup func() error
}

func prepareShellIntegration(spec ProcSpec) (shellSetup, error) {
	setup := shellSetup{
		argv: append(
			[]string(nil),
			spec.Argv...),
		env:     cloneEnvironment(spec.Env),
		cleanup: func() error { return nil },
	}
	if !spec.ShellIntegration || strings.TrimSpace(spec.MarkerNonce) == "" || len(spec.Argv) == 0 {
		return setup, nil
	}
	shell := filepath.Base(spec.Argv[0])
	if shell != "bash" && shell != "zsh" && shell != "fish" {
		return setup, nil
	}
	root, err := os.MkdirTemp("", "compozy-terminal-shim-")
	if err != nil {
		return shellSetup{}, fmt.Errorf("terminal pty: create shell integration directory: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return shellSetup{}, errors.Join(
			fmt.Errorf("terminal pty: secure shell integration directory: %w", err), os.RemoveAll(root),
		)
	}
	setup.cleanup = func() error { return os.RemoveAll(root) }
	switch shell {
	case "bash":
		err = prepareBashIntegration(&setup, root, spec.MarkerNonce)
	case "zsh":
		err = prepareZshIntegration(&setup, root, spec.MarkerNonce)
	case "fish":
		err = prepareFishIntegration(&setup, root, spec.MarkerNonce)
	}
	if err != nil {
		return shellSetup{}, errors.Join(err, setup.cleanup())
	}
	return setup, nil
}

func prepareBashIntegration(setup *shellSetup, root, nonce string) error {
	home, err := shellHome(setup.env)
	if err != nil {
		return fmt.Errorf("terminal pty: resolve bash home: %w", err)
	}
	path := filepath.Join(root, "bashrc")
	script := "if [ -f " + shellQuote(filepath.Join(home, ".bashrc")) + " ]; then . " +
		shellQuote(filepath.Join(home, ".bashrc")) + "; fi\n" + bashMarkerScript(nonce)
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		return fmt.Errorf("terminal pty: write bash integration: %w", err)
	}
	setup.argv = append(setup.argv, "--rcfile", path)
	setup.env["ENV"] = path
	return nil
}

func prepareZshIntegration(setup *shellSetup, root, nonce string) error {
	home, err := shellHome(setup.env)
	if err != nil {
		return fmt.Errorf("terminal pty: resolve zsh home: %w", err)
	}
	originalRoot := strings.TrimSpace(setup.env["ZDOTDIR"])
	if originalRoot == "" {
		originalRoot = home
	}
	envScript := zshSourceIfExists(filepath.Join(originalRoot, ".zshenv"))
	if err := os.WriteFile(filepath.Join(root, ".zshenv"), []byte(envScript), 0o600); err != nil {
		return fmt.Errorf("terminal pty: write zsh env integration: %w", err)
	}
	rcScript := zshSourceIfExists(filepath.Join(originalRoot, ".zshrc")) + zshMarkerScript(nonce)
	if err := os.WriteFile(filepath.Join(root, ".zshrc"), []byte(rcScript), 0o600); err != nil {
		return fmt.Errorf("terminal pty: write zsh integration: %w", err)
	}
	setup.env["ZDOTDIR"] = root
	return nil
}

func zshSourceIfExists(path string) string {
	return "if [[ -f " + shellQuote(path) + " ]]; then source " + shellQuote(path) + "; fi\n"
}

// Fish markers ride the vendor_conf.d mechanism through XDG_DATA_DIRS so the
// user's XDG_CONFIG_HOME (themes, conf.d, functions, fish_variables) stays
// exactly as configured.
func prepareFishIntegration(setup *shellSetup, root, nonce string) error {
	vendorRoot := filepath.Join(root, "fish", "vendor_conf.d")
	if err := os.MkdirAll(vendorRoot, 0o700); err != nil {
		return fmt.Errorf("terminal pty: create fish integration directory: %w", err)
	}
	vendorPath := filepath.Join(vendorRoot, "compozy-terminal.fish")
	if err := os.WriteFile(vendorPath, []byte(fishMarkerScript(nonce)), 0o600); err != nil {
		return fmt.Errorf("terminal pty: write fish vendor integration: %w", err)
	}
	setup.env["XDG_DATA_DIRS"] = root + string(os.PathListSeparator) + resolveDataDirs(setup.env)
	return nil
}

func resolveDataDirs(env map[string]string) string {
	if dirs := strings.TrimSpace(env["XDG_DATA_DIRS"]); dirs != "" {
		return dirs
	}
	if dirs := strings.TrimSpace(os.Getenv("XDG_DATA_DIRS")); dirs != "" {
		return dirs
	}
	// XDG Base Directory spec default; setting the variable must not hide it.
	return "/usr/local/share:/usr/share"
}

func bashMarkerScript(nonce string) string {
	return `__compozy_nonce=` + shellQuote(nonce) + `
__compozy_active=0
__compozy_ready=0
__compozy_pct() {
  local value="$1" out="" char hex i
  for ((i=0; i<${#value}; i++)); do
    char="${value:i:1}"
    case "$char" in [a-zA-Z0-9.~_/-]) out+="$char";; *) printf -v hex '%%%02X' "'$char"; out+="$hex";; esac
  done
  printf '%s' "$out"
}
__compozy_preexec() {
  [[ "$__compozy_guard" == 1 || "$__compozy_ready" != 1 ]] && return
  __compozy_guard=1
  __compozy_ready=0
  local command="$BASH_COMMAND"
  printf '\033]7113;v1;%s;S;cmd=%s;cwd=%s\033\\' "$__compozy_nonce" \
    "$(__compozy_pct "$command")" "$(__compozy_pct "$PWD")"
  __compozy_active=1
  __compozy_guard=0
}
__compozy_precmd() {
  local status=$?
  if [[ "$__compozy_active" == 1 ]]; then printf '\033]7113;v1;%s;F;exit=%d\033\\' "$__compozy_nonce" "$status"; fi
  __compozy_active=0
  __compozy_ready=1
}
trap '__compozy_preexec' DEBUG
PROMPT_COMMAND="__compozy_precmd${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
`
}

func zshMarkerScript(nonce string) string {
	return `typeset __compozy_nonce=` + shellQuote(nonce) + `
typeset -gi __compozy_active=0
autoload -Uz add-zsh-hook
__compozy_pct() {
  local value="$1" out="" char hex i
  for ((i=1; i<=${#value}; i++)); do
    char="${value[i]}"
    case "$char" in [a-zA-Z0-9.~_/-]) out+="$char";; *) printf -v hex '%%%02X' "'$char"; out+="$hex";; esac
  done
  printf '%s' "$out"
}
__compozy_preexec() {
  printf '\033]7113;v1;%s;S;cmd=%s;cwd=%s\033\\' "$__compozy_nonce" "$(__compozy_pct "$1")" "$(__compozy_pct "$PWD")"
  __compozy_active=1
}
__compozy_precmd() {
  # zsh's status is a read-only special parameter; shadowing it aborts the hook.
  local __compozy_exit=$?
  if (( __compozy_active )); then printf '\033]7113;v1;%s;F;exit=%d\033\\' "$__compozy_nonce" "$__compozy_exit"; fi
  __compozy_active=0
}
add-zsh-hook preexec __compozy_preexec
add-zsh-hook precmd __compozy_precmd
`
}

func fishMarkerScript(nonce string) string {
	return `set -g __compozy_nonce ` + fishQuote(nonce) + `
function __compozy_preexec --on-event fish_preexec
  printf '\e]7113;v1;%s;S;cmd=%s;cwd=%s\e\\' $__compozy_nonce \
    (string escape --style=url -- $argv) (string escape --style=url -- $PWD)
end
function __compozy_postexec --on-event fish_postexec
  printf '\e]7113;v1;%s;F;exit=%d\e\\' $__compozy_nonce $status
end
`
}

func cloneEnvironment(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source)+3)
	maps.Copy(cloned, source)
	return cloned
}

func shellHome(env map[string]string) (string, error) {
	if home := strings.TrimSpace(env["HOME"]); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func fishQuote(value string) string { return shellQuote(value) }
