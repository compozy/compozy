import { Eyebrow } from "@compozy/ui";

import { ownerFromRow, ProfileOwnerTag } from "@/systems/profiles";

import { formatHomeTokens } from "../lib/home-formatters";
import type { HomeOverview } from "../types";

export interface HomeProfileShareProps {
  profiles: HomeOverview["usage"]["profiles"];
  windowDays: number;
}

/**
 * Per-profile token breakdown on the machine-wide dashboard.
 *
 * This surface is the one that adopts the labeled aggregate read: scoped views
 * inherit their scoping and render nothing new. Archived owners stay in the
 * list — history that stopped being current did not stop being true — and work
 * recorded before profiles existed belongs to `default`, which is where the
 * daemon already files it.
 *
 * The bars are magnitude in neutral viz ink. The signal palette carries meaning
 * elsewhere and never enters data, and the profile's own colour lives in its
 * glyph, where it is identity rather than a scale.
 */
export function HomeProfileShare({ profiles, windowDays }: HomeProfileShareProps) {
  if (profiles.length === 0) return null;
  const total = profiles.reduce((sum, entry) => sum + entry.tokens, 0);
  return (
    <div
      className="-mx-5 mt-auto border-t border-line-soft px-5 pt-3.5"
      data-slot="home-profile-share"
      data-testid="home-profile-share"
    >
      <div className="mb-2.5 flex items-center justify-between gap-3">
        <Eyebrow className="text-subtle">Per-profile share</Eyebrow>
        <span className="text-micro text-faint">all profiles · last {windowDays} days</span>
      </div>
      <div className="flex flex-col gap-2">
        {profiles.map(entry => (
          <div className="flex flex-col gap-1" key={entry.profile_id}>
            <div className="flex items-center justify-between gap-3">
              <ProfileOwnerTag owner={ownerFromRow(entry)} />
              <span className="font-mono text-micro tabular-nums text-subtle">
                {formatHomeTokens(entry.tokens)}
              </span>
            </div>
            <span
              aria-hidden="true"
              className="h-1.5 overflow-hidden rounded-xs bg-badge-fill"
              data-slot="home-profile-share-meter"
            >
              <span
                className="block h-full"
                style={{
                  background: "var(--color-viz-bar)",
                  width: `${total > 0 && entry.tokens > 0 ? (entry.tokens / total) * 100 : 0}%`,
                }}
              />
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
