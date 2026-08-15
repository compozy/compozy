import { ScrollArea, cn } from "@compozy/ui";

export interface WorktreeNestProps extends React.ComponentProps<"div"> {
  /** Host height cap for the scroll viewport (token-backed max-h class). */
  viewportClassName: string;
  /** Hairline + create row, pinned below the scroll region — always last. */
  footer?: React.ReactNode;
  listClassName?: string;
}

/**
 * The one nest frame every host mounts: every worktree of the workspace in
 * view stays in the list, and a list taller than the host's cap scrolls
 * inside `ScrollArea` instead of folding rows behind an overflow jump. The
 * create footer stays reachable below the scroll region.
 */
export function WorktreeNest({
  viewportClassName,
  footer,
  className,
  listClassName,
  children,
  ...props
}: WorktreeNestProps) {
  return (
    <div data-slot="worktree-nest" className={cn("flex min-h-0 flex-col", className)} {...props}>
      <ScrollArea className="min-h-0" viewportClassName={viewportClassName}>
        <div className={listClassName}>{children}</div>
      </ScrollArea>
      {footer}
    </div>
  );
}
