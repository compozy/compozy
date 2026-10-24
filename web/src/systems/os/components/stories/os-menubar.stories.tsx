import { useEffect } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http } from "msw";
import { fn, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import type { ProviderUsageEnvelope } from "@/systems/provider-usage";
import {
  codexOkTarget,
  defaultUsageEnvelope,
  providerUsageHandlers,
  providerUsageStreamResponse,
  statusTargets,
  usageEnvelope,
  usageTray,
} from "@/systems/provider-usage/mocks";
import type { SettingsUpdateStatus } from "@/systems/settings";
import { settingsUpdateIndicatorState } from "@/systems/settings";
import {
  settingsUpdateApplyingFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateRolledBackFixture,
  settingsUpdateStatusFixture,
} from "@/systems/settings/mocks";
import { DESTINATION_COPY } from "@/systems/workspace";

import { shortcutLabel } from "../../lib/window-manager-shortcuts";
import { OsMenuBar } from "../os-menubar";
import { OsHydrationStatus } from "../os-hydration-status";
import { DesktopShell } from "./_desktop";
import {
  WORKSPACES,
  MenubarFixture,
  MENUBAR_NEST_LISTING,
  READY_WORKTREE,
  MISSING_WORKTREE,
  ALL_PROJECTS_CHIP,
  USER_CHIP,
} from "./_menubar-fixture";

const meta: Meta<typeof OsMenuBar> = {
  title: "systems/os/components/OsMenuBar",
  component: OsMenuBar,
  args: { commandShortcutLabel: shortcutLabel("meta+KeyK") },
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          'The desktop menubar: the CompozyOS system menu, the All projects globe, the workspace switcher, the static Session / Go / Window / Help set, then the tray: desktop pager dots, All desktops, Provider usage, the approvals bell, and the ⌘K button. Settings lives in the system menu on the mark. The mark and the workspace chip are separate `role="menubar"`s so the globe can sit between them without becoming a menu item. Compact chrome keeps mark + globe + chip after the app menus hide.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The wired bar at rest, over a live desktop. */
export const Populated: Story = {
  args: { workspace: { name: "compozy" } },
  render: () => <MenubarFixture />,
};

/** All projects on: the chip names All projects and the globe is pressed. */
export const AllProjectsOn: Story = {
  args: { workspace: { name: DESTINATION_COPY.allProjects } },
  render: () => (
    <MenubarFixture
      breadthMode="all"
      focus={{ kind: "project", workspaceId: WORKSPACES[0].id }}
      chip={ALL_PROJECTS_CHIP}
      onToggleAllProjects={fn()}
    />
  ),
};

/** User focused: the chip reads User; the globe is off. */
export const UserFocused: Story = {
  args: { workspace: { name: DESTINATION_COPY.user } },
  render: () => (
    <MenubarFixture
      breadthMode="focused"
      focus={{ kind: "user" }}
      chip={USER_CHIP}
      onToggleAllProjects={fn()}
    />
  ),
};

/** Zero projects: the switcher offers All projects and User; the globe is never locked. */
export const ZeroProjects: Story = {
  args: { workspace: { name: DESTINATION_COPY.user } },
  render: () => (
    <MenubarFixture
      overlay="workspace-menu"
      workspaces={[]}
      breadthMode="focused"
      focus={{ kind: "user" }}
      chip={USER_CHIP}
      onToggleAllProjects={fn()}
    />
  ),
};

/** The system menu on the mark: identity plus the settings surfaces. */
export const CompozyMenuOpen: Story = {
  args: { workspace: { name: "compozy" } },
  render: () => <MenubarFixture overlay="compozy-menu" />,
};

/** Navigation grouped exactly like the dock strip. */
export const GoMenuOpen: Story = {
  args: { workspace: { name: "compozy" } },
  render: () => <MenubarFixture overlay="go-menu" />,
};

/** Window commands with a focused tiled window and a visible peer: all enabled. */
export const WindowMenuOpen: Story = {
  args: { workspace: { name: "compozy" } },
  render: () => <MenubarFixture overlay="window-menu" />,
};

/**
 * The same menu on a cold desktop: no live client, so every command is disabled
 * rather than hidden — the shape of the menu never changes under you.
 */
export const WindowMenuUnavailable: Story = {
  args: { workspace: { name: "compozy" } },
  render: () => <MenubarFixture overlay="window-menu" live={false} />,
};

/** Real help: the shortcut reference, the docs, and a support path. */
export const HelpMenuOpen: Story = {
  args: { workspace: { name: "compozy" } },
  render: () => <MenubarFixture overlay="help-menu" />,
};

/**
 * VC-15 — workspace menu with the shared nested worktree projection.
 * Every row stays in the nest — discovered and missing included.
 * Play hovers the git-backed parent so the side submenu is capturable.
 */
export const WorkspaceMenuOpen: Story = {
  args: { workspace: { name: "compozy" } },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.hover(await body.findByTestId(`os-workspace-option-${WORKSPACES[0].id}`));
    await body.findByTestId(`os-worktree-submenu-${WORKSPACES[0].id}`);
  },
  render: () => <MenubarFixture overlay="workspace-menu" listing={MENUBAR_NEST_LISTING} />,
};

/** All projects on: the switcher checks All projects, then lists User and the projects. */
export const WorkspaceMenuOpenWhileAllProjects: Story = {
  args: { workspace: { name: DESTINATION_COPY.allProjects } },
  render: () => (
    <MenubarFixture
      overlay="workspace-menu"
      breadthMode="all"
      focus={{ kind: "project", workspaceId: WORKSPACES[0].id }}
      chip={ALL_PROJECTS_CHIP}
      onToggleAllProjects={fn()}
    />
  ),
};

/** VC-16 — the shell chip names only the workspace at its root scope. */
export const WorkspaceOnlyChip: Story = {
  args: { workspace: { name: "compozy" } },
  render: () => <MenubarFixture />,
};

/** VC-17 — an active worktree is part of the shell's visible scope identity. */
export const WorkspaceAndWorktreeChip: Story = {
  args: { workspace: { name: "compozy" } },
  render: () => (
    <MenubarFixture
      worktreeSelection={{
        selectedWorktreeId: READY_WORKTREE.id,
        activeWorktree: READY_WORKTREE,
        resolved: true,
        fallback: null,
      }}
    />
  ),
};

/**
 * VC-18 — a missing selection falls back to the parent and states why.
 * Play hovers the parent so the missing row and the bar notice are both
 * capturable.
 */
export const MissingWorktreeFallback: Story = {
  args: { workspace: { name: "compozy" } },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.hover(await body.findByTestId(`os-workspace-option-${WORKSPACES[0].id}`));
    await body.findByTestId(`os-worktree-submenu-${WORKSPACES[0].id}`);
  },
  render: () => (
    <MenubarFixture
      overlay="workspace-menu"
      listing={MENUBAR_NEST_LISTING}
      worktreeSelection={{
        selectedWorktreeId: MISSING_WORKTREE.id,
        activeWorktree: null,
        resolved: true,
        fallback: {
          worktreeId: MISSING_WORKTREE.id,
          name: MISSING_WORKTREE.name,
          reason: "missing",
        },
      }}
    />
  ),
};

