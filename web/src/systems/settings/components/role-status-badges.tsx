import { Pill, type PillTone } from "@agh/ui";

import type { RoleBadge } from "../lib/roles-view-model";

const BADGE_LABELS: Record<RoleBadge, string> = {
  builtin: "BUILTIN",
  inherit: "INHERIT",
};

const BADGE_TONE: Record<RoleBadge, PillTone> = {
  builtin: "neutral",
  inherit: "info",
};

interface RoleStatusBadgesProps {
  badges: readonly RoleBadge[];
  "data-testid"?: string;
}

/** Compact `BUILTIN` / `INHERIT` pills projected from `RoleStatus`. */
export function RoleStatusBadges({ badges, "data-testid": testId }: RoleStatusBadgesProps) {
  if (badges.length === 0) {
    return null;
  }
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-1.5" data-testid={testId}>
      {badges.map(badge => (
        <Pill
          key={badge}
          mono
          size="xs"
          tone={BADGE_TONE[badge]}
          data-badge={badge}
          data-testid={testId ? `${testId}-${badge}` : undefined}
        >
          {BADGE_LABELS[badge]}
        </Pill>
      ))}
    </div>
  );
}
