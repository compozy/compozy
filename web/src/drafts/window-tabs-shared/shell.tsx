/**
 * The draft app shell around the tab model: the production menubar, pager and rail from props,
 * then the desktop's tab bar on the chrome and the active tab's content below it.
 */
import { Settings } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Icon, ThemeToggle, cn } from "@compozy/ui";

import { DesktopPager } from "@/systems/os/components/desktop-pager";
import { OsDock, OsRailButton } from "@/systems/os/components/os-dock";
import { OsMenuBar } from "@/systems/os/components/os-menubar";
import type { OsDockItemData } from "@/systems/os/lib/os-dock-model";
import { applyTheme } from "@/systems/theme/lib/apply-theme";

import { useCanvasTheme } from "../always-on-agents-shared/use-canvas-theme";
import { DESKTOPS } from "./fixtures";
import type { DraftDesktop } from "./model";

const DOCK = [
  { id: "dashboard", name: "Dashboard", icon: "dashboard" },
  { id: "sessions", name: "Sessions", icon: "sessions" },
  { id: "terminal", name: "Terminal", icon: "terminal" },
  { id: "files", name: "Files", icon: "files" },
  { id: "agents", name: "Agents", icon: "agents" },
  { id: "loops", name: "Loops", icon: "loops" },
  { id: "automations", name: "Automations", icon: "automations" },
  { id: "marketplace", name: "Marketplace", icon: "marketplace" },
  { id: "vault", name: "Vault", icon: "vault" },
] as const;

const MENUS = ["Session", "Go", "Window", "Help"];

function dockItems(): OsDockItemData[] {
  return DOCK.map(item => ({
    ...item,
    running: item.id === "sessions" || item.id === "terminal",
    active: item.id === "sessions",
    badge: item.id === "sessions" ? 2 : undefined,
  }));
}

function RailFoot() {
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

export interface WindowShellProps {
  /** The desktop's tab bar. */
  bar: ReactNode;
  /** The active tab's content. */
  children: ReactNode;
  desktops?: readonly DraftDesktop[];
  activeDesktopId?: string;
  /** Replaces the production pager (the indicator states of `window-desktops`). */
  pager?: ReactNode;
  /** Layers above the desk: drag ghosts, the desktops overview. */
  overlay?: ReactNode;
  /** A notice between the tab bar and the content. */
  notice?: ReactNode;
  /** Compact viewport: no rail. */
  compact?: boolean;
  onOpenOverview?: () => void;
  className?: string;
}

export function WindowShell({
  bar,
  children,
  desktops = DESKTOPS,
  activeDesktopId = "build",
  pager,
  overlay,
  notice,
  compact = false,
  onOpenOverview,
  className,
}: WindowShellProps) {
  return (
    <div
      className={cn(
        "relative grid h-dvh w-full grid-cols-[auto_minmax(0,1fr)] grid-rows-[auto_auto_minmax(0,1fr)] overflow-hidden bg-rail text-fg",
        className
      )}
      data-slot="window-shell"
    >
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
        notifications={2}
        onDesktopsClick={onOpenOverview}
        pager={
          pager ?? (
            <DesktopPager
              activeDesktopId={activeDesktopId}
              desktops={desktops.map(({ id, name, needsYou }) => ({ id, name, needsYou }))}
              onOpenOverview={() => onOpenOverview?.()}
              onSelectDesktop={() => undefined}
            />
          )
        }
        workspace={{ name: "compozy" }}
      />
      {compact ? null : (
        <OsDock
          className="col-start-1 row-span-2 row-start-2"
          foot={<RailFoot />}
          items={dockItems()}
          onSelect={() => undefined}
        />
      )}
      <div className={cn("row-start-2 min-w-0", compact ? "col-span-full" : "col-start-2")}>
        {bar}
      </div>
      <div
        className={cn(
          "relative row-start-3 flex min-h-0 flex-col overflow-hidden border-t border-line",
          compact ? "col-span-full" : "col-start-2 rounded-tl-lg border-l"
        )}
        data-slot="os-desk"
      >
        {notice}
        {children}
      </div>
      {overlay}
    </div>
  );
}
