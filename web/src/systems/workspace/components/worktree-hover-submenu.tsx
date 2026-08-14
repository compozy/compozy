import { cn, Popover, PopoverContent } from "@compozy/ui";

import { useWorktreeHoverSubmenu } from "../hooks/use-worktree-hover-submenu";
import { WORKTREE_SUBMENU_FRAME_CLASS } from "./worktree-submenu-panel";

export interface WorktreeHoverSubmenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  testId: string;
  label: string;
  trigger: React.ReactNode;
  children: React.ReactNode;
  focusOnOpen?: boolean;
  onReturnFocus?: () => void;
}

export function WorktreeHoverSubmenu({
  open,
  onOpenChange,
  testId,
  label,
  trigger,
  children,
  focusOnOpen = false,
  onReturnFocus,
}: WorktreeHoverSubmenuProps) {
  const submenu = useWorktreeHoverSubmenu({
    open,
    onOpenChange,
    focusOnOpen,
    onReturnFocus,
  });

  return (
    <>
      <div
        ref={submenu.triggerRef}
        onMouseEnter={submenu.openNow}
        onMouseLeave={submenu.scheduleClose}
        onFocusCapture={submenu.openNow}
        onBlurCapture={submenu.handleBlur}
        onKeyDown={submenu.handleTriggerKeyDown}
      >
        {trigger}
      </div>
      <Popover modal={false} open={open} onOpenChange={onOpenChange}>
        <PopoverContent
          ref={submenu.contentRef}
          role="dialog"
          aria-label={label}
          data-testid={testId}
          side="right"
          align="start"
          sideOffset={0}
          className={cn(WORKTREE_SUBMENU_FRAME_CLASS, "p-1")}
          anchor={submenu.anchor}
          onMouseEnter={submenu.openNow}
          onMouseLeave={submenu.scheduleClose}
          onFocusCapture={submenu.openNow}
          onBlurCapture={submenu.handleBlur}
          onKeyDown={submenu.handleContentKeyDown}
        >
          {children}
        </PopoverContent>
      </Popover>
    </>
  );
}
