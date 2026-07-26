import { Bell, ChevronsUpDown, Settings } from "lucide-react";

import { Icon, Logo, Menubar, MenubarTrigger } from "@agh/ui";

import { cn } from "@/lib/utils";

/**
 * The desktop menubar: AGH mark, workspace chip, app menus, the approvals bell,
 * the ⌘K palette chip, and Settings. Glass shell chrome (the sanctioned
 * carve-out).
 *
 * The mark, the workspace chip, and every app menu are menubar items, so one
 * `role="menubar"` covers them all — arrow keys traverse the whole bar and
 * hovering a sibling switches the open menu. The bell, the ⌘K chip, and the
 * settings cog stay outside it: they are controls, not menus.
 *
 * The shell owns the menus themselves; a control renders as a real trigger only
 * when a menu owner is supplied, otherwise as truthful presentation.
 */
export interface OsMenuBarProps extends React.ComponentProps<"header"> {
  /** Active workspace identity. */
  workspace: { name: string; monogram: string };
  /** Composed `<MenubarMenu>` children rendered after the workspace chip. */
  menus?: React.ReactNode;
  /** Approvals count from the bell aggregator; 0/undefined renders no badge. */
  notifications?: number;
  /** Non-interactive system status rendered before the approvals bell. */
  status?: React.ReactNode;
  onCommandClick?: () => void;
  onSettingsClick?: () => void;
  /** Renders the AGH mark inside its system-menu owner (shell wiring). */
  logoMenu?: (trigger: React.ReactNode) => React.ReactNode;
  /** Renders the workspace chip inside its menu owner (shell wiring). */
  workspaceMenu?: (trigger: React.ReactNode) => React.ReactNode;
  /** Wraps the bell in its popover owner (shell wiring). */
  wrapBellTrigger?: (trigger: React.ReactElement) => React.ReactNode;
}

const INTERACTIVE =
  "transition-colors duration-base hover:bg-btn-default-fill hover:text-fg-strong focus-visible:shadow-focus-ring focus-visible:outline-none";

interface ControlProps extends Omit<React.ComponentProps<"button">, "onClick" | "children"> {
  onClick?: () => void;
  children: React.ReactNode;
  /** Hands the button to an overlay owner (popover trigger `render`). */
  wrap?: (trigger: React.ReactElement) => React.ReactNode;
}

/**
 * Renders a <button> when a callback or overlay owner exists, else a
 * non-interactive span (truthful chrome, no dead buttons).
 */
function Control({ onClick, wrap, className, children, ...props }: ControlProps) {
  if (wrap) {
    return wrap(
      <button type="button" className={cn(INTERACTIVE, className)} {...props}>
        {children}
      </button>
    );
  }
  if (!onClick) {
    return (
      <span className={className} {...(props as React.ComponentProps<"span">)}>
        {children}
      </span>
    );
  }
  return (
    <button type="button" className={cn(INTERACTIVE, className)} onClick={onClick} {...props}>
      {children}
    </button>
  );
}

interface MenuControlProps extends Omit<React.ComponentProps<"button">, "children"> {
  children: React.ReactNode;
  /** Renders the trigger inside its `MenubarMenu` owner (shell wiring). */
  menu?: (trigger: React.ReactNode) => React.ReactNode;
}

/**
 * A menubar item: a real `MenubarTrigger` when a menu owns it, else inert
 * chrome. Chrome classes ride on the trigger itself so `MenubarTrigger`'s own
 * defaults are merged away instead of stacking.
 */
function MenuControl({ menu, className, children, ...props }: MenuControlProps) {
  if (!menu) {
    return (
      <span className={className} {...(props as React.ComponentProps<"span">)}>
        {children}
      </span>
    );
  }
  return menu(
    <MenubarTrigger className={cn(INTERACTIVE, className)} {...props}>
      {children}
    </MenubarTrigger>
  );
}

function NotificationBadge({ count }: { count: number }) {
  return (
    <span className="absolute top-0.5 right-0 grid h-3.5 min-w-3.5 place-items-center rounded-full bg-accent px-1 font-mono text-micro font-bold text-accent-ink">
      {count > 9 ? "9+" : count}
    </span>
  );
}

export function OsMenuBar({
  workspace,
  menus,
  notifications,
  status,
  onCommandClick,
  onSettingsClick,
  logoMenu,
  workspaceMenu,
  wrapBellTrigger,
  className,
  ...props
}: OsMenuBarProps) {
  // Compact (<960px, `OS_COMPACT_BREAKPOINT`): the shell stops passing `menus`;
  // every action stays reachable through the palette (US-019.EC-3).
  const bar = (
    <>
      <MenuControl
        data-slot="os-menubar-logo"
        aria-label="AGH"
        className="grid size-7 place-items-center rounded-menubar-control p-0"
        menu={logoMenu}
      >
        <Logo variant="symbol" decorative className="size-menubar-logo" />
      </MenuControl>
      <MenuControl
        data-slot="os-menubar-workspace"
        className="flex h-7 items-center gap-menubar-workspace-gap rounded-md px-2"
        menu={workspaceMenu}
      >
        <span className="grid size-workspace-avatar place-items-center rounded-sm border border-line-strong bg-elevated font-mono text-badge font-semibold tracking-mono text-fg">
          {workspace.monogram}
        </span>
        <span className="text-small-body font-semibold text-fg-strong">{workspace.name}</span>
        <Icon as={ChevronsUpDown} size="sm" className="text-subtle" />
      </MenuControl>
      {menus}
    </>
  );

  return (
    <header
      data-slot="os-menubar"
      aria-label="System bar"
      className={cn(
        "flex h-menubar shrink-0 items-center justify-between border-b border-line bg-shell-glass px-2.5 backdrop-blur-shell",
        className
      )}
      {...props}
    >
      {logoMenu || workspaceMenu || menus ? (
        <Menubar data-slot="os-menubar-menus" aria-label="System menus" className="gap-1">
          {bar}
        </Menubar>
      ) : (
        <div data-slot="os-menubar-menus" className="flex items-center gap-1">
          {bar}
        </div>
      )}

      <div className="flex items-center gap-2">
        {status}
        <Control
          data-slot="os-menubar-bell"
          // The badge count reaches assistive tech through the label — the
          // visible badge alone would be stripped by a bare "Approvals" name.
          aria-label={notifications ? `Approvals, ${notifications} waiting` : "Approvals"}
          aria-haspopup={wrapBellTrigger ? "true" : undefined}
          className="relative grid size-7 place-items-center rounded-md text-muted"
          wrap={wrapBellTrigger}
        >
          <Icon as={Bell} size="lg" />
          {notifications ? <NotificationBadge count={notifications} /> : null}
        </Control>
        <Control
          data-slot="os-menubar-command"
          title="Command palette"
          className="flex h-menubar-chip items-center rounded-md border border-line px-2.5 font-mono text-eyebrow text-muted"
          onClick={onCommandClick}
        >
          ⌘K
        </Control>
        <Control
          data-slot="os-menubar-settings"
          aria-label="Settings"
          title="Settings"
          className="grid size-7 place-items-center rounded-md text-muted"
          onClick={onSettingsClick}
        >
          <Icon as={Settings} size="lg" />
        </Control>
      </div>
    </header>
  );
}
