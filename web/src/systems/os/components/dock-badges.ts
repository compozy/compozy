import type { OsAttentionBadges } from "../lib/attention-model";
import type { OsAppDefinition } from "../lib/app-registry";

export function dockBadgeFor(
  app: Pick<OsAppDefinition, "badge">,
  badges: OsAttentionBadges
): number | undefined {
  return app.badge ? badges[app.badge] : undefined;
}
