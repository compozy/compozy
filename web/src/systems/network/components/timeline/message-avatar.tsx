import { Eyebrow } from "@agh/ui";

import { cn } from "@/lib/utils";

import { getIdentityInitial, pickIdentityPaletteColors } from "../../lib/palette";

export type MessageAvatarOwnerRole = "agent" | "human" | "system";

const ROLE_LABEL: Record<MessageAvatarOwnerRole, string> = {
  agent: "Agent",
  human: "Human",
  system: "System",
};

export interface MessageAvatarProps {
  seed: string;
  initialFrom: string;
  /**
   * 36 in the channel timeline (`_design.md` §5.2.1), 32 inside the thread
   * overlay (§3.2), 20 in the channel rail Direct Rooms section.
   */
  sizePx: 36 | 32 | 26 | 20;
  /**
   * Owner role drives the `role="img"` aria-label. When
   * provided, the avatar announces `{Role} {Name}` so screen readers retain
   * the signal that previously came from the role pill on message rows.
   */
  ownerRole?: MessageAvatarOwnerRole;
  /** Human-readable name for the aria-label (defaults to `initialFrom`). */
  name?: string;
  className?: string;
}

export function MessageAvatar({
  seed,
  initialFrom,
  sizePx,
  ownerRole,
  name,
  className,
}: MessageAvatarProps) {
  const [background, foreground] = pickIdentityPaletteColors(seed);
  const initial = getIdentityInitial(initialFrom);
  const labeled = ownerRole !== undefined;
  const announcedName = name?.trim() || initialFrom;
  const ariaLabel = labeled ? `${ROLE_LABEL[ownerRole]} ${announcedName}` : undefined;

  return (
    <div
      aria-hidden={labeled ? undefined : true}
      aria-label={ariaLabel}
      className={cn(
        "flex shrink-0 select-none items-center justify-center rounded-chip",
        className
      )}
      data-owner-role={ownerRole}
      data-testid="network-message-avatar"
      role={labeled ? "img" : undefined}
      style={{
        backgroundColor: background,
        color: foreground,
        height: sizePx,
        width: sizePx,
      }}
    >
      <Eyebrow aria-hidden="true" className="text-current">
        {initial}
      </Eyebrow>
    </div>
  );
}
