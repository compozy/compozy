import type { ComponentProps } from "react";

import { ActionResultBanner, Button, cn } from "@compozy/ui";

import { ownedByProfileLine, switchToProfileLabel } from "../lib/profile-copy";
import type { ProfileOwner } from "../lib/profile-scope";
import { ProfileGlyph } from "./profile-glyph";

export interface ProfileOwnerBannerProps extends Omit<
  ComponentProps<typeof ActionResultBanner>,
  "title" | "actions" | "icon"
> {
  owner: ProfileOwner;
  /** What the operator opened, in the register of the surface: "session", "task". */
  noun: string;
  onSwitch: () => void;
  switchPending?: boolean;
}

/**
 * Explains why a deep-linked item is not in the current context, and offers the
 * one move that fixes it.
 *
 * The item itself still renders — it is read through the labeled aggregate get,
 * so nothing is being hidden or reconstructed client-side. The tone is info
 * rather than warning on purpose: nothing has gone wrong, this simply is not the
 * profile the operator is working in.
 */
export function ProfileOwnerBanner({
  owner,
  noun,
  onSwitch,
  switchPending = false,
  className,
  ...props
}: ProfileOwnerBannerProps) {
  return (
    <ActionResultBanner
      data-testid="profile-owner-banner"
      tone="info"
      className={cn("items-center", className)}
      title={
        <span className="flex items-center gap-2">
          <ProfileGlyph
            decorative
            name={owner.name}
            color={owner.color}
            icon={owner.icon}
            emoji={owner.emoji}
            size="sm"
          />
          {ownedByProfileLine(noun, owner.name)}
        </span>
      }
      actions={
        <Button
          data-testid="profile-owner-banner-switch"
          size="sm"
          variant="outline"
          disabled={switchPending}
          onClick={onSwitch}
        >
          {switchToProfileLabel(owner.name)}
        </Button>
      }
      {...props}
    />
  );
}
