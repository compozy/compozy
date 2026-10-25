## 0.4.1 - 2026-10-25

### Bug Fixes

- Strip AppImage runtime state from child environments (#954)
- Align catalog artifacts and expose version incompatibility (#957)
- Recover Claude usage from renewed native credentials (#959)
- Load and cancel session monitors outside the default profile (#961)
- Recover half-removed worktree checkouts instead of failing with 500 (#964)
- Prevent contention in GlobalDB integration upgrades (#965)
- Keep authored line breaks and full composer links in user messages (#968)
- Preserve worktree locks and finish interrupted removal cleanup
- Preserve authored inline code spacing
- Honor app reduced motion in content transitions
- Preserve skeleton delay under reduced motion
- Synchronize ACP connection startup and align tray coverage
- Keep the settings loading rail out of the live navigation

### Features

- Add Kanban card actions, board group/sort, and per-window session order (#963)
- Add content-shaped loading states and React view transitions (#972)
- Finish loading states, window fallbacks and transition fixes for every route (#973)

### Performance Improvements

- Suspend closed palette topology updates (#960)
- Avoid palette context updates for window geometry (#971)
- Defer loading skeletons until restored frames commit

### Testing

- Align catalog fixtures and runtime readiness with current contracts
- Synchronize clarify keepalive log capture
- Retry transient desktop fixture removal failures
- Seed Kanban data before opening the browser page
