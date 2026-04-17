# Web UI QA Reference

Read this reference when the project has a Web UI surface and browser flows are in scope.

## agent-browser Core Loop

Every browser interaction follows the **Open -> Snapshot -> Interact -> Re-snapshot -> Verify** cycle.

### Navigation

```bash
agent-browser open <url>            # Navigate to page
agent-browser back                  # Go back
agent-browser forward               # Go forward
agent-browser reload                # Reload page
agent-browser close                 # Close browser
```

### Snapshot and Interaction

```bash
agent-browser snapshot -i           # Get interactive elements with refs
agent-browser click @e1             # Click element by ref
agent-browser fill @e2 "text"       # Clear and type into input
agent-browser select @e3 "value"    # Select dropdown option
agent-browser press Enter           # Press key
agent-browser check @e1             # Check checkbox
agent-browser uncheck @e1           # Uncheck checkbox
```

### Evidence Collection

```bash
agent-browser screenshot            # Screenshot to stdout
agent-browser screenshot path.png   # Save screenshot to file
agent-browser screenshot --full     # Full page screenshot
agent-browser get text @e1          # Get element text for assertion
agent-browser get url               # Get current URL
agent-browser get title             # Get page title
```

### Waiting

```bash
agent-browser wait @e1                  # Wait for element to appear
agent-browser wait --text "Success"     # Wait for text to appear
agent-browser wait --load networkidle   # Wait for network idle
agent-browser wait 2000                 # Wait milliseconds
```

## Key Rules

1. **Always re-snapshot after navigation or DOM changes.** Element refs (`@e1`, `@e2`) become stale after page transitions.
2. **Take screenshots at every verification checkpoint.** Store them in the QA artifact directory.
3. **Use `snapshot -i` for interaction planning.** It returns only interactive elements, which is more token-efficient and easier to parse than a full accessibility tree.
4. **Use semantic waiting** (`wait --text`, `wait @e1`) instead of fixed-duration waits whenever possible.
5. **Chain commands with `&&`** when intermediate output is not needed. Run separately when snapshot output must be parsed before the next action.

## Web UI QA Categories

### Functional Flows
- [ ] Core navigation works (page transitions, links, routing)
- [ ] Primary user actions complete successfully (create, read, update, delete)
- [ ] Form submissions process correctly with valid data
- [ ] Success states and confirmations display after completing actions

### Form Validation
- [ ] Required fields show errors when left empty
- [ ] Format validation triggers for email, phone, URL, etc.
- [ ] Boundary inputs are handled (max length, special characters)
- [ ] Error messages are visible and descriptive

### Error and Loading States
- [ ] Loading indicators appear during async operations
- [ ] Error messages display when API calls fail or network errors occur
- [ ] Empty states render correctly when no data exists
- [ ] 404 or not-found pages display for invalid routes

### Navigation and Routing
- [ ] All primary navigation links work
- [ ] Back/forward browser navigation behaves correctly
- [ ] Deep links resolve to the correct page
- [ ] Protected routes redirect unauthenticated users appropriately

### Responsive Behavior

Test when the changed surface affects layout or when responsive behavior is regression-critical.

- [ ] Layout adapts at mobile viewport (375px wide)
- [ ] Layout adapts at tablet viewport (768px wide)
- [ ] Layout renders correctly at desktop viewport (1280px wide)
- [ ] No horizontal overflow or content truncation at any breakpoint

### Accessibility Basics
- [ ] Interactive elements are reachable via keyboard (tab order)
- [ ] Focus indicators are visible on interactive elements
- [ ] Form inputs have associated labels
- [ ] Images have alt text

## Authentication Flows

When the application requires authentication:

```bash
# Login flow
agent-browser open <login-url>
agent-browser snapshot -i
agent-browser fill @e1 "<username>"    # Username/email field
agent-browser fill @e2 "<password>"    # Password field
agent-browser click @e3                # Submit button
agent-browser wait --load networkidle
agent-browser snapshot -i              # Verify authenticated state

# Save state for reuse across flows
agent-browser state save <qa-output-path>/auth.json

# Restore in a subsequent flow
agent-browser state load <qa-output-path>/auth.json
agent-browser open <authenticated-url>
```

Use saved authentication state to avoid repeating the login flow for every scenario.

## Viewport Testing

Test at different viewport sizes using separate sessions:

```bash
agent-browser --session mobile open <url> --viewport 375x812
agent-browser --session mobile snapshot -i
agent-browser --session mobile screenshot <qa-output-path>/mobile.png

agent-browser --session tablet open <url> --viewport 768x1024
agent-browser --session tablet snapshot -i
agent-browser --session tablet screenshot <qa-output-path>/tablet.png

agent-browser --session desktop open <url> --viewport 1280x800
agent-browser --session desktop snapshot -i
agent-browser --session desktop screenshot <qa-output-path>/desktop.png
```

## Pipeline Position

Browser-based QA runs **after** CLI-based validation in this order:

1. Lint + type-check + static analysis
2. Build
3. Unit tests
4. Integration tests
5. **Start dev server -> Browser/E2E flows** (Web UI QA)
6. Visual regression (if baselines exist)
