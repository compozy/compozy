import type { ShellSnapshot } from "../../state/app-state";

export type StepId = "download" | "verify" | "install" | "migrate" | "start" | "connect";
export type StepStatus = "done" | "current" | "pending" | "failed";

export interface JourneyStep {
  readonly id: StepId;
  readonly label: string;
  readonly status: StepStatus;
  /** How long a finished step took, or the attempt a failed start reached. */
  readonly meta?: string;
}

export interface JourneyWork {
  readonly completed: number;
  /** 0 while the runtime is still counting the stage's work. */
  readonly total: number;
  readonly unit: "sessions" | "records";
  readonly percent: number | null;
  /** Remaining minutes at the observed rate; null until the rate is known. */
  readonly minutesLeft: number | null;
}

export interface JourneyNotice {
  readonly tone: "info" | "warning";
  readonly title: string;
  readonly text: string;
  /** A warning offers diagnostics as well as logs. */
  readonly diagnostics: boolean;
}

export interface JourneyView {
  readonly layout: "center" | "journey";
  readonly steps: readonly JourneyStep[];
  readonly current: StepId | null;
  readonly work: JourneyWork | null;
  readonly notice: JourneyNotice | null;
  /** Whole seconds since the window started observing this boot. */
  readonly elapsed: number;
  readonly stalled: boolean;
}

const LABELS: Record<StepId, string> = {
  download: "Download",
  verify: "Verify",
  install: "Install",
  migrate: "Upgrade data",
  start: "Start",
  connect: "Connect",
};
/** An app-driven runtime update downloads; the bundled runtime is verified, then installed. */
const UPDATE_FLOW: readonly StepId[] = ["download", "verify", "install", "start", "connect"];
const BOOT_FLOW: readonly StepId[] = ["verify", "install", "migrate", "start", "connect"];
/** Work that has not advanced for this long gets the slow-step notice with logs and diagnostics. */
export const STALL_AFTER_MS = 120_000;
/** A long data upgrade explains itself once it has run this long. */
export const LONG_AFTER_MS = 30_000;
/** The estimate waits for this much observed work so a first burst does not promise too much. */
const ESTIMATE_AFTER_MS = 10_000;

const NUMBER = new Intl.NumberFormat("en-US");

