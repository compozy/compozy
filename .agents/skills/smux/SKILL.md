---
name: smux
description: Control tmux panes and communicate between AI agents. Use this skill whenever the user mentions tmux panes, cross-pane communication, sending messages to other agents, reading other panes, managing tmux sessions, or interacting with processes running in tmux. Includes tmux-bridge CLI for agent-to-agent messaging and raw tmux commands for direct session control.
metadata:
  { "openclaw": { "emoji": "🖥️", "os": ["darwin", "linux"], "requires": { "bins": ["tmux", "tmux-bridge"] } } }
---

# smux

Tmux pane control and cross-pane agent communication. Use `tmux-bridge` (the high-level CLI) for all cross-pane interactions. Fall back to raw tmux commands only when you need low-level control.

## tmux-bridge — Cross-Pane Communication

A CLI that lets any AI agent interact with any other tmux pane. Works via plain bash. Every command is **atomic**: `type` types text (no Enter), `keys` sends special keys, `read` captures pane content.

### DO NOT WAIT OR POLL

Other panes have agents that will reply to you via tmux-bridge. Their reply appears directly in YOUR pane as a `[tmux-bridge from:...]` message. Do not sleep, poll, read the target pane for a response, or loop. Type your message, press Enter, and move on.

The ONLY time you read a target pane is:
- **Before** interacting with it (enforced by the read guard)
- **After typing** to verify your text landed before pressing Enter
- When interacting with a **non-agent pane** (plain shell, running process)

### Read Guard

The CLI enforces read-before-act. You cannot `type` or `keys` to a pane unless you have read it first.

1. `tmux-bridge read <target>` marks the pane as "read"
2. `tmux-bridge type/keys <target>` checks for that mark — errors if you haven't read
3. After a successful `type`/`keys`, the mark is cleared — you must read again before the next interaction

```
$ tmux-bridge type codex "hello"
error: must read the pane before interacting. Run: tmux-bridge read codex
```

### Command Reference

| Command | Description | Example |
|---|---|---|
| `tmux-bridge list` | Show all panes with target, pid, command, size, label | `tmux-bridge list` |
| `tmux-bridge type <target> <text>` | Type text without pressing Enter | `tmux-bridge type codex "hello"` |
| `tmux-bridge message <target> <text>` | Type text with auto sender info and reply target | `tmux-bridge message codex "review src/auth.ts"` |
| `tmux-bridge read <target> [lines]` | Read last N lines (default 50) | `tmux-bridge read codex 100` |
| `tmux-bridge keys <target> <key>...` | Send special keys | `tmux-bridge keys codex Enter` |
| `tmux-bridge name <target> <label>` | Label a pane (visible in tmux border) | `tmux-bridge name %3 codex` |
| `tmux-bridge resolve <label>` | Print pane target for a label | `tmux-bridge resolve codex` |
| `tmux-bridge id` | Print this pane's ID | `tmux-bridge id` |

### Target Resolution

Targets can be:
- **tmux native**: `session:window.pane` (e.g. `shared:0.1`), pane ID (`%3`), or window index (`0`)
- **label**: Any string set via `tmux-bridge name` — resolved automatically

### Read-Act-Read Cycle

Every interaction follows **read → act → read**. The CLI enforces this.

**Sending a message to an agent:**
```bash
tmux-bridge read codex 20                    # 1. READ — satisfy read guard
tmux-bridge message codex 'Please review src/auth.ts'
                                              # 2. MESSAGE — auto-prepends sender info, no Enter
tmux-bridge read codex 20                    # 3. READ — verify text landed
tmux-bridge keys codex Enter                 # 4. KEYS — submit
# STOP. Do NOT read codex for a reply. The agent replies into YOUR pane.
```

**Approving a prompt (non-agent pane):**
```bash
tmux-bridge read worker 10                   # 1. READ — see the prompt
tmux-bridge type worker "y"                  # 2. TYPE
tmux-bridge read worker 10                   # 3. READ — verify
tmux-bridge keys worker Enter                # 4. KEYS — submit
tmux-bridge read worker 20                   # 5. READ — see the result
```

