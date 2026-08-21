import { Bell, ChevronsUpDown, Command, Settings } from "lucide-react";

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
 * losing mark + toggle + chip. The bell, the palette button, and the settings cog
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
  notifications?: number;
  /** Non-interactive system status rendered in the trailing cluster, before the bell. */
  status?: React.ReactNode;
  /**
   * Interactive update offer, rendered before the approvals bell.
   * Separate from `status` because that slot is non-interactive by contract.
   */
  updateIndicator?: React.ReactNode;
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
  /**
   * Profile switcher, rendered between the command-palette trigger and Settings.
   * Quiet-until-plural is the switcher's own business: the bar just gives it the
   * slot.
   */
  profileSwitcher?: React.ReactNode;
}

const WINDOW_DRAG = "[app-region:drag]";
const WINDOW_NO_DRAG = "[app-region:no-drag]";
const INTERACTIVE = [
  WINDOW_NO_DRAG,
  "transition-colors duration-base hover:bg-btn-default-fill hover:text-fg-strong",
  "focus-visible:shadow-focus-ring focus-visible:outline-none",
].join(" ");

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
  updateIndicator,
  onCommandClick,
  onSettingsClick,
  commandShortcutLabel,
  logoMenu,
  workspaceMenu,
  scopeControl,
  wrapBellTrigger,
  profileSwitcher,
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
        "flex h-menubar shrink-0 items-center justify-between border-b border-line bg-shell-glass backdrop-blur-shell select-none",
        WINDOW_DRAG,
        className
      )}
      {...props}
    >
      <div
        data-slot="os-menubar-safe-area"
        className={cn(
          "flex h-full min-w-0 items-center justify-between px-2.5",
          WINDOW_DRAG,
          "ml-[env(titlebar-area-x,0px)] w-[env(titlebar-area-width,100%)]"
        )}
      >
        <div className={cn("flex min-w-0 items-center gap-1", WINDOW_NO_DRAG)}>
          <div data-slot="os-menubar-identity" className="flex items-center gap-1">
            {wrapMenus ? (
              <Menubar aria-label="System menu" className={cn("gap-1", WINDOW_NO_DRAG)}>
                {logoControl}
              </Menubar>
            ) : (
              logoControl
            )}
            {scopeControl}
            {wrapMenus ? (
              <Menubar aria-label="Workspace" className={cn("gap-1", WINDOW_NO_DRAG)}>
                {workspaceControl}
              </Menubar>
            ) : (
              workspaceControl
            )}
          </div>
          {menus ? (
            <Menubar
              data-slot="os-menubar-menus"
              aria-label="App menus"
              className={cn("gap-1", WINDOW_NO_DRAG)}
            >
              {menus}
            </Menubar>
          ) : null}
        </div>

        <div className={cn("flex items-center gap-2", WINDOW_NO_DRAG)}>
          {/* Outside the menubar's `role="menu"` subtree on purpose: a notice is
              not a menu item, and nesting it there breaks the menu's semantics. */}
          {scopeNotice}
          {status}
          {updateIndicator}
          <Control
            data-slot="os-menubar-bell"
            aria-label={notifications ? `Attention, ${notifications} waiting` : "Attention"}
            aria-haspopup={wrapBellTrigger ? "true" : undefined}
            className="relative grid size-7 place-items-center rounded-md text-muted"
            wrap={wrapBellTrigger}
          >
            <Icon as={Bell} size="lg" />
            {notifications ? <NotificationBadge count={notifications} /> : null}
          </Control>
          <Control
            data-slot="os-menubar-command"
            aria-label="Command palette"
            title={
              commandShortcutLabel ? `Command palette · ${commandShortcutLabel}` : "Command palette"
            }
            className="grid size-7 place-items-center rounded-md text-muted"
            onClick={onCommandClick}
          >
            <Icon as={Command} size="lg" />
          </Control>
          {/* The conventional profile-selector position: last thing before
              Settings, and outside every `role="menubar"` subtree so its popover
              keeps its own semantics. */}
          {profileSwitcher}
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
      </div>
    </header>
  );
}
