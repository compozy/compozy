# Zsh startup fidelity — issue 626

The Unix PTY `TestShellIntegrationContract` suite reproduces the missing plugin bundle using a
real zsh and `${ZDOTDIR:-$HOME}/.zsh_plugins.txt`. Before the fix, the integrated shell resolves
that bundle inside the temporary shim directory; `.zshenv` redirection also skips the marker rc.
The reproduction log is retained in the issue executor's local evidence packet.

The corrected startup restores the original environment before user configuration executes,
retains temporary routing between interactive startup stages, and follows subsequent ZDOTDIR
changes. `typeset -p` preserves unset/set state and export attributes between those stages.
User files are not rewritten. The native shell continues to own `.zlogin` and `.zlogout`.

## Changed-slice replay

- Command: `CGO_ENABLED=1 go test -race ./internal/terminal/pty -run TestShellIntegrationContract -count=1 -timeout=60s`.
- Local platform: macOS arm64, zsh 5.9. All cases pass against real OS PTYs.
- Twelve native/integrated comparisons: interactive and interactive-login shells, with unset
  ZDOTDIR, custom directories containing spaces/quotes, `.zshenv` redirection, `.zshenv` unsetting,
  `.zprofile` redirection, and `.zshrc` redirection.
- Additional cases compare inherited daemon ZDOTDIR and noninteractive login startup, including an
  explicitly empty ZDOTDIR.
- Exact startup traces match native zsh, including startup phase order and ZDOTDIR attributes.
  Parent and nested interactive shells both load their plugin bundle. The child does not load
  the parent's nonce, and the integrated parent emits authenticated start/finish markers.
- Existing bash marker, disabled-integration, private-shim cleanup and fish configuration
  assertions also pass in the canonical suite.
- Each case owns temporary fixture homes and its PTYs; suite cleanup waits/kills only those
  processes and closes them. No daemon or persistent QA lab is started.

## Limits

The changed startup slice is verified at the real PTY boundary. This does not claim a new
rendered Web, packaged-desktop, or Linux run. Linux validation belongs to the PR's current-head
checks; a skipped zsh case does not establish Linux zsh coverage. Global system startup files
continue to run under zsh's native startup mechanism; this replay exercises user startup files.

## Review follow-up: children between startup bridges

CodeRabbit identified an additional interval: a global startup file can launch a child after the
user env/profile file returns and before the next user-file bridge restores ZDOTDIR. The bridge
now carries only the user's directory value and export metadata for that interval. A child entering
the shim restores the environment it would normally inherit, removes the temporary metadata, and
sources its own `.zshenv` without continuing the parent's marker injection. A nonexported parent
ZDOTDIR remains absent in the child. The parent still retains routing to its injected rc.

The existing canonical suite now explicitly launches children in both bridge intervals with unset
and custom ZDOTDIR. It compares native/integrated child output and verifies the child has no parent
nonce. This follow-up is validated exclusively by GitHub CI under the later user instruction;
the earlier local results above apply to the initial implementation only.

The initial Linux CI logs exposed missing zsh, so the Go race-test workflow now installs zsh
before executing its existing sharded suite. Linux evidence must include the real shell cases in
the gotestsum JSON artifact; package success with skipped shell cases is insufficient.


Greptile's inherited-state follow-up binds the child restoration metadata to the current private
shim directory and clears stale metadata before top-level user startup. The existing inherited
daemon-environment case now includes stale bridge variables and still requires authenticated
parent markers and native startup trace equality. This prevents an unrelated inherited variable
from silently selecting the marker-free child path.
