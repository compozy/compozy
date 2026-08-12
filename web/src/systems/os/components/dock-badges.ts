import type { OsAttentionBadges } from "../lib/attention-model";
import type { OsAppDescriptor } from "../lib/app-catalog";

export function dockBadgeFor(
  app: Pick<OsAppDescriptor, "badge">,
  badges: OsAttentionBadges
): number | undefined {
  return app.badge ? badges[app.badge] : undefined;
}
