import { Avatar, AvatarFallback, cn } from "@compozy/ui";

import type { WorkspaceBadgeIdentity } from "../lib/workspace-badge-identity";

export interface WorkspaceBadgeProps extends React.ComponentProps<"span"> {
  identity: WorkspaceBadgeIdentity;
  /** `meta` sits on a row's line 2; `chrome` leads a window title. */
  variant?: "meta" | "chrome";
}

/**
 * Project badge: the switcher's monogram well plus the name, which truncates
 * first. Rows, group headers, and window chrome under All projects share it.
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
        "inline-flex min-w-0 items-center gap-1",
        variant === "meta" ? "text-micro" : "text-body",
        identity.kind === "unknown" ? "text-faint" : "text-muted",
        className
      )}
      {...props}
    >
      <Avatar size="sm" className={cn("shrink-0", variant === "meta" ? "size-3.5" : "size-4.5")}>
        <AvatarFallback className="text-badge font-semibold">{identity.monogram}</AvatarFallback>
      </Avatar>
      <span className="min-w-0 truncate">{identity.name}</span>
    </span>
  );
}
