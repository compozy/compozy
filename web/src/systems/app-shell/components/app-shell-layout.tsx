import type { ReactElement, ReactNode } from "react";

import { Link, useRouterState } from "@tanstack/react-router";
import { ChevronsUpDown, LayoutDashboard, ListOrdered } from "lucide-react";

import {
  AppShell,
  AppShellBrand,
  AppShellContent,
  AppShellHeader,
  AppShellMain,
  AppShellNavSection,
  AppShellSidebar,
  StatusBadge,
  cn,
} from "@compozy/ui";

import type { Workspace } from "../types";

export interface AppShellLayoutProps {
  activeWorkspace: Workspace;
  workspaces: Workspace[];
  onSwitchWorkspace: () => void;
  header?: ReactNode;
  children: ReactNode;
}

interface NavEntry {
  href: string;
  label: string;
  matchPrefix: string;
  icon: ReactNode;
  testId: string;
}

const primaryNav: NavEntry[] = [
  {
    href: "/",
    label: "Dashboard",
    matchPrefix: "/",
    icon: <LayoutDashboard className="size-3.5" aria-hidden />,
    testId: "app-nav-dashboard",
  },
];

const acrossWorkflowsNav: NavEntry[] = [
  {
    href: "/workflows",
    label: "Workflows",
    matchPrefix: "/workflows",
    icon: <ListOrdered className="size-3.5" aria-hidden />,
    testId: "app-nav-workflows",
  },
];

export function AppShellLayout({
  activeWorkspace,
  workspaces,
  onSwitchWorkspace,
  header,
  children,
}: AppShellLayoutProps): ReactElement {
  const pathname = useRouterState({ select: state => state.location.pathname });
  const canSwitch = workspaces.length > 1;

  return (
    <AppShell data-testid="app-shell-layout">
      <AppShellSidebar>
        <AppShellBrand
          badge={<StatusBadge tone="accent">daemon</StatusBadge>}
          detail="localhost · operator runtime"
          title="Compozy"
        />

        <div className="space-y-5">
          <AppShellNavSection title="Workspace">
            {primaryNav.map(entry => (
              <NavLinkItem key={entry.href} active={matchActive(pathname, entry)} entry={entry} />
            ))}
          </AppShellNavSection>

          <AppShellNavSection title="Across workflows">
            {acrossWorkflowsNav.map(entry => (
              <NavLinkItem key={entry.href} active={matchActive(pathname, entry)} entry={entry} />
            ))}
          </AppShellNavSection>
        </div>

        <div
          className="mt-auto rounded-[var(--radius-md)] border border-border bg-black/10 p-3"
          data-testid="app-shell-workspace-card"
        >
          <p className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
            Active workspace
          </p>
          <p
            className="mt-1 truncate font-display text-lg tracking-[-0.01em] text-foreground"
            data-testid="app-shell-active-workspace-name"
            title={activeWorkspace.name}
          >
            {activeWorkspace.name}
          </p>
          <p
            className="mt-1 truncate text-xs text-muted-foreground"
            data-testid="app-shell-active-workspace-root"
            title={activeWorkspace.root_dir}
          >
            {activeWorkspace.root_dir}
          </p>
          {canSwitch ? (
            <button
              className="mt-3 inline-flex items-center gap-1 text-xs text-accent hover:underline"
              data-testid="app-shell-switch-workspace"
              onClick={onSwitchWorkspace}
              type="button"
            >
              <ChevronsUpDown className="size-3.5" aria-hidden />
              Switch workspace
            </button>
          ) : null}
        </div>
      </AppShellSidebar>

      <AppShellMain>
        <AppShellHeader>{header}</AppShellHeader>
        <AppShellContent>{children}</AppShellContent>
      </AppShellMain>
    </AppShell>
  );
}

function matchActive(pathname: string, entry: NavEntry): boolean {
  if (entry.matchPrefix === "/") {
    return pathname === "/";
  }
  return pathname === entry.matchPrefix || pathname.startsWith(`${entry.matchPrefix}/`);
}

function NavLinkItem({ active, entry }: { active: boolean; entry: NavEntry }): ReactElement {
  return (
    <Link
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex w-full items-center gap-3 rounded-[calc(var(--radius)-2px)] px-3 py-2 text-left transition-colors",
        active
          ? "bg-sidebar-accent text-sidebar-accent-foreground"
          : "text-muted-foreground hover:bg-accent hover:text-foreground"
      )}
      data-testid={entry.testId}
      to={entry.href}
    >
      <span
        aria-hidden
        className={cn(
          "flex size-4 shrink-0 items-center justify-center",
          active ? "text-primary" : "text-muted-foreground"
        )}
      >
        {entry.icon}
      </span>
      <span className="min-w-0 flex-1 truncate text-sm">{entry.label}</span>
    </Link>
  );
}
