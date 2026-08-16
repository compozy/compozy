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
  const {
    anchor,
    contentRef,
    handleBlur,
    handleContentKeyDown,
    handleTriggerKeyDown,
    openNow,
    scheduleClose,
    triggerRef,
  } = useWorktreeHoverSubmenu({
    open,
    onOpenChange,
    focusOnOpen,
    onReturnFocus,
  });

  return (
    <>
      <div
        ref={triggerRef}
        onMouseEnter={openNow}
        onMouseLeave={scheduleClose}
        onFocusCapture={openNow}
        onBlurCapture={handleBlur}
        onKeyDown={handleTriggerKeyDown}
      >
        {trigger}
      </div>
      <Popover modal={false} open={open} onOpenChange={onOpenChange}>
        <PopoverContent
          ref={contentRef}
          role="dialog"
          aria-label={label}
          data-testid={testId}
          side="right"
          align="start"
          sideOffset={0}
          className={cn(WORKTREE_SUBMENU_FRAME_CLASS, "p-1")}
          anchor={anchor}
          onMouseEnter={openNow}
          onMouseLeave={scheduleClose}
          onFocusCapture={openNow}
          onBlurCapture={handleBlur}
          onKeyDown={handleContentKeyDown}
        >
          {children}
        </PopoverContent>
      </Popover>
    </>
  );
}
