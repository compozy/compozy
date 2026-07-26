import type { ComponentProps } from "react";
import { AlertTriangle } from "lucide-react";

import { cn, DataSurface, Skeleton } from "@agh/ui";

import { useHomeDashboard } from "../hooks/use-home-dashboard";
import { HomeActivityFeed } from "./home-activity-feed";
import { HomeAgentsPanel } from "./home-agents-panel";
import { HomeAttentionZone } from "./home-attention-zone";
import { HomeKpiStrip } from "./home-kpi-strip";
import { HomeNetworkPanel } from "./home-network-panel";
import { HomeOutcomesChart } from "./home-outcomes-chart";
import { HomePageMeta } from "./home-page-meta";
import { HomePulseHeatmap } from "./home-pulse-heatmap";
import { HomeSystemPanel } from "./home-system-panel";
import { HomeUsageChart } from "./home-usage-chart";
import { HomeWorkingNow } from "./home-working-now";

function HomeDashboardSkeleton() {
  return (
    <div aria-label="Loading home dashboard" className="flex flex-col gap-6" role="status">
      <Skeleton className="h-24 rounded-lg" />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {[0, 1, 2, 3].map(index => (
          <Skeleton className="h-24 rounded-lg" key={index} />
        ))}
      </div>
      <div className="grid grid-cols-1 gap-5 min-[1080px]:grid-cols-2">
        <Skeleton className="h-48 rounded-lg" />
        <Skeleton className="h-48 rounded-lg" />
      </div>
    </div>
  );
}

/**
 * The 7-zone end-user home. Zone order is the design contract: page meta →
 * needs-you → KPI strip → working-now | network → pulse → outcomes | usage →
 * agents | activity → system.
 */
export function HomeDashboard({ className, ...props }: ComponentProps<"div">) {
  const model = useHomeDashboard();
  const { workingNow, network, agents, system } = model;

  const overview = model.overview;
  const overviewState =
    model.overviewStatus === "ready" && overview ? "ready" : model.overviewStatus;

  return (
    <div
      className={cn("mx-auto w-full max-w-[1240px] px-9 pt-6 pb-20", className)}
      data-testid="home-body"
      {...props}
    >
      <HomePageMeta workspaceName={model.scope.workspaceParam ? model.activeWorkspaceName : null} />
      <DataSurface state={overviewState}>
        <div data-surface-state="loading">
          <HomeDashboardSkeleton />
        </div>
        <DataSurface.Error
          description={model.overviewErrorMessage ?? "The daemon did not return the overview."}
          icon={AlertTriangle}
          title="Unable to load the home overview"
        />
        <DataSurface.Content>
          {overview ? (
            <div className="flex flex-col gap-6">
              <HomeAttentionZone attention={overview.attention} />
              <HomeKpiStrip
                overview={overview}
                workingNowDetail={workingNowDetail(workingNow.sessionCount, workingNow.runCount)}
                workingNowTotal={workingNow.total}
              />
              <div className="grid grid-cols-1 items-stretch gap-5 min-[1080px]:grid-cols-2">
                <HomeWorkingNow
                  cards={workingNow.cards}
                  errorMessage={workingNow.errorMessage}
                  status={workingNow.status}
                  total={workingNow.total}
                />
                <HomeNetworkPanel rows={network.rows} />
              </div>
              <HomePulseHeatmap pulse={overview.pulse} />
              <div className="grid grid-cols-1 items-stretch gap-5 min-[1080px]:grid-cols-2">
                <HomeOutcomesChart outcomes={overview.outcomes} />
                <HomeUsageChart
                  onWindowChange={model.setUsageWindow}
                  usage={overview.usage}
                  window={model.usageWindow}
                />
              </div>
              <div className="grid grid-cols-1 items-stretch gap-5 min-[1080px]:grid-cols-2">
                <HomeAgentsPanel rows={agents.rows} />
                <HomeActivityFeed
                  errorMessage={model.activityErrorMessage}
                  events={model.activity ?? []}
                  status={model.activityStatus}
                />
              </div>
              <HomeSystemPanel
                onOpenChange={model.setSystemOpen}
                open={model.systemOpen}
                system={system}
              />
            </div>
          ) : null}
        </DataSurface.Content>
      </DataSurface>
    </div>
  );
}

function workingNowDetail(sessions: number, runs: number): string {
  if (sessions + runs === 0) {
    return "no live work";
  }
  const sessionLabel = `${sessions} session${sessions === 1 ? "" : "s"}`;
  const runLabel = `${runs} task run${runs === 1 ? "" : "s"}`;
  if (runs === 0) {
    return sessionLabel;
  }
  if (sessions === 0) {
    return runLabel;
  }
  return `${sessionLabel} · ${runLabel}`;
}
