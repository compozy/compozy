import type { StatusPayload } from "@/systems/status";

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

export interface HomeSystemActivity {
  hookRunsToday: number | undefined;
  hookFailuresToday: number | undefined;
  retentionDays: number | undefined;
}

/** A tile plus whether it flags the system as not-normal and its summary phrase. */
interface HomeSystemSection {
  tile: HomeSystemTile;
  warning?: boolean;
  summary?: string;
}

const HEALTHY_PROVIDER_STATES = new Set(["ok", "ready", "authenticated"]);

function formatUptime(uptimeSeconds: number): string {
  const days = Math.floor(uptimeSeconds / 86_400);
  const hours = Math.floor((uptimeSeconds % 86_400) / 3600);
  if (days > 0) {
    return `${days}d ${hours}h`;
  }
  const minutes = Math.floor((uptimeSeconds % 3600) / 60);
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  return `${minutes}m`;
}

function daemonSection(status: StatusPayload): HomeSystemSection {
  const running = `Running for ${formatUptime(status.health.uptime_seconds)}`;
  return {
    tile: { key: "daemon", label: "CompozyOS", value: "Running", detail: running, tone: "success" },
    summary: running,
  };
}

function providersSection(status: StatusPayload): HomeSystemSection | null {
  const providers = status.providers ?? [];
  if (providers.length === 0) return null;
  const healthy = providers.filter(provider => HEALTHY_PROVIDER_STATES.has(provider.state)).length;
  const names = providers
    .map(provider => provider.display_name ?? provider.name)
    .slice(0, 3)
    .join(" · ");
  const allHealthy = healthy === providers.length;
  return {
    tile: {
      key: "providers",
      label: "Providers",
      value: `${healthy} of ${providers.length} ready`,
      detail: names,
      tone: allHealthy ? "success" : "warning",
    },
    warning: !allHealthy,
    summary: `${healthy} of ${providers.length} providers ready`,
  };
}

function formatNextFire(nextFire: string): string {
  const time = new Date(nextFire).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  });
  return `Next run ${time}`;
}

function automationSection(status: StatusPayload): HomeSystemSection {
  const { enabled, next_fire: nextFire } = status.automation;
  return {
    tile: {
      key: "scheduler",
      label: "Automations",
      value: enabled ? "On" : "Off",
      detail: nextFire ? formatNextFire(nextFire) : undefined,
      tone: enabled ? "success" : undefined,
    },
  };
}

function hooksSection(hookRunsToday: number, hookFailuresToday = 0): HomeSystemSection {
  const failed = hookFailuresToday > 0;
  return {
    tile: {
      key: "hooks",
      label: "Hooks",
      value: `${hookRunsToday} runs today`,
      detail: failed ? `${hookFailuresToday} failed` : "None failed",
      tone: failed ? "warning" : "success",
    },
    warning: failed,
  };
}

function retentionSection(retentionDays: number): HomeSystemSection {
  return {
    tile: {
      key: "retention",
      label: "Data kept",
      value: retentionDays === 0 ? "Forever" : `${retentionDays} days`,
      detail: "Activity, usage, and approvals",
    },
  };
}

function statusSections(status: StatusPayload | undefined): (HomeSystemSection | null)[] {
  if (!status) return [];
  return [daemonSection(status), providersSection(status), automationSection(status)];
}

/** Projects daemon status and today's hook/retention facts into the Home system panel model. */
export function buildHomeSystemModel(
  status: StatusPayload | undefined,
  activity: HomeSystemActivity
): HomeSystemModel {
  const { hookRunsToday, hookFailuresToday, retentionDays } = activity;
  const sections = [
    ...statusSections(status),
    hookRunsToday === undefined ? null : hooksSection(hookRunsToday, hookFailuresToday),
    retentionDays === undefined ? null : retentionSection(retentionDays),
  ].filter((section): section is HomeSystemSection => section !== null);

  const warning = sections.some(section => section.warning === true);
  return {
    allNormal: !warning && status !== undefined,
    summary: sections.flatMap(section => (section.summary ? [section.summary] : [])).join(" · "),
    tiles: sections.map(section => section.tile),
  };
}
