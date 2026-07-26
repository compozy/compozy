# AGH first run — onboarding as a shell panel

**Scope.** The OS shell is delivered: menu bar, dock, window manager, desktops, ⌘K and window-scoped modals all ship from `web/src/systems/os`. Onboarding is the one surface still on the pre-OS layout — a full-page two-column wizard with its own chrome that renders *instead of* the desktop. This document covers only that change.

**Prototype.** `onboarding.html` + `onboarding.css` + `onboarding.js`. Both shell sheets (`os-v2.css`, `os-v2-apps.css`) load unmodified; `onboarding.css` only adds state-scoped rules (`body[data-onboarding]`, `.dock-zone[data-locked]`) and never redefines a shell component.

## The change in one line

`DesktopGate` stops swapping the desktop for a wizard and starts rendering setup **over** it.

```diff
  if (onboarding.data?.completed === true) return children;

- if (onboarding.data?.completed === false) {
-   return <OnboardingWizard onComplete={() => void onboarding.refetch()} />;
- }
+ if (onboarding.data?.completed === false) {
+   return (
+     <>
+       <div inert>{children}</div>
+       <OnboardingSetupPanel onComplete={() => void onboarding.refetch()} />
+     </>
+   );
+ }
```

Loading and error states keep today's `GateFrame` — before the daemon answers there is nothing truthful to render behind a scrim.

## Panel anatomy

The panel borrows the window grammar without being a window.

| Zone | Height | Contents |
| --- | --- | --- |
| Head | 52px | AGH mark · `Set up AGH` · flex · `Runs locally` chip |
| Step strip | 44px | Two segments (`1 Runtime`, `2 Workspace`), each with a 2px progress rule; the slot the OS context strip occupies |
| Body | animated | One pane per step; height measured from content, width 660px → 960px |
| Footer | 58px | What will be saved (label + mono value) · `Back` · `Continue` \| `Finish setup` |

Rules:

- **No traffic lights.** The panel cannot be closed, minimised or zoomed, so it never renders a control it would have to disable. Non-dismissible `Dialog` — `showCloseButton={false}`, Esc and outside-press disabled.
- **Identity renders once**, in the head. No stepper rail, no page head, no route breadcrumb.
- **The panel resizes per step.** Step 1 is a decision → one column. Step 2 is browse-then-confirm → a two-pane split whose columns close flush. Width and height animate together on `--ease-spring`.
- **The shell stays legible behind it**: `rgba(6,5,5,.5)` + `blur(3px)`, menu bar at 68% opacity, dock dimmed to 50% and offset 6px down. Enough to read the desktop you are about to unlock, never enough to read as available.
- **Focus is contained** to the panel (plus the runtime popover, which portals above it) for as long as setup is open.

## The two steps

Both keep the contract in `web/src/systems/onboarding` verbatim — nothing was added to it.

| Step | Content | Continue is enabled when |
| --- | --- | --- |
| 1 · Runtime | `RuntimeSelector` (provider · model · reasoning) + a one-line facts strip for the selected model (context, price, tools, reasoning levels, harness) + auth mode (`native_cli` \| `bound_secret`) + env var / API key when bound | `useOnboardingDefaultModel().isValid` — provider set, provider + general settings loaded, bound secret has a target env |
| 2 · Workspace | `DirectoryBrowser` (`GET /api/fs/browse`, `dirs_only`) on the left, selected workspaces on the right, network mention below the split | at least one workspace resolved |

Auth mode defaults from the provider harness (`acp` → CLI, `pi_acp` → API key) and clears bound credentials when the provider changes, as `updateRuntime` already does. Step 2 names the first resolved workspace in the menu bar behind the scrim — the shell fills in while you are still setting up.

## `web/` component map

| File | Change |
| --- | --- |
| `systems/os/components/desktop-gate.tsx` | Render chrome + panel instead of swapping; mark the chrome subtree `inert` while setup is open |
| `systems/onboarding/components/onboarding-wizard.tsx` | Rewritten as `onboarding-setup-panel.tsx`: `Dialog` host, 52px head, 44px step strip, animated body, 58px footer |
| `systems/onboarding/components/step-default-model.tsx` | Keep sections and fields; add the model facts line; drop the outer page padding |
| `systems/onboarding/components/step-workspaces.tsx` | Same data, re-laid out as the two-pane split; the network mention moves below the split |
| `systems/onboarding/components/directory-browser.tsx` | Unchanged |
| `systems/onboarding/hooks/*`, `stores/use-onboarding-draft-store.ts`, `adapters/*` | Unchanged — `step`, `maxStep`, `goToStep`, `commit`, `finish` already model this flow |
| `systems/onboarding/index.ts` | Export the panel instead of the wizard |
| `routes/_app/-app-preload.ts` | Unchanged — it already skips route preloading until `completed === true` |

**Delete targets.** The wizard's `grid h-dvh md:grid-cols-[360px_1fr]` shell, its `<aside>` rail, the `Stepper` block, the page `<header>` (eyebrow + title + lead) and the page `<footer>`. `Stepper*` then has no consumer left in `web/` — decide keep-or-delete on the `@agh/ui` export in the same change rather than leaving it orphaned.

## States

| State | Panel |
| --- | --- |
| Status loading / error | Panel not mounted; today's `GateFrame` spinner or retry `Empty` |
| Step 1 invalid | Footer swaps the summary for the danger message (`Enter the environment variable the provider expects.`), env input goes `aria-invalid`, `Continue` disabled |
| Step 1 commit fails | Message stays in the footer, step does not advance, `commit` error surfaces as today |
| Step 2 empty | Dashed empty state in the right pane, footer reads `None yet — add at least one folder to finish.` |
| Committing | `Finish setup` shows a spinner and disables |
| Complete | Panel exits, chrome un-inerts, dock wakes with a staggered lift, desktop hint appears |

## Requirements this places on the shell

- **The chrome must render with zero workspaces and zero windows.** That is the first-run desktop: wallpaper, a menu bar whose workspace slot reads `No workspace`, and a dock with nothing running. Anything in `DesktopChrome` that assumes an active workspace needs a null path.
- **No route data loads before setup completes** — already true via `-app-preload.ts`.

## Verification

- New test on `DesktopGate` (none exists today): chrome renders behind while `completed === false`, the panel is present and blocking, and completion reveals the desktop without remounting it.
- Existing onboarding hook tests are untouched; `onboarding-steps.stories.tsx` re-hosts on the panel.
- `make verify`, plus an `agh-ui-screenshot` capture of both steps for the visual contract.

## Non-goals

Step count, field set, endpoints and validation are unchanged. No new onboarding step, no provider sign-in flow inside setup, no desktop widgets, no change to Network behaviour — finishing setup still does not enable coordination.
