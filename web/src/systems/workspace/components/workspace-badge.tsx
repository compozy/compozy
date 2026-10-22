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
      {/* The Avatar's own `sm` step (24px) outranks a size class, so the badge sizes the default
          Avatar to sit on the meta line beside the 14px profile glyph and provider mark. */}
      <Avatar className={cn("shrink-0", variant === "meta" ? "size-4" : "size-5")}>
        <AvatarFallback
          className={cn(
            "font-semibold",
            variant === "meta" ? "text-pill-group-badge" : "text-badge"
          )}
        >
          {identity.monogram}
        </AvatarFallback>
      </Avatar>
      <span className="min-w-0 truncate">{identity.name}</span>
    </span>
  );
}
