import type { ComponentProps } from "react";
import { AlertTriangle } from "lucide-react";

import { cn, DataSurface, PAGE_CONTENT_GUTTER } from "@compozy/ui";

import { useHomeDashboard } from "../hooks/use-home-dashboard";
import { HomeActivityFeed } from "./home-activity-feed";
import { HomeAgentsPanel } from "./home-agents-panel";
import { HomeAttentionZone } from "./home-attention-zone";
import { HomeDashboardSkeleton } from "./home-dashboard-skeleton";
import { HomeFirstRun } from "./home-first-run";
import { HomeKpiStrip } from "./home-kpi-strip";
import { HomeOutcomesChart } from "./home-outcomes-chart";
import { HomePageMeta } from "./home-page-meta";
import { HomePulseHeatmap } from "./home-pulse-heatmap";
import { HomeSystemPanel } from "./home-system-panel";
import { HomeUsageChart } from "./home-usage-chart";
import { HomeWorkingNow } from "./home-working-now";
import { HOME_BODY_CLASS, HOME_ZONE_PAIR_CLASS, HOME_ZONE_STACK_CLASS } from "./home-zone-shape";

/**
 * The 7-zone end-user home. Zone order is the design contract: page meta →
 * needs-you (only when something waits) → KPI strip (only once work exists) → working-now → pulse → outcomes | usage →
 * agents | activity → system.
 */
export interface HomeDashboardProps extends ComponentProps<"div"> {
  liveEnabled?: boolean;
}

export function HomeDashboard({ className, liveEnabled = true, ...props }: HomeDashboardProps) {
  const model = useHomeDashboard({ liveEnabled });
  const { workingNow, agents, system } = model;

  const overview = model.overview;
  const overviewState =
    model.overviewStatus === "ready" && overview ? "ready" : model.overviewStatus;
  // The meta line names a project; the first-run title also names User.
  const projectName = model.scope.dataScope.kind === "project" ? model.activeWorkspaceName : null;
  const workspaceName = model.scope.dataScope.kind === "all" ? null : model.activeWorkspaceName;

  return (
    <div
      className={cn(PAGE_CONTENT_GUTTER, HOME_BODY_CLASS, className)}
      data-testid="home-body"
      {...props}
    >
      <HomePageMeta workspaceName={projectName} />
      <DataSurface state={overviewState}>
        <DataSurface.Loading className={HOME_ZONE_STACK_CLASS} label="Loading home dashboard">
          <HomeDashboardSkeleton usageWindow={model.usageWindow} />
        </DataSurface.Loading>
        <DataSurface.Error
          description={model.overviewErrorMessage ?? "Try again in a moment."}
          icon={AlertTriangle}
          title="Unable to load the home overview"
        />
        <DataSurface.Content>
          {!overview ? null : (
            <div className={HOME_ZONE_STACK_CLASS}>
              <HomeAttentionZone
                attention={overview.attention}
                actions={model.attentionActions}
                notificationScope={model.notificationScope}
                scopeLabel={model.activeWorkspaceName ?? "Selected project"}
                bindItemsToOwner={model.scope.dataScope.kind === "all"}
              />
              {model.hasNoWork ? (
                <HomeFirstRun workspaceName={workspaceName} />
              ) : (
                <>
                  <HomeKpiStrip
                    overview={overview}
                    workingNowDetail={workingNowDetail(
                      workingNow.sessionCount,
                      workingNow.runCount
                    )}
                    workingNowLoading={workingNow.status === "loading"}
                    workingNowTotal={workingNow.total}
                  />
                  <HomeWorkingNow
                    cards={workingNow.cards}
                    errorMessage={workingNow.errorMessage}
                    liveEnabled={liveEnabled}
                    status={workingNow.status}
                    total={workingNow.total}
                  />
                  <HomePulseHeatmap pulse={overview.pulse} />
                  <div className={HOME_ZONE_PAIR_CLASS}>
                    <HomeOutcomesChart outcomes={overview.outcomes} />
                    <HomeUsageChart
                      onWindowChange={model.setUsageWindow}
                      profileAggregate={model.profileAggregate}
                      usage={overview.usage}
                      window={model.usageWindow}
                    />
                  </div>
                  <div className={HOME_ZONE_PAIR_CLASS}>
                    <HomeAgentsPanel loading={agents.loading} rows={agents.rows} />
                    <HomeActivityFeed
                      errorMessage={model.activityErrorMessage}
                      events={model.activity ?? []}
                      status={model.activityStatus}
                    />
                  </div>
                  <HomeSystemPanel
                    loading={model.systemLoading}
                    onOpenChange={model.setSystemOpen}
                    open={model.systemOpen}
                    system={system}
                  />
                </>
              )}
            </div>
          )}
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