/** Presentation-only — no menu owners, so the bar renders as inert chrome. */
export const PresentationOnly: Story = {
  args: { workspace: { name: "compozy" }, notifications: 0 },
  render: args => <DesktopShell wallpaper="carbon" deskHint topbar={<OsMenuBar {...args} />} />,
};

/**
 * macOS Electron — the frameless window's traffic lights own the bar's leading
 * 84px (fallback when `env(titlebar-area-*)` is unavailable); the mark follows.
 */
export const TrafficLightsReserved: Story = {
  args: {
    workspace: { name: "compozy", worktree: "continue-fork" },
    notifications: 3,
    trafficLights: true,
    onCommandClick: fn(),
    onDesktopsClick: fn(),
  },
  render: args => <DesktopShell wallpaper="carbon" deskHint topbar={<OsMenuBar {...args} />} />,
};

/**
 * Degraded desktop sync — the warning stays non-blocking, names the state in
 * text, and leaves every shell command available.
 */
export const DegradedSync: Story = {
  args: {
    workspace: { name: "compozy" },
    status: <OsHydrationStatus hydration="degraded" />,
    notifications: 2,
    onCommandClick: fn(),
    onDesktopsClick: fn(),
  },
  render: args => <DesktopShell wallpaper="carbon" deskHint topbar={<OsMenuBar {...args} />} />,
};

