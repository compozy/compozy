/** Data for the landing Computers showcase (`remote-computers-09-computers-section`, ADR-014 §2). */

export type ShowcaseTab = "computers" | "run-on" | "phone";

export type ComputerStatus = "connected" | "needs_attention" | "connecting" | "disabled";

export interface LandingComputer {
  readonly id: string;
  readonly name: string;
  readonly platform: "macos" | "linux";
  readonly status: ComputerStatus;
  readonly routeLabel: string;
  readonly routesCount: number;
  readonly cpuPercent: number;
  readonly memPercent: number;
  readonly enabled: boolean;
  readonly attentionHint?: string;
}

export const LANDING_COMPUTERS: readonly LandingComputer[] = [
  {
    id: "cmp_studio",
    name: "studio",
    platform: "macos",
    status: "connected",
    routeLabel: "via LAN · 10.0.0.18",
    routesCount: 2,
    cpuPercent: 18,
    memPercent: 34,
    enabled: true,
  },
  {
    id: "cmp_devbox",
    name: "devbox",
    platform: "linux",
    status: "connected",
    routeLabel: "via SSH · pedro@10.0.0.42",
    routesCount: 3,
    cpuPercent: 46,
    memPercent: 52,
    enabled: true,
  },
  {
    id: "cmp_mini",
    name: "mini-build",
    platform: "macos",
    status: "needs_attention",
    routeLabel: "via hosted route",
    routesCount: 1,
    cpuPercent: 12,
    memPercent: 28,
    enabled: true,
    attentionHint: "Claude signed out on mini-build",
  },
  {
    id: "cmp_hetzner",
    name: "hetzner-eu",
    platform: "linux",
    status: "connecting",
    routeLabel: "via SSH · root@hetzner-eu",
    routesCount: 2,
    cpuPercent: 64,
    memPercent: 71,
    enabled: true,
  },
];

export interface FleetCandidate {
  readonly id: string;
  readonly name: string;
  readonly cpuPercent: number;
  readonly memPercent: number;
  readonly winner?: boolean;
}

export const FLEET_CANDIDATES: readonly FleetCandidate[] = [
  { id: "cmp_studio", name: "studio", cpuPercent: 18, memPercent: 34, winner: true },
  { id: "local", name: "MacBook Pro", cpuPercent: 32, memPercent: 58 },
  { id: "cmp_devbox", name: "devbox", cpuPercent: 46, memPercent: 52 },
];

export const PHONE_HOSTED_URL = "https://studio-4a2b.compozy.run";

export const PHONE_POINTS: readonly string[] = [
  "Installable as a home-screen web app on iOS and Android",
  "Device-scoped permissions and optional first-time approval",
  "Switch between computers with hosted routes from one menu",
];

export interface PhoneSessionRow {
  readonly id: string;
  readonly title: string;
  readonly project: string;
  readonly status: "Needs you" | "Working" | "Done";
  readonly detail: string;
}

export const PHONE_SESSIONS: readonly PhoneSessionRow[] = [
  {
    id: "sess_1",
    title: "Refactor checkout webhook retries",
    project: "api",
    status: "Needs you",
    detail: "Approve command: git push origin fix/retries",
  },
  {
    id: "sess_2",
    title: "Add Postgres index for session replay",
    project: "api",
    status: "Working",
    detail: "Running migration tests · 3m",
  },
  {
    id: "sess_3",
    title: "Audit OpenAPI client error codes",
    project: "web",
    status: "Done",
    detail: "Ready for review · 14m ago",
  },
];
