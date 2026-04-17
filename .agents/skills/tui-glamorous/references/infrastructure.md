# Charm Infrastructure & Development Tools

Self-hosted services, SSH applications, and development utilities.

---

## Table of Contents

- [Wish: SSH App Server](#wish-ssh-app-server)
- [Soft Serve: Git Server](#soft-serve-git-server)
- [Pop: Terminal Email](#pop-terminal-email)
- [Skate: Key-Value Store](#skate-key-value-store)
- [Melt: SSH Key Backup](#melt-ssh-key-backup)
- [Wishlist: SSH Gateway](#wishlist-ssh-gateway)
- [Testing TUIs: teatest](#testing-tuis-teatest)
- [Terminal Detection: x/term](#terminal-detection-xterm)
- [Quick Install](#quick-install)

---

## Wish: SSH App Server

Build SSH-accessible TUI applications.

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/log"
    "github.com/charmbracelet/wish"
    "github.com/charmbracelet/wish/activeterm"
    "github.com/charmbracelet/wish/bubbletea"
    "github.com/charmbracelet/wish/logging"
)

func main() {
    s, err := wish.NewServer(
        wish.WithAddress(":2222"),
        wish.WithHostKeyPath(".ssh/term_info_ed25519"),
        wish.WithMiddleware(
            bubbletea.Middleware(teaHandler),
            activeterm.Middleware(),
            logging.Middleware(),
        ),
    )
    if err != nil {
        log.Fatal("Could not start server", "error", err)
    }

    done := make(chan os.Signal, 1)
    signal.Notify(done, os.Interrupt, syscall.SIGTERM)

    log.Info("Starting SSH server", "addr", s.Addr)
    go func() {
        if err := s.ListenAndServe(); err != nil {
            log.Fatal("Server error", "error", err)
        }
    }()

    <-done
    log.Info("Shutting down...")
    ctx := context.Background()
    s.Shutdown(ctx)
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
    m := NewModel(s.User())
    return m, []tea.ProgramOption{tea.WithAltScreen()}
}
```

**Connect:** `ssh localhost -p 2222`

### Middleware Stack

```go
wish.WithMiddleware(
    bubbletea.Middleware(handler),   // TUI app
    activeterm.Middleware(),          // Terminal detection
    logging.Middleware(),             // Request logging
    // Custom auth:
    func(h ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {
            if !authorized(s) {
                s.Exit(1)
                return
            }
            h(s)
        }
    },
)
```

---

## Soft Serve: Git Server

Self-hosted Git with TUI.

```bash
# Install
brew install soft-serve

# Start
soft serve

# Access
ssh localhost -p 23231
git clone ssh://localhost:23231/repo
```

### Configuration

```yaml
# ~/.config/soft-serve/config.yaml
name: "My Soft Serve"
host: 0.0.0.0
port: 23231
initial_admin_keys:
  - "ssh-ed25519 AAAA... you@example.com"
```

### SSH Commands

```bash
ssh git.example.com                    # Browse repos TUI
ssh git.example.com repo create myrepo
ssh git.example.com repo delete myrepo
ssh git.example.com repo list
ssh git.example.com repo info myrepo
ssh git.example.com user list
ssh git.example.com user add "ssh-ed25519..."
```

---

## Pop: Terminal Email

```bash
# Install
brew install pop

# Send email
pop send \
  --from "me@example.com" \
  --to "you@example.com" \
  --subject "Hello" \
  --body "Message body"

# With attachment
pop send \
  --to "team@example.com" \
  --subject "Report" \
  --attach report.pdf \
  --body "See attached"

# From stdin
cat update.md | pop send \
  --to "team@example.com" \
  --subject "Weekly Update"

# Interactive
pop
```

### Configuration

```yaml
# ~/.config/pop/pop.yml
from: me@example.com
smtp:
  host: smtp.gmail.com
  port: 587
  username: me@example.com
  password_env: SMTP_PASSWORD
```

---

## Skate: Key-Value Store

Simple encrypted storage.

```bash
# Install
brew install skate

# Set/Get
skate set api_key "sk-1234567890"
skate get api_key

# Namespaced keys
skate set config.theme "dark"
skate set config.editor "vim"
skate list config.

# Delete
skate delete api_key

# Sync across machines
skate sync
```

### In Scripts

```bash
API_KEY=$(skate get api_key)
curl -H "Authorization: Bearer $API_KEY" https://api.example.com

THEME=$(skate get config.theme || echo "light")
```

### In Go

```go
import "github.com/charmbracelet/skate"

db, _ := skate.Open("myapp")
defer db.Close()

db.Set("key", []byte("value"))
value, _ := db.Get("key")
db.Delete("key")

// List with prefix
keys, _ := db.List("config.")
```

---

## Melt: SSH Key Backup

```bash
# Install
brew install melt

# Backup (creates encrypted file)
melt backup
# Creates ~/.melt/backup.melt

# Backup to specific file
melt backup -o my-keys.melt

# Restore
melt restore
melt restore -i my-keys.melt
```

**New machine workflow:**
```bash
# On old machine
melt backup -o keys.melt
# Transfer keys.melt securely

# On new machine
brew install melt
melt restore -i keys.melt
```

---

## Wishlist: SSH Gateway

Serve multiple SSH apps on one port.

```yaml
# wishlist.yaml
listen: 0.0.0.0:22
endpoints:
  - name: git
    address: localhost:23231
  - name: chat
    address: localhost:2222
  - name: todos
    address: localhost:2223
```

```bash
wishlist serve
ssh myserver.com  # Shows menu: [git] [chat] [todos]
```

---

## Testing TUIs: teatest

Headless testing for Bubble Tea apps.

```go
import (
    "testing"
    "time"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/x/exp/teatest"
)

func TestApp(t *testing.T) {
    m := NewModel()
    tm := teatest.NewTestModel(t, m)

    // Send keys
    tm.Send(tea.KeyMsg{Type: tea.KeyDown})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    // Type text
    tm.Type("hello world")

    // Wait for condition
    teatest.WaitFor(t, tm, func(bts []byte) bool {
        return strings.Contains(string(bts), "Expected output")
    }, teatest.WithDuration(time.Second))

    // Get final output
    out := tm.FinalOutput(t)
    if !strings.Contains(string(out), "success") {
        t.Fatal("expected success message")
    }

    // Quit
    tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
    tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}
```

### Golden File Testing

```go
func TestGolden(t *testing.T) {
    m := NewModel()
    tm := teatest.NewTestModel(t, m)

    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    out := tm.FinalOutput(t)

    // Compare against saved "golden" output
    teatest.RequireEqualOutput(t, out)
    // First run: creates testdata/TestGolden.golden
    // Subsequent: compares against it
}

// Update golden files: go test -update
```

**Install:**
```bash
go get github.com/charmbracelet/x/exp/teatest@latest
```

---

## Terminal Detection: x/term

Detect terminal capabilities.

```go
import "github.com/charmbracelet/x/term"

// Is this a terminal?
if term.IsTerminal(os.Stdin.Fd()) {
    runTUI()
} else {
    runPlainMode()
}

// Terminal size
width, height, _ := term.GetSize(os.Stdout.Fd())

// Color support
if term.HasDarkBackground() {
    useTheme("dark")
} else {
    useTheme("light")
}
```

### Full Detection Pattern

```go
func main() {
    isTTY := term.IsTerminal(os.Stdin.Fd())
    isPiped := !term.IsTerminal(os.Stdout.Fd())
    noColor := os.Getenv("NO_COLOR") != ""

    switch {
    case !isTTY:
        runFilter()  // stdin is piped
    case isPiped:
        runPlainOutput()  // stdout is piped
    case noColor:
        runNoColor()
    default:
        runTUI()
    }
}
```

**Install:**
```bash
go get github.com/charmbracelet/x/term@latest
```

---

## Quick Install

```bash
# Infrastructure tools
brew install soft-serve pop skate melt

# Go libraries
go get github.com/charmbracelet/wish@latest \
       github.com/charmbracelet/x/exp/teatest@latest \
       github.com/charmbracelet/x/term@latest
```