/** `18s`, `4m 07s`, `1h 02m`: compact durations for finished steps. */
export function formatDuration(milliseconds: number): string {
  if (milliseconds < 1000) return "<1s";
  const seconds = Math.round(milliseconds / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ${String(seconds % 60).padStart(2, "0")}s`;
  return `${Math.floor(minutes / 60)}h ${String(minutes % 60).padStart(2, "0")}m`;
}

/** The step a snapshot reports, or null for states that are not part of a journey. */
export function stepOf(snapshot: ShellSnapshot): StepId | null {
  switch (snapshot.state) {
    case "provisioning":
      // A completed provision reports `verify`: the copy is in place and being checked.
      return snapshot.stage === "migrate" ? "migrate" : "install";
    case "updating":
      if (snapshot.stage === "ready") return "connect";
      return snapshot.stage && snapshot.stage !== "migrate" ? snapshot.stage : "download";
    case "starting":
      return snapshot.stage === "ready" ? "connect" : "start";
    case "attaching":
      return "connect";
    default:
      return null;
  }
}

interface Sample {
  readonly completed: number;
  readonly total: number;
  readonly at: number;
}

/**
 * Follows one boot across snapshots: which steps ran, how long each took, whether the counted
 * work still advances and how long it has left. Everything is observed from the snapshot stream,
 * so the window never claims a step or a number the runtime did not report.
 */
export class BootJourney {
  readonly #startedAt: number;
  #flow: "boot" | "update" | null = null;
  #migrated = false;
  #current: StepId | null = null;
  #stepStartedAt: number;
  #lastAdvance: number;
  #first: Sample | null = null;
  #last: Sample | null = null;
  #durations = new Map<StepId, number>();
  #snapshot: ShellSnapshot = { state: "resolving" };

  constructor(now: number) {
    this.#startedAt = now;
    this.#stepStartedAt = now;
    this.#lastAdvance = now;
  }

  observe(snapshot: ShellSnapshot, now: number): JourneyView {
    this.#snapshot = snapshot;
    const step = stepOf(snapshot);
    if (snapshot.state === "updating") this.#flow = "update";
    else if (step === "install" || step === "migrate") this.#flow ??= "boot";
    if (step === "migrate") this.#migrated = true;
    if (step && step !== this.#current) {
      if (this.#current) this.#durations.set(this.#current, now - this.#stepStartedAt);
      this.#current = step;
      this.#stepStartedAt = now;
      this.#lastAdvance = now;
      this.#first = null;
      this.#last = null;
    }
    this.#sample(snapshot, now);
    return this.view(now);
  }

  #sample(snapshot: ShellSnapshot, now: number): void {
    if (snapshot.completed === undefined || snapshot.total === undefined) return;
    const sample = { completed: snapshot.completed, total: snapshot.total, at: now };
    const last = this.#last;
    if (!last || last.completed !== sample.completed || last.total !== sample.total) {
      this.#lastAdvance = now;
      this.#last = sample;
    }
    if (!this.#first || (this.#first.total !== sample.total && sample.total > 0)) {
      this.#first = sample;
    }
  }

  /** The view at `now`; the page calls it every second so clocks and notices stay current. */
  view(now: number): JourneyView {
    const snapshot = this.#snapshot;
    const failed = snapshot.state === "error";
    const running = ["provisioning", "starting", "updating", "attaching"].includes(snapshot.state);
    const layout =
      failed || (this.#flow !== null && running) || snapshot.state === "updating"
        ? "journey"
        : "center";
    const work = running ? this.#work(now) : null;
    const idle = now - this.#lastAdvance;
    const stalled = running && this.#current !== null && idle >= STALL_AFTER_MS;
    return {
      layout,
      steps: this.#flow ? this.#steps(failed) : [],
      current: running || failed ? this.#current : null,
      work,
      notice: running ? this.#notice(work, idle, now) : null,
      elapsed: Math.floor((now - this.#startedAt) / 1000),
      stalled,
    };
  }

  #steps(failed: boolean): JourneyStep[] {
    const flow = (this.#flow === "update" ? UPDATE_FLOW : BOOT_FLOW).filter(
      id => id !== "migrate" || this.#migrated
    );
    const at = this.#current ? flow.indexOf(this.#current) : -1;
    return flow.map((id, index): JourneyStep => {
      let status: StepStatus = "pending";
      if (index < at) status = "done";
      else if (index === at) status = failed ? "failed" : "current";
      const duration = this.#durations.get(id);
      const attempt = this.#snapshot.attempt;
      let meta: string | undefined;
      if (status === "done" && duration !== undefined) meta = formatDuration(duration);
      else if (status === "failed" && id === "start" && attempt) meta = `${attempt} of 5`;
      return meta === undefined
        ? { id, label: LABELS[id], status }
        : { id, label: LABELS[id], status, meta };
    });
  }

  #work(now: number): JourneyWork | null {
    const last = this.#last;
    if (!last) return null;
    const percent =
      last.total > 0 ? Math.min(100, Math.floor((last.completed * 100) / last.total)) : null;
    return {
      completed: last.completed,
      total: last.total,
      unit: this.#snapshot.work ?? "records",
      percent,
      minutesLeft: this.#minutesLeft(last, now),
    };
  }

  #minutesLeft(last: Sample, now: number): number | null {
    const first = this.#first;
    if (!first || last.total <= 0 || now - this.#lastAdvance >= STALL_AFTER_MS) return null;
    const span = last.at - first.at;
    const done = last.completed - first.completed;
    if (span < ESTIMATE_AFTER_MS || done <= 0) return null;
    return (last.total - last.completed) / (done / span) / 60_000;
  }

  #notice(work: JourneyWork | null, idle: number, now: number): JourneyNotice | null {
    if (this.#current === null) return null;
    if (idle >= STALL_AFTER_MS) {
      const minutes = Math.floor(idle / 60_000);
      return {
        tone: "warning",
        title: work
          ? `No progress for ${minutes} minutes`
          : `Still on this step after ${minutes} minutes`,
        text: "CompozyOS keeps trying. The logs show what it is waiting for.",
        diagnostics: true,
      };
    }
    if (this.#current === "migrate" && work && now - this.#stepStartedAt >= LONG_AFTER_MS) {
      return {
        tone: "info",
        title: work.unit === "sessions" ? "You have a lot of history" : "This is a large upgrade",
        text:
          work.unit === "sessions" && work.total > 0
            ? `${NUMBER.format(work.total)} sessions take a while. CompozyOS is still working.`
            : "Large upgrades take a while. CompozyOS is still working.",
        diagnostics: false,
      };
    }
    return null;
  }
}
