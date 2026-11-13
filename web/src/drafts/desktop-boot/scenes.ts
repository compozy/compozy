/**
 * Fixtures for the boot window redesign (#1549): one scene per reviewed state. `state` and `stage`
 * are today's boot contract (`desktop/src/boot/boot-contract.ts`); everything else is what the
 * page would derive from the proposed additive fields (`completed`/`total`, versions, elapsed,
 * attempt bound, slow threshold).
 */

export type BootTone = "progress" | "success" | "warning" | "danger";
export type StepStatus = "done" | "current" | "pending" | "failed";
export type BootAction = "retry" | "logs" | "quit" | "diagnose";

export interface BootStep {
  id: string;
  label: string;
  status: StepStatus;
  /** Right-aligned note: how long a finished step took, or the live value of the current one. */
  meta?: string;
}

/** A determinate unit of work the runtime reports while a step runs. */
export interface BootWork {
  completed: number;
  total: number;
  unit: string;
  /** Units per minute; drives the estimate and the live demo tick. */
  rate?: number;
  /** `false` freezes the counter (stalled). */
  moving?: boolean;
}

export interface BootScene {
  state: string;
  stage?: string;
  tone: BootTone;
  /** `center` is the short path (no step list); `journey` shows the steps. */
  layout: "center" | "journey";
  /** Name of the running step, shown above the point field. */
  workName?: string;
  title: string;
  detail?: string;
  versions?: { from?: string; to: string };
  /** Ring: a number is determinate, `null` indeterminate, `"still"` a resting full ring. */
  ring: number | null | "still";
  steps?: BootStep[];
  work?: BootWork;
  hint?: string;
  /** Seconds already elapsed when the scene opens; the footer keeps counting. */
  elapsed?: number;
  activity?: string;
  notice?: { tone: "info" | "warning"; title: string; text: string; actions: BootAction[] };
  error?: { message: string; code: string };
  actions?: BootAction[];
  diagnostics?: { open: boolean; rows: [string, string][] };
  result?: string;
}

const UPDATE = { from: "0.5.4", to: "0.6.0" };

function journey(current: string, meta: Record<string, string> = {}): BootStep[] {
  const order: [string, string][] = [
    ["download", "Download"],
    ["verify", "Verify"],
    ["install", "Install"],
    ["migrate", "Upgrade data"],
    ["start", "Start"],
    ["connect", "Connect"],
  ];
  const index = order.findIndex(([id]) => id === current);
  return order.map(([id, label], position) => ({
    id,
    label,
    status: position < index ? "done" : position === index ? "current" : "pending",
    meta: meta[id],
  }));
}

const DONE: Record<string, string> = { download: "18s", verify: "2s", install: "6s" };

const FAILED_START: BootStep[] = journey("start", { ...DONE, migrate: "34m 02s" }).map(step =>
  step.status === "current" ? { ...step, status: "failed", meta: "5 of 5" } : step
);

const DIAGNOSTICS: [string, string][] = [
  ["Error code", "runtime_start_failed"],
  ["Boot phase", "starting"],
  ["Boot ID", "boot_01JCW8Z4M7Q2"],
  ["App version", "0.6.0"],
  ["Runtime version", "0.6.0"],
  ["Runtime owner", "Managed by CompozyOS"],
];

