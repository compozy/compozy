import { cn, PAGE_CONTENT_GUTTER, SkeletonRegion } from "@compozy/ui";

import { HomeDashboardSkeleton } from "./home-dashboard-skeleton";
import { HomePageMeta } from "./home-page-meta";
import { HOME_BODY_CLASS, HOME_ZONE_STACK_CLASS } from "./home-zone-shape";

/**
 * The Home window before its code or its overview has arrived: the page gutter, the date line and
 * the zones a busy desk shows, in the frames the loaded dashboard fills, under one busy status.
 * It reads no query and no store (the usage tile names the default 30-day window), so a restored
 * desktop of hidden Home windows loads cheaply, and it imports neither the dashboard hooks nor
 * its live zones.
 */
export function DashboardWindowSkeleton() {
  return (
    <div className={cn(PAGE_CONTENT_GUTTER, HOME_BODY_CLASS)} data-testid="home-window-loading">
      <HomePageMeta workspaceName={null} />
      <SkeletonRegion className={HOME_ZONE_STACK_CLASS} label="Loading home dashboard">
        <HomeDashboardSkeleton usageWindow={30} />
      </SkeletonRegion>
    </div>
  );
}