/** Menubar update indicator (S2): one story per state the daemon can report. */
function updateIndicatorStory(snapshot: SettingsUpdateStatus): Story {
  return {
    args: { workspace: { name: "compozy" } },
    render: () => (
      <MenubarFixture
        update={{ state: settingsUpdateIndicatorState(snapshot, false), onUpdate: fn() }}
      />
    ),
  };
}

function FocusUpdateIndicatorSetup() {
  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      document.querySelector<HTMLElement>("[data-testid='os-menubar-update']")?.focus();
    });
    return () => cancelAnimationFrame(frame);
  }, []);
  return null;
}

/**
 * An update is offered: an accent "Update" pill joins the trailing cluster before
 * the approvals bell. One click updates CompozyOS and restarts it in place.
 */
export const UpdateIndicatorAvailable: Story = updateIndicatorStory(
  settingsUpdateBothAvailableFixture
);

/** Keyboard focus exposes the indicator's visible focus treatment. */
export const UpdateIndicatorFocused: Story = {
  ...updateIndicatorStory(settingsUpdateBothAvailableFixture),
  render: () => (
    <>
      <MenubarFixture update={{ state: "available", onUpdate: fn() }} />
      <FocusUpdateIndicatorSetup />
    </>
  ),
};

/** The calm default: nothing is offered, so the indicator is absent from the DOM. */
export const UpdateIndicatorHidden: Story = updateIndicatorStory(settingsUpdateStatusFixture);

/** An operation is live: the pill holds a spinner until the app restarts updated. */
export const UpdateIndicatorUpdating: Story = updateIndicatorStory(settingsUpdateApplyingFixture);

/** The last update failed: the pill turns danger and opens Settings for the detail. */
export const UpdateIndicatorFailed: Story = updateIndicatorStory(settingsUpdateRolledBackFixture);

/** Provider usage in the tray (S1/S2): the shell's profile envelope served over MSW. */
function usageStory(envelope: (now: number) => ProviderUsageEnvelope): Story {
  return {
    args: { workspace: { name: "compozy" } },
    parameters: storybookMswParameters({
      "provider-usage": providerUsageHandlers(() => envelope(Date.now())),
    }),
    render: () => <MenubarFixture />,
  };
}

/**
 * The popover over Claude and Codex: one row per target with its tightest
 * window, header Refresh, and the Open Usage / Settings footer.
 */
export const UsagePopoverOpen: Story = {
  ...usageStory(defaultUsageEnvelope),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const control = await within(canvasElement).findByRole("button", {
      name: /^Claude · Weekly · Opus/,
    });
    await userEvent.click(control);
  },
};

/**
 * The first usage read has not answered: the control already holds its slot, with the meter's
 * shape unmeasured, so the controls beside it do not move when the numbers land.
 */
export const UsageTrayLoading: Story = {
  args: { workspace: { name: "compozy" } },
  parameters: storybookMswParameters({
    "provider-usage": [
      http.get("*/api/provider-usage/stream", () => providerUsageStreamResponse()),
      http.get("*/api/provider-usage", async () => {
        await delay("infinite");
        return new Response(null, { status: 204 });
      }),
    ],
  }),
  render: () => <MenubarFixture />,
};

/** Below 80 %: the meter fills in neutral ink. */
export const UsageTrayNeutral: Story = usageStory(now =>
  usageEnvelope([codexOkTarget(now)], { tray: usageTray({}, now) }, now)
);

/** The tightest window is spent: the meter fills in the danger tone; the label says so. */
export const UsageTrayExhausted: Story = usageStory(now =>
  usageEnvelope(
    [codexOkTarget(now)],
    { tray: usageTray({ used_percent: 112, severity: "exhausted" }, now) },
    now
  )
);

/** The tray window comes from old data: dimmed bar plus the ring marker. */
export const UsageTrayStale: Story = usageStory(now =>
  usageEnvelope(
    [{ ...codexOkTarget(now), status: "stale" }],
    { tray: usageTray({ stale: true }, now) },
    now
  )
);

/** Targets exist but none has a known window: the glyph alone, tooltip "Provider usage". */
export const UsageTrayNoMeter: Story = usageStory(now =>
  usageEnvelope([statusTargets(now).unsupported], {}, now)
);

/** Feature off: the control is absent from the tray, not disabled. */
export const UsageTurnedOff: Story = usageStory(now => usageEnvelope([], { enabled: false }, now));
