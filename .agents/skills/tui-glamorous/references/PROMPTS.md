# THE EXACT PROMPTS for Charm TUIs

Copy-paste prompts for common Charm tasks.

---

## Table of Contents

- [Go TUI Prompts](#go-tui-prompts)
  - [Make My CLI Glamorous](#make-my-cli-glamorous)
  - [Build a TUI Dashboard](#build-a-tui-dashboard)
  - [Add Charm to Existing CLI](#add-charm-to-existing-cli)
- [Shell Script Prompts](#shell-script-prompts)
  - [Interactive Deploy Script](#interactive-deploy-script)
  - [Git Commit Helper](#git-commit-helper)
  - [Menu-Driven Tool](#menu-driven-tool)
- [SSH App Prompts](#ssh-app-prompts)
  - [SSH TUI Service](#ssh-tui-service)
- [Documentation Prompts](#documentation-prompts)
  - [VHS Demo Recording](#vhs-demo-recording)
  - [Beautiful Code Screenshots](#beautiful-code-screenshots)

---

## Go TUI Prompts

### Make My CLI Glamorous

```
I have a Go CLI tool that currently uses fmt.Println and flag parsing.
Transform it into a polished TUI using Charmbracelet libraries:

1. Replace all fmt.Println output with Lip Gloss styled text
2. Replace any user prompts with Huh forms or Bubbles inputs
3. Add a proper help screen using Glamour for markdown rendering
4. Add keyboard navigation with clear visual feedback
5. Handle terminal resize gracefully
6. Add a loading spinner for any async operations
7. Use the alt screen for full-window mode

Preserve all existing functionality while dramatically improving UX.
```

### Build a TUI Dashboard

```
Create a terminal dashboard using Charmbracelet that displays:
- A header with app name and status
- A sidebar with navigation (list component)
- A main content area (viewport for scrolling)
- A footer with keyboard hints (help component)

Requirements:
- Responsive to terminal resize
- Mouse support for clicking items
- Smooth transitions when switching views
- Proper focus management between panes
- Clean exit behavior (restore terminal state)

Use Bubble Tea for state, Bubbles for components, Lip Gloss for layout.
```

### Add Charm to Existing CLI

```
I have an existing CLI using [cobra/urfave/flag]. Add Charm polish:

1. Keep the existing command structure
2. Add interactive mode when run without args
3. Style all output with Lip Gloss
4. Add progress bars for long operations
5. Add confirmation prompts for destructive actions
6. Show errors in styled error boxes
7. Add --no-tui flag to disable for scripting

Show me how to integrate without breaking existing behavior.
```

---

## Shell Script Prompts

### Interactive Deploy Script

```
Create a bash deployment script using Gum that:

1. Shows a styled header/banner
2. Lets user select environment (staging/production) with gum choose
3. For production: requires confirmation with gum confirm
4. Lets user multi-select services to deploy with gum choose --no-limit
5. Shows a spinner during each deployment with gum spin
6. Displays success/failure with styled output

Include proper error handling and early exit on failures.
```

### Git Commit Helper

```
Create a bash script using Gum that helps write conventional commits:

1. Use gum choose for commit type (feat, fix, docs, style, refactor, test, chore)
2. Use gum input for optional scope
3. Use gum input for summary (with character limit)
4. Use gum confirm to ask about adding body
5. If yes, use gum write for multi-line body
6. Show final message in styled box with gum style
7. Confirm and run git commit

Handle empty inputs gracefully and allow user to cancel at any step.
```

### Menu-Driven Tool

```
Create a bash script using Gum that provides a menu-driven interface for:

1. Main menu with gum choose for actions
2. Sub-menus for complex operations
3. File/directory selection with gum file
4. Text viewing with gum pager for long outputs
5. Fuzzy filtering with gum filter for lists
6. Loop back to main menu until user selects "Exit"

Structure it with functions for each menu option.
```

---

## SSH App Prompts

### SSH TUI Service

```
Create an SSH-accessible TUI application using Wish that:

1. Authenticates users via SSH keys
2. Shows a personalized welcome based on ssh username
3. Provides a Bubble Tea TUI with navigation
4. Handles multiple concurrent SSH sessions
5. Logs connections with wish logging middleware
6. Gracefully shuts down on SIGTERM

Include both the server code and a sample TUI model.
Show how to generate and configure host keys.
```

---

## Documentation Prompts

### VHS Demo Recording

```
Create a VHS tape file to record a demo of my CLI tool that:

1. Shows installation command
2. Demonstrates 3-4 key features
3. Uses appropriate pauses for readability
4. Has clean theme and font settings
5. Outputs as optimized GIF for README

Include:
- Hide/Show for setup commands
- Realistic typing speed
- Strategic Sleep commands between actions

Target: 15-30 second final GIF, under 5MB.
```

### Beautiful Code Screenshots

```
Create Freeze configuration and commands to generate beautiful code
screenshots for my project documentation:

1. Consistent theme matching my project branding
2. Line numbers enabled
3. Appropriate padding and shadows
4. Window chrome for that "editor" look

Provide:
- freeze.json config file
- Shell commands for common screenshot tasks
- Batch script for multiple files
```

---

## Advanced Prompts

### Refactor to Bubble Tea

```
I have this Go CLI code that uses a traditional loop with fmt.Scanf prompts.
Refactor it to use the Bubble Tea architecture:

1. Extract all state into a model struct
2. Convert input handling to Update with tea.KeyMsg
3. Convert output to View function with Lip Gloss styling
4. Replace blocking operations with tea.Cmd
5. Add proper initialization with Init()

Maintain all existing functionality while gaining:
- Non-blocking UI updates
- Proper terminal handling
- Resize support
- Clean exit behavior
```

### Component Composition

```
I have multiple Bubble Tea components that need to work together:
- A list for navigation
- A text input for search
- A viewport for content display

Show me how to:
1. Structure the parent model to contain child components
2. Route messages to the correct component based on focus
3. Handle focus switching between components (Tab key)
4. Coordinate state changes between components
5. Compose their Views with Lip Gloss layouts
```

### Multi-Screen App

```
Create a Bubble Tea app with multiple screens:
- Loading screen with spinner
- Main menu screen
- Detail view screen
- Settings screen

Show:
1. Screen state enum
2. Routing in Update based on current screen
3. Per-screen component initialization
4. Transitions between screens
5. Shared header/footer across screens
```