export const SCENES: Record<string, BootScene> = {
  resolving: {
    state: "resolving",
    tone: "progress",
    layout: "center",
    title: "Looking for CompozyOS",
    ring: null,
    elapsed: 1,
  },
  download: {
    state: "provisioning",
    stage: "download",
    tone: "progress",
    layout: "journey",
    title: "Setting up CompozyOS",
    versions: { to: "0.6.0" },
    detail: "Getting the runtime that runs your agents on this computer.",
    ring: 41,
    steps: journey("download").filter(step => step.id !== "migrate"),
    work: { completed: 38, total: 92, unit: "MB", rate: 190 },
    elapsed: 12,
  },
  verify: {
    state: "provisioning",
    stage: "verify",
    tone: "progress",
    layout: "journey",
    title: "Setting up CompozyOS",
    versions: { to: "0.6.0" },
    detail: "Checking that the download is complete and signed.",
    ring: null,
    steps: journey("verify", DONE).filter(step => step.id !== "migrate"),
    hint: "Checking the signature",
    elapsed: 31,
  },
  install: {
    state: "provisioning",
    stage: "install",
    tone: "progress",
    layout: "journey",
    title: "Setting up CompozyOS",
    versions: { to: "0.6.0" },
    detail: "Putting the runtime in place. Nothing else on your computer changes.",
    ring: null,
    steps: journey("install", DONE).filter(step => step.id !== "migrate"),
    hint: "Copying files",
    elapsed: 34,
  },
  migrate: {
    state: "provisioning",
    stage: "migrate",
    tone: "progress",
    layout: "journey",
    title: "Updating CompozyOS",
    versions: UPDATE,
    detail: "Upgrading your session history for 0.6.0. This runs once.",
    ring: 29,
    steps: journey("migrate", DONE),
    work: { completed: 744, total: 2554, unit: "sessions", rate: 75 },
    elapsed: 621,
    activity: "upgraded sess-4f1c…a92e",
  },
  "migrate-slow": {
    state: "provisioning",
    stage: "migrate",
    tone: "progress",
    layout: "journey",
    title: "Updating CompozyOS",
    versions: UPDATE,
    detail: "Upgrading your session history for 0.6.0. This runs once.",
    ring: 61,
    steps: journey("migrate", DONE),
    work: { completed: 1561, total: 2554, unit: "sessions", rate: 75 },
    elapsed: 1275,
    notice: {
      tone: "info",
      title: "You have a lot of history",
      text: "2,554 sessions take a while. CompozyOS is still working.",
      actions: ["logs"],
    },
  },
  stalled: {
    state: "provisioning",
    stage: "migrate",
    tone: "warning",
    layout: "journey",
    title: "Updating CompozyOS",
    versions: UPDATE,
    detail: "Upgrading your session history for 0.6.0. This runs once.",
    ring: 61,
    steps: journey("migrate", DONE),
    work: { completed: 1561, total: 2554, unit: "sessions", moving: false },
    elapsed: 1490,
    notice: {
      tone: "warning",
      title: "No progress for 2 minutes",
      text: "CompozyOS keeps trying. The logs show what it is waiting for.",
      actions: ["logs", "diagnose"],
    },
  },
  starting: {
    state: "starting",
    stage: "start",
    tone: "progress",
    layout: "journey",
    title: "Updating CompozyOS",
    versions: UPDATE,
    detail: "Starting the runtime with your upgraded data.",
    ring: null,
    steps: journey("start", { ...DONE, migrate: "34m 02s" }),
    hint: "Opening your workspaces",
    elapsed: 2074,
  },
  "starting-retry": {
    state: "starting",
    stage: "start",
    tone: "progress",
    layout: "center",
    title: "Starting CompozyOS",
    detail: "The runtime did not answer the first time. Trying again.",
    ring: null,
    elapsed: 9,
    activity: "attempt 2 of 5",
  },
  "starting-waiting": {
    state: "starting",
    stage: "start",
    tone: "progress",
    layout: "center",
    title: "Starting CompozyOS",
    detail:
      "Another update or restart is in progress (process 4242). CompozyOS continues when it finishes.",
    ring: null,
    elapsed: 38,
  },
  "starting-resuming": {
    state: "starting",
    stage: "start",
    tone: "progress",
    layout: "center",
    title: "Starting CompozyOS",
    detail: "Resuming interrupted sessions.",
    ring: null,
    elapsed: 12,
  },
  "error-update-running": {
    state: "error",
    tone: "danger",
    layout: "center",
    title: "CompozyOS could not start",
    ring: "still",
    error: {
      message:
        "Another CompozyOS update or restart is still running. Wait for it to finish, then try again.",
      code: "update_lock_held",
    },
    actions: ["retry", "logs", "quit"],
  },
  attaching: {
    state: "attaching",
    tone: "progress",
    layout: "center",
    title: "Connecting to CompozyOS",
    ring: null,
    elapsed: 2,
  },
  ready: {
    state: "product",
    stage: "ready",
    tone: "success",
    layout: "center",
    title: "CompozyOS is ready",
    versions: UPDATE,
    ring: "still",
  },
  updating: {
    state: "updating",
    stage: "download",
    tone: "progress",
    layout: "journey",
    title: "Updating CompozyOS",
    versions: { from: "0.6.0", to: "0.6.1" },
    detail: "Your sessions pause for a moment and continue after the update.",
    ring: 72,
    steps: journey("download").filter(step => step.id !== "migrate"),
    work: { completed: 66, total: 92, unit: "MB", rate: 190 },
    elapsed: 21,
  },
  disconnected: {
    state: "disconnected",
    tone: "warning",
    layout: "center",
    title: "CompozyOS is unavailable",
    detail: "The runtime stopped answering. Reconnecting.",
    ring: null,
    elapsed: 14,
    actions: ["logs", "quit"],
  },
  skew: {
    state: "skew",
    tone: "warning",
    layout: "center",
    title: "CompozyOS needs an update",
    versions: { from: "0.5.4", to: "0.6.0" },
    detail: "This app is older than the runtime on this computer. Update the app to reconnect.",
    ring: "still",
    actions: ["logs", "quit"],
  },
  error: {
    state: "error",
    tone: "danger",
    layout: "journey",
    title: "CompozyOS could not start",
    ring: "still",
    error: {
      message: "The runtime stopped while starting. Your sessions and data are safe.",
      code: "runtime_start_failed · attempt 5 of 5",
    },
    actions: ["retry", "logs", "quit"],
    steps: FAILED_START,
    diagnostics: { open: false, rows: DIAGNOSTICS },
  },
  "error-details": {
    state: "error",
    tone: "danger",
    layout: "journey",
    title: "CompozyOS could not start",
    ring: "still",
    error: {
      message: "The runtime stopped while starting. Your sessions and data are safe.",
      code: "runtime_start_failed · attempt 5 of 5",
    },
    actions: ["retry", "logs", "quit"],
    steps: FAILED_START,
    diagnostics: { open: true, rows: DIAGNOSTICS },
    result: "Diagnostics copied.",
  },
};
