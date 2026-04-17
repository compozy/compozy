# Shell Scripts with Charm

Beautiful terminal UI for bash/shell scripts without writing Go.

---

## Table of Contents

- [Gum: The Essential Tool](#gum-the-essential-tool)
  - [Input & Text](#input--text)
  - [Selection](#selection)
  - [Confirmation](#confirmation)
  - [Spinners & Progress](#spinners--progress)
  - [Styled Output](#styled-output)
  - [Join & Layout](#join--layout)
  - [Format & Markdown](#format--markdown)
  - [File Operations](#file-operations)
- [Complete Gum Recipes](#complete-gum-recipes)
- [VHS: Terminal Recording](#vhs-terminal-recording)
- [Mods: AI in Terminal](#mods-ai-in-terminal)
- [Glow: Markdown Viewer](#glow-markdown-viewer)
- [Freeze: Code Screenshots](#freeze-code-screenshots)
- [Quick Install](#quick-install)

---

## Gum: The Essential Tool

Gum provides all the UI primitives you need for shell scripts.

```bash
# Install
brew install gum
# or: go install github.com/charmbracelet/gum@latest
```

### Input & Text

```bash
# Single line input
NAME=$(gum input --placeholder "Your name")
NAME=$(gum input --value "default" --prompt "> ")
NAME=$(gum input --password --placeholder "Password")  # Hidden input
NAME=$(gum input --char-limit 50)  # Limit length

# Multi-line input (textarea)
DESCRIPTION=$(gum write --placeholder "Enter description...")
DESCRIPTION=$(gum write --width 80 --height 10)
COMMIT_MSG=$(gum write --header "Commit Message")
```

### Selection

```bash
# Single choice
COLOR=$(gum choose "red" "green" "blue")
COLOR=$(gum choose --header "Pick a color:" "red" "green" "blue")
ITEM=$(gum choose --limit 1 "one" "two" "three")  # Explicit single

# Multi-select
TOPPINGS=$(gum choose --no-limit "cheese" "pepperoni" "mushrooms" "olives")
SELECTED=$(gum choose --limit 3 "a" "b" "c" "d" "e")  # Max 3 selections

# From stdin (pipe anything!)
BRANCH=$(git branch | gum choose)
FILE=$(ls | gum choose)
PROCESS=$(ps aux | gum choose | awk '{print $2}')

# Fuzzy filter (searchable)
BRANCH=$(git branch | gum filter)
BRANCH=$(gum filter --placeholder "Search branches..." < <(git branch))
FILE=$(find . -name "*.go" | gum filter --height 20)
```

### Confirmation

```bash
# Basic confirm (returns exit code)
gum confirm "Delete all files?" && rm -rf ./tmp

# With custom labels
gum confirm "Deploy to production?" \
  --affirmative "Yes, deploy" \
  --negative "Cancel" && ./deploy.sh

# In conditionals
if gum confirm "Continue?"; then
    echo "Proceeding..."
else
    echo "Cancelled"
    exit 1
fi
```

### Spinners & Progress

```bash
# Spinner while command runs
gum spin --title "Installing..." -- npm install
gum spin --spinner dot --title "Building..." -- make build
gum spin --spinner line --title "Fetching..." -- curl -s https://api.example.com

# Available spinners: line, dot, minidot, jump, pulse, points, globe, moon, monkey, meter, hamburger

# Show spinner with custom command
gum spin --title "Processing..." -- sleep 5

# Capture output while showing spinner
OUTPUT=$(gum spin --show-output --title "Running..." -- ./my-script.sh)
```

### Styled Output

```bash
# Basic styling
gum style "Hello World"
gum style --foreground 212 "Pink text"
gum style --foreground "#ff0000" "Red text"
gum style --background 235 --foreground 255 "Styled box"

# Borders
gum style --border normal "Normal border"
gum style --border rounded "Rounded border"
gum style --border double "Double border"
gum style --border thick "Thick border"
gum style --border hidden "Hidden border (padding only)"

# Padding and margins
gum style --padding "1 2" "Padded text"  # vertical horizontal
gum style --margin "1 2 1 2" "With margin"  # top right bottom left

# Alignment
gum style --width 40 --align center "Centered"
gum style --width 40 --align right "Right aligned"

# Bold, italic, etc.
gum style --bold "Bold text"
gum style --italic "Italic text"
gum style --strikethrough "Strikethrough"
gum style --underline "Underlined"

# Combine everything
gum style \
  --border rounded \
  --border-foreground 212 \
  --padding "1 2" \
  --foreground 212 \
  --bold \
  "Beautiful Box"
```

### Join & Layout

```bash
# Horizontal join
gum join --horizontal "Left" "Middle" "Right"
gum join --horizontal --align center "A" "B" "C"

# Vertical join
gum join --vertical "Line 1" "Line 2" "Line 3"

# Combined layouts
HEADER=$(gum style --bold "Header")
BODY=$(gum style --border rounded "Content")
FOOTER=$(gum style --faint "Footer")
gum join --vertical "$HEADER" "$BODY" "$FOOTER"
```

### Format & Markdown

```bash
# Format template strings
gum format "Hello, **world**!"
gum format "Code: \`inline\`"
gum format -- "# Heading" "Paragraph text" "- List item"

# From file
gum format < template.md

# With emoji
gum format ":rocket: Deploying..."
gum format ":white_check_mark: Done"
```

### File Operations

```bash
# File picker
FILE=$(gum file .)
FILE=$(gum file --directory)  # Directories only
FILE=$(gum file --all)  # Include hidden files
FILE=$(gum file --height 20 /path/to/start)

# Pager (for long output)
cat long-file.txt | gum pager
gum pager < README.md
git diff | gum pager --show-line-numbers
```

---

## Complete Gum Recipes

### Git Commit Script

```bash
#!/bin/bash
# Conventional commit helper

TYPE=$(gum choose "feat" "fix" "docs" "style" "refactor" "test" "chore")
SCOPE=$(gum input --placeholder "scope (optional)")
SUMMARY=$(gum input --placeholder "summary" --char-limit 50)

# Build scope part
if [ -n "$SCOPE" ]; then
    SCOPE="($SCOPE)"
fi

# Get optional body
gum confirm "Add body?" && BODY=$(gum write --placeholder "Details...")

# Build message
MSG="$TYPE$SCOPE: $SUMMARY"
if [ -n "$BODY" ]; then
    MSG="$MSG

$BODY"
fi

# Confirm and commit
echo
gum style --border rounded --padding "1 2" "$MSG"
echo
gum confirm "Commit with this message?" && git commit -m "$MSG"
```

### Interactive Deploy Script

```bash
#!/bin/bash

gum style --border double --padding "1 2" --foreground 212 "🚀 Deployment Tool"
echo

# Select environment
ENV=$(gum choose --header "Select environment:" "staging" "production")

# Production confirmation
if [ "$ENV" = "production" ]; then
    gum style --foreground 196 "⚠️  WARNING: Production deployment!"
    gum confirm "Are you sure?" || exit 1
    gum input --password --placeholder "Enter deploy password" | grep -q "secret" || {
        gum style --foreground 196 "Wrong password"
        exit 1
    }
fi

# Select services
SERVICES=$(gum choose --no-limit --header "Select services:" \
    "api" "web" "worker" "scheduler")

# Run deployment
for SERVICE in $SERVICES; do
    gum spin --spinner dot --title "Deploying $SERVICE to $ENV..." -- \
        ./deploy.sh "$ENV" "$SERVICE"
done

gum style --foreground 82 "✓ Deployment complete!"
```

### File Browser & Editor

```bash
#!/bin/bash

while true; do
    FILE=$(gum file --height 20 .)

    if [ -z "$FILE" ]; then
        break
    fi

    ACTION=$(gum choose "View" "Edit" "Delete" "Back")

    case $ACTION in
        "View")
            cat "$FILE" | gum pager
            ;;
        "Edit")
            ${EDITOR:-vim} "$FILE"
            ;;
        "Delete")
            gum confirm "Delete $FILE?" && rm "$FILE"
            ;;
        "Back")
            continue
            ;;
    esac
done
```

### Database Query Tool

```bash
#!/bin/bash

DB=$(gum choose "production" "staging" "development")
QUERY=$(gum write --header "Enter SQL query" --placeholder "SELECT * FROM...")

gum style --faint "Running on $DB..."
gum spin --title "Executing query..." -- \
    psql -h "$DB.example.com" -c "$QUERY" | gum pager
```

---

## VHS: Terminal Recording

Record terminal sessions as GIFs for documentation.

```bash
# Install
brew install vhs
```

### Basic Tape File

```tape
# demo.tape
Output demo.gif

Set FontSize 14
Set Width 1200
Set Height 600
Set Theme "Catppuccin Mocha"

Type "echo 'Hello, World!'"
Sleep 500ms
Enter
Sleep 1s

Type "ls -la"
Enter
Sleep 2s
```

```bash
# Record
vhs demo.tape
```

### Complete Command Reference

```tape
# === OUTPUT ===
Output demo.gif           # GIF (default)
Output demo.mp4           # Video
Output demo.webm          # WebM
Output frames/            # PNG frames

# === SETTINGS ===
Set Shell "bash"          # Shell to use
Set FontSize 16           # Font size
Set FontFamily "JetBrains Mono"
Set Width 1200            # Terminal width
Set Height 600            # Terminal height
Set Padding 20            # Padding around terminal
Set Theme "Dracula"       # Color theme
Set TypingSpeed 50ms      # Delay between keystrokes
Set Framerate 60          # FPS for recordings
Set CursorBlink false     # Disable cursor blink
Set WindowBar Colorful    # Window decoration style
Set WindowBarSize 40      # Window bar height
Set LoopOffset 0          # GIF loop start offset

# === TYPING ===
Type "echo hello"         # Type text
Type@100ms "slow typing"  # Custom typing speed
Type "fast" # comment     # Comments after commands

# === KEYS ===
Enter                     # Press enter
Space                     # Press space
Tab                       # Tab key
Backspace                 # Backspace once
Backspace 5               # Backspace 5 times
Delete                    # Delete key
Up                        # Arrow up
Down                      # Arrow down
Left                      # Arrow left
Right                     # Arrow right
Ctrl+C                    # Key combination
Ctrl+A                    # Select all
Escape                    # Escape key

# === TIMING ===
Sleep 1s                  # Wait 1 second
Sleep 500ms               # Wait 500 milliseconds
Sleep 2.5s                # Wait 2.5 seconds

# === SCREEN ===
Hide                      # Hide commands from output
Show                      # Show commands again
Screenshot demo.png       # Take screenshot
```

### Popular Themes

```tape
Set Theme "Dracula"
Set Theme "Catppuccin Mocha"
Set Theme "GitHub Dark"
Set Theme "One Dark"
Set Theme "Tokyo Night"
Set Theme "Nord"
Set Theme "Solarized Dark"
```

### Example: CLI Tool Demo

```tape
Output tool-demo.gif

Set FontSize 16
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms

# Show installation
Type "brew install mytool"
Enter
Sleep 1s
Hide
Type "echo 'Installed!'"
Enter
Show
Sleep 500ms

# Demo usage
Type "mytool --help"
Enter
Sleep 2s

Type "mytool create project"
Enter
Sleep 1s

Type "cd project && mytool run"
Enter
Sleep 3s

# Clean ending
Type "exit"
Enter
```

---

## Mods: AI in Terminal

Pipe anything to AI.

```bash
# Install
brew install mods

# Or with Go
go install github.com/charmbracelet/mods@latest
```

### Basic Usage

```bash
# Ask questions
mods "What is the capital of France?"

# Pipe content
echo "Explain this" | mods
cat error.log | mods "what's wrong?"

# Files
mods "summarize this" < README.md
cat *.go | mods "find bugs"
```

### Code Operations

```bash
# Code review
git diff | mods "review for bugs and style issues"

# Generate code
mods "write a bash function to backup dotfiles" > backup.sh

# Explain code
cat complex-function.py | mods "explain this code"

# Refactor suggestions
cat old-code.js | mods "modernize this JavaScript"
```

### Git Integration

```bash
# Generate commit message
git diff --staged | mods "write a conventional commit message"

# PR description
git log main..HEAD --oneline | mods "write PR description"

# Changelog
git log --oneline v1.0..v2.0 | mods "write changelog entry"
```

### Configuration

```yaml
# ~/.config/mods/mods.yml
default-model: gpt-4
apis:
  openai:
    api-key-env: OPENAI_API_KEY
  anthropic:
    api-key-env: ANTHROPIC_API_KEY

# Model aliases
aliases:
  fast: gpt-3.5-turbo
  smart: gpt-4
  creative: claude-3-opus
```

```bash
# Use specific model
mods --model gpt-4 "complex question"
mods -m claude-3-opus "creative task"

# Conversation continuation
mods "initial question"
mods --continue "follow-up"
mods -c "another follow-up"

# Format output
mods --format "explain X" | glow  # Pipe markdown to glow
mods -f "write docs" > DOCS.md
```

### Power Recipes

```bash
# Auto-fix linting errors
eslint . 2>&1 | mods "fix these errors" > fixes.patch

# Explain error and suggest fix
./broken-script.sh 2>&1 | mods "explain this error and suggest fix"

# Generate tests
cat src/utils.js | mods "write Jest tests" > src/utils.test.js

# Documentation
cat *.go | mods "write godoc comments" > docs.go

# SQL from natural language
mods "SQL to get users who signed up last week"
```

---

## Glow: Markdown Viewer

Beautiful markdown in terminal.

```bash
# Install
brew install glow

# View file
glow README.md

# With pager (scrollable)
glow -p README.md

# From URL
glow https://raw.githubusercontent.com/user/repo/main/README.md

# From stdin
cat file.md | glow -
echo "# Hello" | glow -

# Styles
glow -s dark README.md    # Dark theme
glow -s light README.md   # Light theme
glow -s auto README.md    # Auto-detect
glow -s notty README.md   # No styling (for piping)

# Width
glow -w 80 README.md      # Wrap at 80 chars
```

### Stashing (Offline Reading)

```bash
# Save for later
glow stash README.md
glow stash https://example.com/article.md

# List stashed
glow stash list

# Read stashed
glow stash show 1

# Search stashed
glow stash list | grep "keyword"
```

---

## Freeze: Code Screenshots

Beautiful code images for documentation.

```bash
# Install
brew install freeze
```

### Basic Usage

```bash
# From file
freeze main.go -o code.png

# From stdin
cat snippet.py | freeze --language python -o snippet.png

# Specific lines
freeze main.go --lines 10,20 -o function.png
freeze main.go --lines 10-30 -o block.png
```

### Styling Options

```bash
freeze main.go \
  --theme "catppuccin-mocha" \
  --font "JetBrains Mono" \
  --font-size 14 \
  --line-height 1.4 \
  --shadow \
  --padding 20 \
  --margin 20 \
  --line-numbers \
  --window \
  --border-radius 8 \
  -o beautiful-code.png
```

### Configuration File

```json
// freeze.json
{
  "theme": "catppuccin-mocha",
  "font": {
    "family": "JetBrains Mono",
    "size": 14
  },
  "shadow": {
    "blur": 20,
    "x": 0,
    "y": 10
  },
  "padding": [20, 40, 20, 20],
  "margin": [0, 0, 0, 0],
  "line_numbers": true,
  "window": true,
  "border": {
    "radius": 8,
    "width": 1,
    "color": "#444"
  }
}
```

```bash
freeze --config freeze.json main.go -o code.png
```

### Available Themes

```
catppuccin-mocha, catppuccin-latte, dracula, github-dark, github-light,
monokai, nord, one-dark, solarized-dark, solarized-light, tokyo-night
```

---

## Quick Install

```bash
# All shell tools at once
brew install gum glow vhs freeze mods

# Or from Charm tap
brew tap charmbracelet/tap
brew install charmbracelet/tap/gum \
             charmbracelet/tap/glow \
             charmbracelet/tap/vhs \
             charmbracelet/tap/freeze \
             charmbracelet/tap/mods
```
