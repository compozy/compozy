/**
 * The draft app shell: the production `DesktopShell` grid (topbar across the top, rail, inset desk)
 * from the real shell pieces that render from props, around one zoomed window. The wired pieces
 * reach the daemon, so the frame around the real parts is local: `DraftTabBar` lays out plates the
 * way `OsTabBar` does, `DraftRailFoot` stands in for `OsRailFoot`, and the plain menu row stands
 * in for `DesktopMenubar`'s owned menus.
 */
import { Settings } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Icon, Kbd, Pill, ThemeToggle, useTopbarSlot, type TopbarSlotValue } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { DesktopPager } from "@/systems/os/components/desktop-pager";
import { OsDock, OsRailButton } from "@/systems/os/components/os-dock";
import { OsMenuBar, type OsMenuBarProps } from "@/systems/os/components/os-menubar";
import { OsWindowSurface } from "@/systems/os/components/os-window-surface";
import type { OsDockItemData } from "@/systems/os/lib/os-dock-model";
import { applyTheme } from "@/systems/theme/lib/apply-theme";

import { useCanvasTheme } from "./use-canvas-theme";

export type DraftApp = "sessions" | "agents" | "dashboard";

const DOCK = [
  { id: "dashboard", name: "Dashboard", icon: "dashboard" },
  { id: "sessions", name: "Sessions", icon: "sessions" },
  { id: "terminal", name: "Terminal", icon: "terminal" },
  { id: "agents", name: "Agents", icon: "agents" },
  { id: "tasks", name: "Tasks", icon: "tasks" },
  { id: "loops", name: "Loops", icon: "loops" },
  { id: "automations", name: "Automations", icon: "automations" },
  { id: "marketplace", name: "Marketplace", icon: "marketplace" },
  { id: "vault", name: "Vault", icon: "vault" },
] as const;

const DESKTOPS = [
  { id: "control", name: "Control" },
  { id: "build", name: "Build" },
  { id: "review", name: "Review" },
];

const MENUS = ["Session", "Go", "Window", "Help"];

function dockItems(app: DraftApp, sessionBadge: number): OsDockItemData[] {
  return DOCK.map(item => ({
    ...item,
    running: item.id === app || item.id === "terminal",
    active: item.id === app,
    badge: item.id === "sessions" && sessionBadge > 0 ? sessionBadge : undefined,
  }));
}

function DraftRailFoot() {
  const theme = useCanvasTheme();
  return (
    <>
      <ThemeToggle
        onClick={() => applyTheme(theme === "dark" ? "light" : "dark")}
        resolvedTheme={theme}
      />
      <OsRailButton label="Settings">
        <Icon as={Settings} className="size-4" />
      </OsRailButton>
    </>
  );
}

export interface DraftTab {
  id: string;
  title: ReactNode;
  glyph?: ReactNode;
  /** Decisions waiting in that session; the tab counts them. */
  pending?: number;
}

/** `OsTabBar` without the window manager: one plate per tab on the bar's own surface. */
function DraftTabBar({ tabs, activeId }: { tabs: readonly DraftTab[]; activeId: string }) {
  return (
    <div className="flex h-10 min-w-0 items-center gap-1 bg-rail pr-2 pl-1.5" data-slot="os-tab-bar">
      <div aria-label="Tabs" className="flex min-w-0 flex-1 items-center gap-1" role="tablist">
        {tabs.map(tab => (
          <div
            aria-selected={tab.id === activeId}
            className={cn(
              "flex h-6.75 max-w-56 min-w-10 flex-[1_1_14rem] items-center gap-2 rounded-sm px-2 text-body font-medium",
              tab.id === activeId ? "bg-fg/7 text-fg" : "bg-fg/4 text-fg-2"
            )}
            key={tab.id}
            role="tab"
          >
            {tab.glyph}
            <span className="min-w-0 flex-1 truncate">{tab.title}</span>
            <Pill.Count count={tab.pending ?? 0} />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Publishes the head zones the way a route does, from inside the window's slot provider. */
function PublishTopbar({ value }: { value: TopbarSlotValue }) {
  useTopbarSlot(value);
  return null;
}

export interface DraftShellProps {
  app: DraftApp;
  /** The window title; `topbar.crumb` overrides it when a route publishes its own leaf. */
  title: ReactNode;
  glyph?: ReactNode;
  /** Host context before the title, as the real window shows its project. */
  leading?: ReactNode;
  topbar?: TopbarSlotValue;
  tabs?: readonly DraftTab[];
  activeTabId?: string;
  /** Project chip in the system bar. */
  scope?: string;
  /** The dock's needs-you count. */
  sessionBadge?: number;
  /** Bell count in the system bar. */
  notifications?: number;
  children: ReactNode;
  /** Dialogs and popovers that sit over the window. */
  overlay?: ReactNode;
  /** Wraps the system bar's bell in its popover, as `DesktopMenubar` does. */
  wrapBellTrigger?: OsMenuBarProps["wrapBellTrigger"];
}

/** Topbar, rail, desk and one window around a surface, as in the running product. */
export function DraftShell({
  app,
  title,
  glyph,
  leading,
  topbar,
  tabs,
  activeTabId,
  scope = "compozy",
  sessionBadge = 1,
  notifications = 1,
  children,
  overlay,
  wrapBellTrigger,
}: DraftShellProps) {
  return (
    <div className="relative grid h-dvh w-full grid-cols-[auto_minmax(0,1fr)] grid-rows-[auto_auto_minmax(0,1fr)] overflow-hidden bg-rail text-fg">
      <OsMenuBar
        className="col-span-full"
        commandShortcutLabel="⌘K"
        menus={MENUS.map(menu => (
          <span
            className="flex h-8.5 items-center rounded-md px-2.5 text-body text-muted-foreground"
            key={menu}
          >
            {menu}
          </span>
        ))}
        notifications={notifications}
        pager={
          <DesktopPager
            activeDesktopId="build"
            desktops={DESKTOPS}
            onOpenOverview={() => undefined}
            onSelectDesktop={() => undefined}
          />
        }
        workspace={{ name: scope }}
        wrapBellTrigger={wrapBellTrigger}
      />
      <OsDock
        className="col-start-1 row-span-2 row-start-2"
        foot={<DraftRailFoot />}
        items={dockItems(app, sessionBadge)}
        onSelect={() => undefined}
      />
      <div className="col-start-2 row-start-2 min-w-0">
        {tabs && tabs.length > 0 ? (
          <DraftTabBar activeId={activeTabId ?? tabs[0]!.id} tabs={tabs} />
        ) : (
          <div className="h-10 bg-rail" />
        )}
      </div>
      <div
        className="relative col-start-2 row-start-3 min-h-0 overflow-hidden rounded-tl-lg border-t border-l border-line"
        data-slot="os-desk"
      >
        <OsWindowSurface
          className="absolute inset-0"
          glyph={glyph}
          leading={leading}
          onClose={() => undefined}
          title={title}
        >
          {topbar ? <PublishTopbar value={topbar} /> : null}
          {children}
        </OsWindowSurface>
      </div>
      {overlay}
    </div>
  );
}

export function DraftUnsavedPill({ count }: { count: number }) {
  return (
    <Pill className="border border-dashed border-line-strong" form="plain" size="sm">
      {count} unsaved
    </Pill>
  );
}

export function DraftShortcutHint({ children }: { children: ReactNode }) {
  return <Kbd>{children}</Kbd>;
}
