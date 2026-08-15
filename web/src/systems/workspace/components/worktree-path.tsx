import { Tooltip, TooltipContent, TooltipTrigger, cn } from "@compozy/ui";

import { contractHomePath } from "../lib/display-path";

export interface WorktreePathProps extends Omit<React.ComponentProps<"span">, "children"> {
  /** Absolute checkout path. Rendered truncated; the tooltip carries the full string. */
  path: string;
  /** Disable the extra tab stop when an interactive ancestor already owns focus. */
  focusable?: boolean;
  /** When set, home-rooted paths display `~`-contracted; the tooltip stays absolute. */
  userHomeDir?: string;
}

/**
 * The worktree checkout path. It is micro mono and demoted, and it must shrink
 * inside compact dialogs and nest rows — a long absolute path never owns layout.
 */
export function WorktreePath({
  path,
  focusable = true,
  userHomeDir,
  className,
  ...props
}: WorktreePathProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span
            data-slot="worktree-path"
            className={cn(
              "block min-w-0 w-full max-w-full truncate font-mono text-micro text-faint",
              focusable && "rounded-xs focus-visible:outline-none focus-visible:shadow-focus-ring",
              className
            )}
            tabIndex={focusable ? 0 : undefined}
            {...props}
          >
            {contractHomePath(path, userHomeDir)}
          </span>
        }
      />
      <TooltipContent className="max-w-xs break-all font-mono">{path}</TooltipContent>
    </Tooltip>
  );
}
