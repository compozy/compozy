import { Bell, ChevronsUpDown, Settings } from "lucide-react";

import { Icon, Logo, Menubar, MenubarTrigger } from "@compozy/ui";

import { cn } from "@/lib/utils";

/**
 * The desktop menubar: CompozyOS mark, Global globe toggle, workspace chip, app
 * menus, the approvals bell, the ⌘K palette chip, and Settings. Glass shell
 * chrome (the sanctioned carve-out).
 *
 * The mark and the workspace chip are separate `role="menubar"`s so the globe
 * toggle can sit between them without becoming a menu item. App menus follow
 * in a third menubar. Compact chrome can hide Session/Go/Window/Help without
 * losing mark + toggle + chip. The bell, the ⌘K chip, and the settings cog
 * stay outside all three: they are controls, not menus.
 *
 * The shell owns the menus themselves; a control renders as a real trigger only
 * when a menu owner is supplied, otherwise as truthful presentation.
 */
export interface OsMenuBarProps extends React.ComponentProps<"header"> {
  /**
   * Active scope identity. `worktree` is present only while a worktree can
   * actually receive work — a missing one reverts the chip to the workspace so
   * the bar never advertises a scope that cannot run anything.
   */
  workspace: { name: string; monogram: string; worktree?: string | null };
  /** Functional notice rendered outside `role="menu"` (e.g. scope fell back). */
  scopeNotice?: React.ReactNode;
  /** Composed `<MenubarMenu>` children rendered after the workspace chip. */
  menus?: React.ReactNode;
  /** Approvals count from the bell aggregator; 0/undefined renders no badge. */
  notifications?: number;
  /** Non-interactive system status rendered in the trailing cluster, before the bell. */
  status?: React.ReactNode;
  onCommandClick?: () => void;
  onSettingsClick?: () => void;
  /** Live daemon binding for the command-palette chip. */
  commandShortcutLabel?: string;
  /** Renders the CompozyOS mark inside its system-menu owner (shell wiring). */
  logoMenu?: (trigger: React.ReactNode) => React.ReactNode;
  /** Renders the workspace chip inside its menu owner (shell wiring). */
  workspaceMenu?: (trigger: React.ReactNode) => React.ReactNode;
  /**
   * Global-scope globe toggle. Lives between the mark and the workspace chip,
   * outside `role="menubar"`, so compact chrome can hide app menus without
   * losing mark + toggle + chip.
   */
  scopeControl?: React.ReactNode;
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
  scopeNotice,
  menus,
  notifications,
  status,
  onCommandClick,
  onSettingsClick,
  commandShortcutLabel,
  logoMenu,
  workspaceMenu,
  scopeControl,
  wrapBellTrigger,
  className,
  ...props
}: OsMenuBarProps) {
  // Compact (<960px, `OS_COMPACT_BREAKPOINT`): the shell stops passing `menus`;
  // mark, globe toggle, and chip stay leading. Hidden actions stay in the palette.
  const wrapMenus = Boolean(logoMenu || workspaceMenu);
  const logoControl = (
    <MenuControl
      data-slot="os-menubar-logo"
      aria-label="CompozyOS"
      className="grid size-7 place-items-center rounded-menubar-control p-0"
      menu={logoMenu}
    >
      <Logo variant="symbol" decorative className="size-menubar-logo" />
    </MenuControl>
  );
  const workspaceControl = (
    <MenuControl
      data-slot="os-menubar-workspace"
      className="flex h-7 items-center gap-menubar-workspace-gap rounded-md px-2"
      menu={workspaceMenu}
    >
      <span className="grid size-workspace-avatar place-items-center rounded-sm border border-line-strong bg-elevated font-mono text-badge font-semibold tracking-mono text-fg">
        {workspace.monogram}
      </span>
      <span className="text-small-body font-semibold text-fg-strong">{workspace.name}</span>
      {workspace.worktree ? (
        <>
          <span aria-hidden="true" className="text-small-body text-faint">
            /
          </span>
          <span
            data-slot="os-menubar-worktree"
            className="max-w-40 truncate text-small-body text-fg"
          >
            {workspace.worktree}
          </span>
        </>
      ) : null}
      <Icon as={ChevronsUpDown} size="sm" className="text-subtle" />
    </MenuControl>
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
      <div className="flex min-w-0 items-center gap-1">
        <div data-slot="os-menubar-identity" className="flex items-center gap-1">
          {wrapMenus ? (
            <Menubar aria-label="System menu" className="gap-1">
              {logoControl}
            </Menubar>
          ) : (
            logoControl
          )}
          {scopeControl}
          {wrapMenus ? (
            <Menubar aria-label="Workspace" className="gap-1">
              {workspaceControl}
            </Menubar>
          ) : (
            workspaceControl
          )}
        </div>
        {menus ? (
          <Menubar data-slot="os-menubar-menus" aria-label="App menus" className="gap-1">
            {menus}
          </Menubar>
        ) : null}
      </div>

      <div className="flex items-center gap-2">
        {/* Outside the menubar's `role="menu"` subtree on purpose: a notice is
            not a menu item, and nesting it there breaks the menu's semantics. */}
        {scopeNotice}
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
        {commandShortcutLabel ? (
          <Control
            data-slot="os-menubar-command"
            title="Command palette"
            className="flex h-menubar-chip items-center rounded-md border border-line px-2.5 font-mono text-eyebrow text-muted"
            onClick={onCommandClick}
          >
            {commandShortcutLabel}
          </Control>
        ) : null}
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
