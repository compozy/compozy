import type { HomeUsageWindow } from "../types";
import {
  HomeActivityFeedSkeleton,
  HomeAgentsPanelSkeleton,
  HomeKpiStripSkeleton,
  HomeOutcomesChartSkeleton,
  HomePulseHeatmapSkeleton,
  HomeSystemPanelSkeleton,
  HomeUsageChartSkeleton,
  HomeWorkingNowSkeleton,
} from "./home-zone-skeletons";
import { HOME_ZONE_PAIR_CLASS } from "./home-zone-shape";

/**
 * The home overview while its read is out: the zones a busy desk shows, each in its real frame
 * and order (KPI strip, working now, pulse, outcomes | usage, agents | activity, system), with
 * the figures left as blocks. The needs-you zone is left out because it only exists when
 * something waits. Render it inside the stack the loaded zones use, under the one
 * `SkeletonRegion` of `DataSurface.Loading`.
 */
export function HomeDashboardSkeleton({ usageWindow }: { usageWindow: HomeUsageWindow }) {
  return (
    <>
      <HomeKpiStripSkeleton usageWindow={usageWindow} />
      <HomeWorkingNowSkeleton />
      <HomePulseHeatmapSkeleton />
      <div className={HOME_ZONE_PAIR_CLASS}>
        <HomeOutcomesChartSkeleton />
        <HomeUsageChartSkeleton />
      </div>
      <div className={HOME_ZONE_PAIR_CLASS}>
        <HomeAgentsPanelSkeleton />
        <HomeActivityFeedSkeleton />
      </div>
      <HomeSystemPanelSkeleton />
    </>
  );
}
