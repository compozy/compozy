import { cn } from "@compozy/ui";

import type { WorkspaceBadgeIdentity } from "../lib/workspace-badge-identity";

export interface WorkspaceBadgeProps extends React.ComponentProps<"span"> {
  identity: WorkspaceBadgeIdentity;
  /** `meta` sits on a row's line 2; `chrome` leads a window title. */
  variant?: "meta" | "chrome";
}

/**
 * Project badge: the project's name alone, truncating first. Rows, group
 * headers, and window chrome under All projects share it.
 */
export function WorkspaceBadge({
  identity,
  variant = "meta",
  className,
  ...props
}: WorkspaceBadgeProps) {
  return (
    <span
      data-slot="workspace-badge"
      data-kind={identity.kind}
      className={cn(
        "inline-flex min-w-0 items-center",
        variant === "meta" ? "text-micro" : "text-body",
        identity.kind === "unknown" ? "text-faint" : "text-muted-foreground",
        className
      )}
      {...props}
    >
      <span className="min-w-0 truncate">{identity.name}</span>
    </span>
  );
}