### Messaging Convention

The `message` command auto-prepends sender info and location:

```
[tmux-bridge from:claude pane:%4 at:3:0.0] Please review src/auth.ts
```

The receiver gets: who sent it (`from`), the exact pane to reply to (`pane`), and the session/window location (`at`). When you see this header, reply using tmux-bridge to the pane ID from the header.

### Agent-to-Agent Workflow

```bash
# 1. Label yourself
tmux-bridge name "$(tmux-bridge id)" claude

# 2. Discover other panes
tmux-bridge list

# 3. Send a message (read-act-read)
tmux-bridge read codex 20
tmux-bridge message codex 'Please review the changes in src/auth.ts'
tmux-bridge read codex 20
tmux-bridge keys codex Enter
```

### Example Conversation

**Agent A (claude) sends:**
```bash
tmux-bridge read codex 20
tmux-bridge message codex 'What is the test coverage for src/auth.ts?'
tmux-bridge read codex 20
tmux-bridge keys codex Enter
```

**Agent B (codex) sees in their prompt:**
```
[tmux-bridge from:claude pane:%4 at:3:0.0] What is the test coverage for src/auth.ts?
```

**Agent B replies using the pane ID from the header:**
```bash
tmux-bridge read %4 20
tmux-bridge message %4 '87% line coverage. Missing the OAuth refresh token path (lines 142-168).'
tmux-bridge read %4 20
tmux-bridge keys %4 Enter
```

---

## Raw tmux Commands

Use these when you need direct tmux control beyond what tmux-bridge provides — session management, window navigation, creating panes, or low-level scripting.

### Capture Output

```bash
tmux capture-pane -t shared -p | tail -20    # Last 20 lines
tmux capture-pane -t shared -p -S -          # Entire scrollback
tmux capture-pane -t shared:0.0 -p           # Specific pane
```

### Send Keys

```bash
tmux send-keys -t shared -l -- "text here"   # Type text (literal mode)
tmux send-keys -t shared Enter               # Press Enter
tmux send-keys -t shared Escape              # Press Escape
tmux send-keys -t shared C-c                 # Ctrl+C
tmux send-keys -t shared C-d                 # Ctrl+D (EOF)
```

For interactive TUIs, split text and Enter into separate sends:
```bash
tmux send-keys -t shared -l -- "Please apply the patch"
sleep 0.1
tmux send-keys -t shared Enter
```

### Panes and Windows

```bash
# Create panes (prefer over new windows)
tmux split-window -h -t SESSION              # Horizontal split
tmux split-window -v -t SESSION              # Vertical split
tmux select-layout -t SESSION tiled          # Re-balance

# Navigate
tmux select-window -t shared:0
tmux select-pane -t shared:0.1
tmux list-windows -t shared
```

### Session Management

```bash
tmux list-sessions
tmux new-session -d -s newsession
tmux kill-session -t sessionname
tmux rename-session -t old new
```

### Claude Code Patterns

```bash
# Check if session needs input
tmux capture-pane -t worker-3 -p | tail -10 | grep -E "❯|Yes.*No|proceed|permission"

# Approve a prompt
tmux send-keys -t worker-3 'y' Enter

# Check all sessions
for s in shared worker-2 worker-3 worker-4; do
  echo "=== $s ==="
  tmux capture-pane -t $s -p 2>/dev/null | tail -5
done
```

## Tips

- **Read guard is enforced** — you MUST read before every `type`/`keys`
- **Every action clears the read mark** — after `type`, read again before `keys`
- **Never wait or poll** — agent panes reply via tmux-bridge into YOUR pane
- **Label panes early** — easier than using `%N` IDs
- **`type` uses literal mode** — special characters are typed as-is
- **`read` defaults to 50 lines** — pass a higher number for more context
- **Non-agent panes** are the exception — you DO need to read them to see output
- Use `capture-pane -p` to print to stdout (essential for scripting)
- Target format: `session:window.pane` (e.g., `shared:0.0`)
