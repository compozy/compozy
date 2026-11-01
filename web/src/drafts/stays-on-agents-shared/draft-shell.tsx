/**
 * The draft app shell: the production `DesktopShell` grid (topbar across the top, rail, inset desk)
 * from the real shell pieces that render from props, around one zoomed window. The wired pieces
 * reach the daemon, so the frame around the real parts is local: `DraftDeck` lays out real
 * `OsWindowTabView` plates the way `OsWindowDeck` does, `DraftRailFoot` stands in for `OsRailFoot`,
 * and the plain menu row stands in for `DesktopMenubar`'s owned menus.
 */
import { Settings } from "lucide-react";
import type { ReactNode } from "react";

import { Icon, Kbd, Pill, ThemeToggle, useTopbarSlot, type TopbarSlotValue } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { DesktopPager } from "@/systems/os/components/desktop-pager";
import { OsDock, OsRailButton } from "@/systems/os/components/os-dock";
import { OsMenuBar } from "@/systems/os/components/os-menubar";
import { OsTrafficLights } from "@/systems/os/components/os-traffic-lights";
import { OsWindowChrome, OsWindowSurface } from "@/systems/os/components/os-window-frame";
import {
  OS_WINDOW_DECK_SLOT_CLASS,
  OS_WINDOW_DECK_STRIP_CLASS,
} from "@/systems/os/components/os-window-deck-classes";
import { OsWindowTabView } from "@/systems/os/components/os-window-tab-view";
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
    minimized: false,
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

/** `OsWindowDeck` without the window manager: real tabs in the deck's own strip and slots. */
function DraftDeck({ tabs, activeId }: { tabs: readonly DraftTab[]; activeId: string }) {
  return (
    <div className={OS_WINDOW_DECK_STRIP_CLASS} data-slot="os-window-deck">
      <div
        aria-label="Open tabs"
        className="no-scrollbar flex h-full min-w-0 flex-1 items-end overflow-x-auto px-2.5"
        role="tablist"
      >
        {tabs.map(tab => (
          <div
            className={cn(OS_WINDOW_DECK_SLOT_CLASS, "w-deck-tab-max min-w-deck-tab")}
            data-active={tab.id === activeId ? "" : undefined}
            data-deck-slot=""
            key={tab.id}
          >
            <OsWindowTabView
              active={tab.id === activeId}
              glyph={tab.glyph}
              label={tab.title}
              pendingCount={tab.pending}
              title={tab.title}
              windowId={tab.id}
            />
          </div>
        ))}
      </div>
      <div className="flex h-full shrink-0 items-center pl-2">
        <OsTrafficLights />
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
}: DraftShellProps) {
  const deck = tabs && tabs.length > 1;
  return (
    <div className="relative grid h-dvh w-full grid-cols-[auto_minmax(0,1fr)] grid-rows-[auto_minmax(0,1fr)] overflow-hidden bg-rail text-fg">
      <OsMenuBar
        className="col-span-full"
        commandShortcutLabel="⌘K"
        menus={MENUS.map(menu => (
          <span
            className="flex h-8.5 items-center rounded-md px-2.5 text-body text-muted"
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
      />
      <OsDock
        className="col-start-1 row-start-2"
        foot={<DraftRailFoot />}
        items={dockItems(app, sessionBadge)}
        onSelect={() => undefined}
      />
      <div
        className="relative col-start-2 row-start-2 min-h-0 overflow-hidden rounded-tl-lg border-t border-l border-line"
        data-slot="os-desk"
      >
        <OsWindowChrome className="absolute inset-0" kind="tiled">
          {deck ? <DraftDeck activeId={activeTabId ?? tabs[0]!.id} tabs={tabs} /> : null}
          <OsWindowSurface
            controls={deck ? "deck" : "head"}
            glyph={glyph}
            leading={leading}
            onTrafficLight={() => undefined}
            title={title}
          >
            {topbar ? <PublishTopbar value={topbar} /> : null}
            {children}
          </OsWindowSurface>
        </OsWindowChrome>
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
