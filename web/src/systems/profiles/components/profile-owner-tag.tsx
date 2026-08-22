import type { ComponentProps } from "react";

import { cn, Pill } from "@compozy/ui";

import { ARCHIVED_OWNER_SUFFIX } from "../lib/profile-copy";
import type { ProfileOwner } from "../lib/profile-scope";
import { ProfileGlyph } from "./profile-glyph";

export interface ProfileOwnerTagProps extends Omit<ComponentProps<typeof Pill>, "children"> {
  owner: ProfileOwner;
  /** Surface the tag sits on, so identity ink is measured against the right plate. */
  surface?: string;
}

/**
 * Names the profile that owns a row.
 *
 * Only aggregate listings wear it — a scoped view already answers "whose work is
 * this", so tagging every row there would be noise. Identity colour stays in the
 * glyph, where it is data; the tag's own tone carries the one piece of state that
 * is a signal: an archived owner reads as info, never as a failure.
 *
 * The glyph is decorative here: the tag's own text names the owner, so labelling
 * the mark as well would announce the profile twice for one tag.
 */
export function ProfileOwnerTag({ owner, surface, className, ...props }: ProfileOwnerTagProps) {
  return (
    <Pill
      data-slot="profile-owner-tag"
      data-testid="profile-owner-tag"
      data-archived={owner.archived ? "true" : undefined}
      tone={owner.archived ? "info" : "neutral"}
      size="xs"
      className={cn("gap-1.5 py-0 pr-2 pl-0.5", className)}
      {...props}
    >
      <ProfileGlyph
        decorative
        name={owner.name}
        color={owner.color}
        icon={owner.icon}
        emoji={owner.emoji}
        size="sm"
        surface={surface}
        className="size-3.5 rounded-[4px]"
      />
      {owner.archived ? `${owner.name}${ARCHIVED_OWNER_SUFFIX}` : owner.name}
    </Pill>
  );
}
