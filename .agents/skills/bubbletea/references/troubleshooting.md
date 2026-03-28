# TUI Troubleshooting Guide

Common issues and their solutions when building Bubbletea applications.

## Layout Issues

### Panels Covering Header/Status Bar

**Symptom:**
Panels overflow and cover the title bar or status bar, especially on portrait/vertical monitors.

**Root Cause:**
Height calculation doesn't account for panel borders.

**Solution:**
Always subtract 2 for borders in height calculations. See [Golden Rules #1](golden-rules.md#rule-1-always-account-for-borders).

```go
// WRONG
contentHeight := totalHeight - titleLines - statusLines

// CORRECT
contentHeight := totalHeight - titleLines - statusLines - 2  // -2 for borders
```

**Quick Fix:**
```go
func (m model) calculateLayout() (int, int) {
    contentHeight := m.height
    if m.config.UI.ShowTitle {
        contentHeight -= 3  // title bar
    }
    if m.config.UI.ShowStatus {
        contentHeight -= 1  // status bar
    }
    contentHeight -= 2  // CRITICAL: borders
    return m.width, contentHeight
}
```

### Panels Misaligned (Different Heights)

**Symptom:**
One panel appears one or more rows higher/lower than adjacent panels.

**Root Cause:**
Text wrapping. Long strings wrap to multiple lines in narrower panels, making them taller.

**Solution:**
Never rely on auto-wrapping. Truncate all text explicitly. See [Golden Rules #2](golden-rules.md#rule-2-never-auto-wrap-in-bordered-panels).

```go
maxTextWidth := panelWidth - 4  // -2 borders, -2 padding

// Truncate everything
title = truncateString(title, maxTextWidth)
subtitle = truncateString(subtitle, maxTextWidth)

for i := range contentLines {
    contentLines[i] = truncateString(contentLines[i], maxTextWidth)
}
```

**Helper function:**
```go
func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    if maxLen < 1 {
        return ""
    }
    return s[:maxLen-1] + "…"
}
```

### Borders Not Rendering

**Symptom:**
Panel borders missing or showing weird characters.

**Possible Causes:**

1. **Terminal doesn't support Unicode box drawing**
   ```go
   // Use ASCII fallback
   border := lipgloss.NormalBorder()  // Uses +-| instead of ┌─┐
   ```

2. **Terminal encoding issue**
   ```bash
   export LANG=en_US.UTF-8
   export LC_ALL=en_US.UTF-8
   ```

3. **Wrong border style**
   ```go
   // Make sure you're using a valid border
   import "github.com/charmbracelet/lipgloss"

   border := lipgloss.RoundedBorder()  // ╭─╮
   // or
   border := lipgloss.NormalBorder()   // ┌─┐
   // or
   border := lipgloss.DoubleBorder()   // ╔═╗
   ```

### Content Overflows Panel

**Symptom:**
Text or content extends beyond panel boundaries.

**Solutions:**

1. **For text content:**
   ```go
   // Truncate to fit
   maxWidth := panelWidth - 4
   content = truncateString(content, maxWidth)
   ```

2. **For multi-line content:**
   ```go
   // Limit both width and height
   maxWidth := panelWidth - 4
   maxHeight := panelHeight - 2

   lines := strings.Split(content, "\n")
   for i := 0; i < maxHeight && i < len(lines); i++ {
       displayLines = append(displayLines,
           truncateString(lines[i], maxWidth))
   }
   ```

3. **For wrapped content:**
   ```go
   // Use lipgloss MaxWidth
   content := lipgloss.NewStyle().
       MaxWidth(panelWidth - 4).
       Render(text)
   ```

## Mouse Issues

### Mouse Clicks Not Working

**Symptom:**
Clicking panels doesn't change focus or trigger actions.

**Possible Causes:**

1. **Mouse not enabled in program**
   ```go
   // In main()
   p := tea.NewProgram(
       initialModel(),
       tea.WithAltScreen(),
       tea.WithMouseCellMotion(),  // Enable mouse
   )
   ```

2. **Not handling MouseMsg**
   ```go
   func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
       switch msg := msg.(type) {
       case tea.MouseMsg:
           return m.handleMouse(msg)
       }
   }
   ```

3. **Wrong coordinate system**
   See [Mouse Detection Not Matching Layout](#mouse-detection-not-matching-layout).

### Mouse Detection Not Matching Layout

**Symptom:**
Clicks work in horizontal layout but break when terminal is resized to vertical stack (or vice versa).

**Root Cause:**
Using X coordinates when layout is vertical, or Y coordinates when horizontal.

**Solution:**
Check layout mode before processing mouse events. See [Golden Rules #3](golden-rules.md#rule-3-match-mouse-detection-to-layout).

```go
func (m model) handleLeftClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
    if m.shouldUseVerticalStack() {
        // Vertical: use Y coordinates
        if msg.Y < topPanelHeight {
            m.focusedPanel = "top"
        } else {
            m.focusedPanel = "bottom"
        }
    } else {
        // Horizontal: use X coordinates
        if msg.X < leftPanelWidth {
            m.focusedPanel = "left"
        } else {
            m.focusedPanel = "right"
        }
    }
    return m, nil
}
```

### Mouse Scrolling Not Working

**Symptom:**
Mouse wheel doesn't scroll content.

**Solution:**
```go
case tea.MouseMsg:
    switch msg.Type {
    case tea.MouseWheelUp:
        m.scroll -= 3
        if m.scroll < 0 {
            m.scroll = 0
        }
    case tea.MouseWheelDown:
        m.scroll += 3
        maxScroll := len(m.content) - m.visibleLines
        if m.scroll > maxScroll {
            m.scroll = maxScroll
        }
    }
```

## Rendering Issues

### Flickering/Jittering

**Symptom:**
Screen flickers or elements jump around during updates.

**Causes & Solutions:**

1. **Updating too frequently**
   ```go
   // Don't update on every tick
   case tickMsg:
       if m.needsUpdate {
           m.needsUpdate = false
           return m, nil
       }
       return m, tick()  // Skip render
   ```

2. **Inconsistent dimensions**
   ```go
   // Cache dimensions, don't recalculate every frame
   type model struct {
       width, height int
       cachedLayout  string
       layoutDirty   bool
   }

   func (m model) View() string {
       if m.layoutDirty {
           m.cachedLayout = m.renderLayout()
           m.layoutDirty = false
       }
       return m.cachedLayout
   }
   ```

3. **Using alt screen incorrectly**
   ```go
   // Always use alt screen for full-screen TUIs
   p := tea.NewProgram(
       initialModel(),
       tea.WithAltScreen(),  // Essential!
   )
   ```

### Colors Not Showing

**Symptom:**
Colors appear as plain text or wrong colors.

**Possible Causes:**

1. **Terminal doesn't support colors**
   ```bash
   # Check color support
   echo $COLORTERM  # Should show "truecolor" or "24bit"
   tput colors      # Should show 256 or more
   ```

2. **Not using lipgloss properly**
   ```go
   // Use lipgloss for color
   import "github.com/charmbracelet/lipgloss"

   style := lipgloss.NewStyle().
       Foreground(lipgloss.Color("#FF0000")).
       Background(lipgloss.Color("#000000"))
   ```

3. **Environment variables**
   ```bash
   export TERM=xterm-256color
   export COLORTERM=truecolor
   ```

### Emojis/Unicode Wrong Width

**Symptom:**
Emojis cause text misalignment, borders broken, columns don't line up.

**Root Cause:**
Different terminals calculate emoji width differently (1 vs 2 cells).

**Solutions:**

1. **Detect and adjust**
   ```go
   import "github.com/mattn/go-runewidth"

   // Get actual display width
   width := runewidth.StringWidth(text)
   ```

2. **Avoid emojis in structural elements**
   ```go
   // DON'T use emojis in borders, tables, or aligned content
   // DO use emojis in content that doesn't need precise alignment
   ```

3. **Use icons from fixed-width sets**
   ```go
   // Use Nerd Fonts or similar fixed-width icon fonts instead
   // 󰈙 (vs 📁 emoji)
   ```

4. **Terminal-specific settings**
   For WezTerm, see project's `docs/EMOJI_WIDTH_FIX.md`.

## Keyboard Issues

### Keyboard Shortcuts Not Working

**Symptom:**
Key presses don't trigger expected actions.

**Debugging Steps:**

1. **Log the key events**
   ```go
   case tea.KeyMsg:
       log.Printf("Key: %s, Type: %s", msg.String(), msg.Type)
   ```

2. **Check key matching**
   ```go
   import "github.com/charmbracelet/bubbles/key"

   type keyMap struct {
       Quit key.Binding
   }

   var keys = keyMap{
       Quit: key.NewBinding(
           key.WithKeys("q", "ctrl+c"),
           key.WithHelp("q", "quit"),
       ),
   }

   // In Update
   case tea.KeyMsg:
       if key.Matches(msg, keys.Quit) {
           return m, tea.Quit
       }
   ```

3. **Check focus state**
   ```go
   // Make sure the right component has focus
   case tea.KeyMsg:
       switch m.focused {
       case "input":
           // Route to input
       case "list":
           // Route to list
       }
   ```

### Special Keys Not Detected

**Symptom:**
Function keys, Ctrl combinations, or other special keys don't work.

**Solution:**
Use tea.KeyType constants:

```go
case tea.KeyMsg:
    switch msg.Type {
    case tea.KeyCtrlC:
        return m, tea.Quit
    case tea.KeyTab:
        m.nextPanel()
    case tea.KeyF1:
        m.showHelp()
    case tea.KeyEnter:
        m.confirm()
    }
```

Common keys:
- `tea.KeyTab`
- `tea.KeyEnter`
- `tea.KeyEsc`
- `tea.KeyCtrlC`
- `tea.KeyUp/Down/Left/Right`
- `tea.KeyF1` through `tea.KeyF12`

## Performance Issues

### Slow Rendering

**Symptom:**
Noticeable lag when updating the display.

**Solutions:**

1. **Only render visible content**
   ```go
   // Don't render 1000 lines when only 20 are visible
   visibleStart := m.scroll
   visibleEnd := min(m.scroll + m.height, len(m.lines))

   for i := visibleStart; i < visibleEnd; i++ {
       rendered = append(rendered, m.lines[i])
   }
   ```

2. **Cache expensive computations**
   ```go
   type model struct {
       content       []string
       renderedCache string
       contentDirty  bool
   }

   func (m *model) View() string {
       if m.contentDirty {
           m.renderedCache = m.renderContent()
           m.contentDirty = false
       }
       return m.renderedCache
   }
   ```

3. **Avoid string concatenation in loops**
   ```go
   // SLOW
   var s string
   for _, line := range lines {
       s += line + "\n"  // Creates new string each iteration
   }

   // FAST
   var b strings.Builder
   for _, line := range lines {
       b.WriteString(line)
       b.WriteString("\n")
   }
   s := b.String()
   ```

4. **Lazy load data**
   ```go
   // Don't load all files upfront
   type model struct {
       fileList    []string
       fileContent map[string]string  // Load on demand
   }

   func (m *model) getFileContent(path string) string {
       if content, ok := m.fileContent[path]; ok {
           return content
       }
       content := loadFile(path)
       m.fileContent[path] = content
       return content
   }
   ```

### High Memory Usage

**Symptom:**
Application uses excessive memory.

**Solutions:**

1. **Limit cache size**
   ```go
   const maxCacheEntries = 100

   func (m *model) addToCache(key, value string) {
       if len(m.cache) >= maxCacheEntries {
           // Evict oldest entry
           for k := range m.cache {
               delete(m.cache, k)
               break
           }
       }
       m.cache[key] = value
   }
   ```

2. **Stream large files**
   ```go
   // Don't load entire file into memory
   func readLines(path string, start, count int) ([]string, error) {
       f, err := os.Open(path)
       if err != nil {
           return nil, err
       }
       defer f.Close()

       scanner := bufio.NewScanner(f)
       var lines []string
       lineNum := 0

       for scanner.Scan() {
           if lineNum >= start && lineNum < start+count {
               lines = append(lines, scanner.Text())
           }
           lineNum++
           if lineNum >= start+count {
               break
           }
       }

       return lines, scanner.Err()
   }
   ```

## Configuration Issues

### Config File Not Loading

**Symptom:**
Application doesn't respect config file settings.

**Common Locations:**
```go
configPaths := []string{
    "./config.yaml",                           // Current directory
    "~/.config/yourapp/config.yaml",          // XDG config
    "/etc/yourapp/config.yaml",               // System-wide
}

for _, path := range configPaths {
    if fileExists(expandPath(path)) {
        return loadConfig(path)
    }
}
```

**Debug config loading:**
```go
func loadConfig(path string) (*Config, error) {
    log.Printf("Attempting to load config from: %s", path)

    data, err := os.ReadFile(path)
    if err != nil {
        log.Printf("Failed to read config: %v", err)
        return nil, err
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        log.Printf("Failed to parse config: %v", err)
        return nil, err
    }

    log.Printf("Successfully loaded config: %+v", cfg)
    return &cfg, nil
}
```

## Debugging Decision Tree

```
Problem?
│
├─ Layout issue?
│  ├─ Panels covering title/status? → Check border accounting (Rule #1)
│  ├─ Panels misaligned? → Check text wrapping (Rule #2)
│  ├─ Borders missing? → Check terminal Unicode support
│  └─ Content overflow? → Check truncation
│
├─ Mouse issue?
│  ├─ Clicks not working? → Check mouse enabled + MouseMsg handling
│  ├─ Wrong panel focused? → Check layout orientation (Rule #3)
│  └─ Scrolling broken? → Check MouseWheel handling
│
├─ Rendering issue?
│  ├─ Flickering? → Check update frequency + alt screen
│  ├─ No colors? → Check terminal support + TERM variable
│  └─ Emoji alignment? → Check terminal emoji width settings
│
├─ Keyboard issue?
│  ├─ Shortcuts not working? → Log KeyMsg, check key.Matches
│  ├─ Special keys broken? → Use tea.KeyType constants
│  └─ Wrong component responding? → Check focus state
│
└─ Performance issue?
   ├─ Slow rendering? → Cache, virtual scrolling, visible-only
   └─ High memory? → Limit cache, stream data

```

## General Debugging Tips

### 1. Enable Debug Logging

```go
// Create debug log file
func setupDebugLog() *os.File {
    f, err := os.OpenFile("debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return nil
    }
    log.SetOutput(f)
    return f
}

// In main()
logFile := setupDebugLog()
if logFile != nil {
    defer logFile.Close()
}
```

### 2. Log All Messages

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    log.Printf("Update: %T %+v", msg, msg)
    // ... rest of update logic
}
```

### 3. Inspect Terminal Capabilities

```bash
# Check terminal type
echo $TERM

# Check color support
tput colors

# Check dimensions
tput cols
tput lines

# Check capabilities
infocmp $TERM
```

### 4. Test in Different Terminals

Try your app in multiple terminals:
- iTerm2 (macOS)
- Alacritty (cross-platform)
- kitty (cross-platform)
- WezTerm (cross-platform)
- Windows Terminal (Windows)
- Termux (Android)

### 5. Use Alt Screen

Always use alt screen for full-screen TUIs:

```go
p := tea.NewProgram(
    initialModel(),
    tea.WithAltScreen(),  // Essential!
    tea.WithMouseCellMotion(),
)
```

This prevents messing up the user's terminal when your app exits.

## Getting Help

If you're still stuck:

1. Check the [Golden Rules](golden-rules.md) - 90% of issues are layout-related
2. Review the [Components Guide](components.md) for proper component usage
3. Check Bubbletea examples: https://github.com/charmbracelet/bubbletea/tree/master/examples
4. Ask in Charm Discord: https://charm.sh/discord
5. Search Bubbletea issues: https://github.com/charmbracelet/bubbletea/issues
