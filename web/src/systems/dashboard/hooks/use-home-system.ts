import { useQuery } from "@tanstack/react-query";

import { statusOptions } from "@/systems/status";

export interface HomeSystemTile {
  key: string;
  label: string;
  value: string;
  detail?: string;
  tone?: "success" | "warning" | "danger";
}

export interface HomeSystemModel {
  allNormal: boolean;
  summary: string;
  tiles: HomeSystemTile[];
}

function formatUptime(uptimeSeconds: number): string {
  const days = Math.floor(uptimeSeconds / 86_400);
  const hours = Math.floor((uptimeSeconds % 86_400) / 3600);
  if (days > 0) {
    return `up ${days}d ${hours}h`;
  }
  const minutes = Math.floor((uptimeSeconds % 3600) / 60);
  if (hours > 0) {
    return `up ${hours}h ${minutes}m`;
  }
  return `up ${minutes}m`;
}

const HEALTHY_PROVIDER_STATES = new Set(["ok", "ready", "authenticated"]);

export function useHomeSystem(
  hookRunsToday: number | undefined,
  hookFailuresToday: number | undefined,
  retentionDays: number | undefined
): HomeSystemModel {
  const statusQuery = useQuery(statusOptions());
  const status = statusQuery.data;

  const tiles: HomeSystemTile[] = [];
  const summaryParts: string[] = [];
  let warning = false;

  if (status) {
    const uptime = formatUptime(status.health.uptime_seconds);
    tiles.push({
      key: "daemon",
      label: "Daemon",
      value: "Running",
      detail: `v${status.daemon.version} · ${uptime}`,
      tone: "success",
    });
    summaryParts.push(`v${status.daemon.version}`, uptime);

    const providers = status.providers ?? [];
    if (providers.length > 0) {
      const healthy = providers.filter(provider =>
        HEALTHY_PROVIDER_STATES.has(provider.state)
      ).length;
      const names = providers
        .map(provider => provider.display_name ?? provider.name)
        .slice(0, 3)
        .join(" · ");
      const allHealthy = healthy === providers.length;
      warning = warning || !allHealthy;
      tiles.push({
        key: "providers",
        label: "Providers",
        value: `${healthy} of ${providers.length} ready`,
        detail: names,
        tone: allHealthy ? "success" : "warning",
      });
      summaryParts.push(`providers ${healthy}/${providers.length}`);
    }

    const nextFire = status.automation.next_fire;
    tiles.push({
      key: "scheduler",
      label: "Scheduler",
      value: status.automation.enabled ? "Running" : "Off",
      detail: nextFire
        ? `next wake ${new Date(nextFire).toLocaleTimeString(undefined, {
            hour: "2-digit",
            minute: "2-digit",
          })}`
        : undefined,
      tone: status.automation.enabled ? "success" : undefined,
    });
    summaryParts.push(`scheduler ${status.automation.enabled ? "running" : "off"}`);

    tiles.push({
      key: "memory",
      label: "Memory",
      value: status.memory.enabled ? "Enabled" : "Off",
      detail: status.memory.dream_enabled ? "dream consolidation on" : undefined,
      tone: status.memory.enabled ? "success" : undefined,
    });
  }

  if (hookRunsToday !== undefined) {
    const failures = hookFailuresToday ?? 0;
    warning = warning || failures > 0;
    tiles.push({
      key: "hooks",
      label: "Hooks",
      value: `${hookRunsToday} runs today`,
      detail: `${failures} failed`,
      tone: failures > 0 ? "warning" : "success",
    });
  }
  if (retentionDays !== undefined) {
    tiles.push({
      key: "retention",
      label: "Retention",
      value: retentionDays === 0 ? "Keep forever" : `${retentionDays} days`,
      detail: "events · usage · permissions",
    });
  }

  return {
    allNormal: !warning && status !== undefined,
    summary: summaryParts.join(" · "),
    tiles,
  };
}
