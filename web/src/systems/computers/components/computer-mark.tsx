import { cn } from "@compozy/ui";

import type { ComputerIconKind, ComputerPlatform } from "../mocks/remote-computers-types";
import { ComputerPlatformIcon } from "./computer-platform-icon";

export interface ComputerMarkProps {
  platform: ComputerPlatform;
  iconKind: ComputerIconKind;
  className?: string;
}

/** Platform icon used in the chevron cell of Together-layer headers and inline tags (`F6`). */
export function ComputerMark({ platform, iconKind, className }: ComputerMarkProps) {
  return (
    <ComputerPlatformIcon
      className={cn("size-3 shrink-0", className)}
      iconKind={iconKind}
      platform={platform}
      size="sm"
      tone="muted"
    />
  );
}

export interface ComputerTagProps {
  name: string;
  platform?: ComputerPlatform;
  iconKind?: ComputerIconKind;
  /** When true, omits the leading platform mark (used when the chevron cell already holds it). */
  bare?: boolean;
  className?: string;
}

/** A computer named after a project: optional platform mark plus truncated name in the meta tone. */
export function ComputerTag({
  name,
  platform,
  iconKind,
  bare = false,
  className,
}: ComputerTagProps) {
  return (
    <span className={cn("flex min-w-0 items-center gap-1 text-eyebrow text-subtle", className)}>
      {!bare && platform && iconKind ? (
        <ComputerMark iconKind={iconKind} platform={platform} />
      ) : null}
      <span className="truncate">{name}</span>
    </span>
  );
}
