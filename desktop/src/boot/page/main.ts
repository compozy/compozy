// The boot window's page script, bundled by scripts/build-main.ts into pages/boot.js. It renders the
// shell snapshots main publishes over the preload bridge; it has no other data source.
import type { BootMethod } from "../boot-contract";
import type { ShellSnapshot, ShellStateName } from "../../state/app-state";
import { publicSafeText } from "../../state/public-safe-text";
import { clock, detailFor, estimateFor, statusWordFor, titleFor, workNameFor } from "./copy";
import { BootJourney, type JourneyView } from "./journey";
import { PointField } from "./points";
import { BootRunner } from "./runner";

interface BootBridge {
  onState(listener: (value: unknown) => void): void;
  invoke(method: BootMethod, params?: Record<string, unknown>): Promise<unknown>;
}

declare global {
  interface Window {
    readonly compozyBoot: BootBridge;
  }
}

const NUMBER = new Intl.NumberFormat("en-US");
const STATES = new Set<ShellStateName>([
  "resolving",
  "provisioning",
  "starting",
  "attaching",
  "product",
  "updating",
  "disconnected",
  "skew",
  "error",
]);
/** States whose window is waiting on work: the mark pulses and the clock counts. */
const ALIVE = new Set<ShellStateName>([
  "resolving",
  "provisioning",
  "starting",
  "attaching",
  "updating",
  "disconnected",
]);

function byId<T extends HTMLElement = HTMLElement>(id: string): T {
  const value = document.getElementById(id);
  if (!value) throw new Error(`Missing boot element ${id}.`);
  return value as T;
}

const root = byId("boot");
const title = byId("boot-title");
const versions = byId("boot-versions");
const versionFrom = byId("boot-version-from");
const versionArrow = byId("boot-version-arrow");
const versionTo = byId("boot-version-to");
const releaseNotes = byId<HTMLButtonElement>("boot-release-notes");
const detail = byId("boot-detail");
const errorCode = byId("boot-error-code");
const controls = byId("boot-controls");
const retry = byId<HTMLButtonElement>("boot-retry");
const openLogs = byId<HTMLButtonElement>("boot-open-logs");
const quit = byId<HTMLButtonElement>("boot-quit");
const runnerHost = byId("boot-runner");
const statusWord = byId("boot-status-word");
const elapsed = byId("boot-elapsed");
const activity = byId("boot-activity");
const panel = byId("boot-panel");
const steps = byId("boot-steps");
const work = byId("boot-work");
const workName = byId("boot-work-name");
const workCount = byId("boot-work-count");
const progress = byId("boot-progress");
const workHint = byId("boot-work-hint");
const workPercent = byId("boot-work-percent");
const notice = byId("boot-notice");
const noticeTitle = byId("boot-notice-title");
const noticeText = byId("boot-notice-text");
const noticeLogs = byId<HTMLButtonElement>("boot-notice-logs");
const noticeDiagnose = byId<HTMLButtonElement>("boot-notice-diagnose");
const technical = byId("boot-technical");
const diagnostics = byId("boot-diagnostics");
const diagnose = byId<HTMLButtonElement>("boot-diagnose");
const copy = byId<HTMLButtonElement>("boot-copy");
const exportButton = byId<HTMLButtonElement>("boot-export");
const result = byId("boot-result");

// The center pulse never knows a total: its band of light keeps crossing.
new PointField(byId("boot-pulse"), 24, 1).paint(null);
const field = new PointField(byId("boot-field"), 52, 18);
const journey = new BootJourney(performance.now());
let snapshot: ShellSnapshot = { state: "resolving" };
let runner: BootRunner | null = null;
let runnerAutoplay = false;
let diagnosticsLoaded = false;
/** A slow step's Diagnostics button keeps the report open across later snapshots. */
let diagnosticsRequested = false;

function setResult(message: string, level: "status" | "success" | "error" = "status"): void {
  const text = publicSafeText(message, "");
  result.textContent = text;
  result.hidden = text.length === 0;
  result.dataset.level = text ? level : "";
  result.setAttribute("role", level === "error" ? "alert" : "status");
  result.setAttribute("aria-live", level === "error" ? "assertive" : "polite");
}

function show(node: HTMLElement, visible: boolean): void {
  node.hidden = !visible;
}

function addDiagnostic(label: string, value: unknown): void {
  const text = publicSafeText(value, "");
  if (!text) return;
  const row = document.createElement("div");
  const term = document.createElement("dt");
  const description = document.createElement("dd");
  term.textContent = label;
  description.textContent = text;
  row.append(term, description);
  diagnostics.append(row);
}

/** Before diagnostics load, technical details show what the snapshot already knows. */
function renderSnapshotDiagnostics(): void {
  if (diagnosticsLoaded) return;
  diagnostics.replaceChildren();
  addDiagnostic("Error code", snapshot.error?.code);
}

interface DiagnosticReportView {
  readonly schema_version?: unknown;
  readonly boot_id?: unknown;
  readonly boot_phase?: unknown;
  readonly app_version?: unknown;
  readonly runtime_version?: unknown;
  readonly runtime_owned?: unknown;
  readonly current_error?: { readonly code?: unknown; readonly safe_message?: unknown } | null;
  readonly previous_crash?: {
    readonly boot_phase?: unknown;
    readonly boot_id?: unknown;
    readonly started_at?: unknown;
  } | null;
}

function showDiagnostics(report: unknown): void {
  const value = report as DiagnosticReportView | null;
  if (!value || typeof value !== "object" || value.schema_version !== 1) {
    throw new Error("Diagnostics could not be loaded.");
  }
  diagnosticsLoaded = true;
  diagnostics.replaceChildren();
  addDiagnostic("Error code", value.current_error?.code);
  addDiagnostic("Current error", value.current_error?.safe_message);
  addDiagnostic("Boot phase", value.boot_phase);
  addDiagnostic("Boot ID", value.boot_id);
  addDiagnostic("App version", value.app_version);
  addDiagnostic("Runtime version", value.runtime_version);
  addDiagnostic(
    "Runtime owner",
    value.runtime_owned === true
      ? "Managed by CompozyOS"
      : value.runtime_owned === false
        ? "External"
        : ""
  );
  addDiagnostic("Previous crash phase", value.previous_crash?.boot_phase);
  addDiagnostic("Previous crash boot ID", value.previous_crash?.boot_id);
  addDiagnostic("Previous crash started", value.previous_crash?.started_at);
  show(technical, true);
  show(diagnose, false);
  show(copy, true);
  show(exportButton, true);
}

async function invoke(
  button: HTMLButtonElement,
  method: BootMethod,
  params: Record<string, unknown>,
  pending: string,
  onSuccess: (response: unknown) => void
): Promise<void> {
  const label = button.textContent;
  button.disabled = true;
  button.setAttribute("aria-busy", "true");
  button.textContent = pending;
  setResult("");
  try {
    onSuccess(await window.compozyBoot.invoke(method, params));
  } catch (error) {
    setResult(
      error instanceof Error ? error.message : "The requested action could not be completed.",
      "error"
    );
  } finally {
    button.disabled = false;
    button.removeAttribute("aria-busy");
    button.textContent = label;
  }
}

const onLogs = (button: HTMLButtonElement) => () =>
  void invoke(button, "open_logs", {}, "Opening…", () => setResult("Logs opened.", "success"));
const onDiagnose = (button: HTMLButtonElement) => () =>
  void invoke(button, "diagnose", {}, "Loading…", response => {
    showDiagnostics(response);
    if (snapshot.state !== "error") {
      diagnosticsRequested = true;
      showDiagnosticsPanel();
    }
  });

retry.addEventListener(
  "click",
  () => void invoke(retry, "retry", {}, "Retrying…", () => setResult("Retry started.", "success"))
);
openLogs.addEventListener("click", onLogs(openLogs));
noticeLogs.addEventListener("click", onLogs(noticeLogs));
noticeDiagnose.addEventListener("click", onDiagnose(noticeDiagnose));
diagnose.addEventListener("click", onDiagnose(diagnose));
quit.addEventListener("click", () => void window.compozyBoot.invoke("quit", {}));
releaseNotes.addEventListener(
  "click",
  () => void invoke(releaseNotes, "open_release_notes", {}, "Opening…", () => setResult(""))
);
copy.addEventListener(
  "click",
  () =>
    void invoke(copy, "copy_diagnostics", {}, "Copying…", () =>
      setResult("Diagnostics copied.", "success")
    )
);
exportButton.addEventListener(
  "click",
  () =>
    void invoke(exportButton, "export_diagnostics", { consent: true }, "Exporting…", response => {
      const value = response as { bundle_path?: unknown } | null;
      if (!value || typeof value.bundle_path !== "string") {
        throw new Error("Diagnostics could not be exported.");
      }
      setResult("Diagnostics export created.", "success");
    })
);

/** A slow step's Diagnostics button reveals the loaded report in place of the work. */
function showDiagnosticsPanel(): void {
  show(work, false);
  show(technical, true);
}

function renderVersions(): void {
  const to = publicSafeText(snapshot.to_version, "");
  show(versions, to !== "" && snapshot.state !== "error");
  const from = publicSafeText(snapshot.from_version, "");
  versionFrom.textContent = from;
  show(versionFrom, from !== "");
  show(versionArrow, from !== "");
  versionTo.textContent = to;
}

function renderSteps(view: JourneyView): void {
  show(steps, view.steps.length > 0);
  steps.replaceChildren(
    ...view.steps.map(step => {
      const item = document.createElement("li");
      item.className = "boot__step";
      item.dataset.status = step.status;
      if (step.status === "current") item.setAttribute("aria-current", "step");
      const label = document.createElement("span");
      label.textContent = step.label;
      const meta = document.createElement("span");
      meta.className = "boot__step-meta";
      meta.textContent = step.meta ?? "";
      label.append(meta);
      item.append(label);
      return item;
    })
  );
}

function renderWork(view: JourneyView): void {
  const running = view.layout === "journey" && snapshot.state !== "error" && view.current;
  show(work, Boolean(running) && technical.hidden === true);
  if (!running || !view.current) return;
  const counted = view.work;
  const unit = counted?.unit === "sessions" ? "sessions" : "records";
  workName.textContent = workNameFor(view.current, snapshot.work);
  show(workCount, counted !== null && counted.total > 0);
  if (counted && counted.total > 0) {
    const done = document.createElement("b");
    done.textContent = NUMBER.format(counted.completed);
    workCount.replaceChildren(done, ` of ${NUMBER.format(counted.total)} ${unit}`);
  }
  const fraction = counted && counted.total > 0 ? counted.completed / counted.total : null;
  field.paint(fraction);
  if (counted?.percent === null || counted === null) {
    progress.removeAttribute("aria-valuenow");
    progress.setAttribute("aria-valuetext", workName.textContent);
  } else {
    progress.setAttribute("aria-valuenow", String(counted.percent));
    progress.setAttribute(
      "aria-valuetext",
      `${NUMBER.format(counted.completed)} of ${NUMBER.format(counted.total)} ${unit}`
    );
  }
  workHint.textContent = counted
    ? estimateFor(counted.minutesLeft, counted.total, view.stalled)
    : view.stalled
      ? "Waiting for the runtime"
      : "";
  show(workPercent, counted?.percent !== null && counted !== null);
  workPercent.textContent = counted?.percent === null || !counted ? "" : `${counted.percent}%`;
  show(notice, view.notice !== null);
  if (view.notice) {
    notice.dataset.tone = view.notice.tone;
    noticeTitle.textContent = view.notice.title;
    noticeText.textContent = view.notice.text;
    show(noticeDiagnose, view.notice.diagnostics);
  }
}

/** The game plays itself while work runs; after a failure it waits at rest for Space. */
function renderRunner(view: JourneyView): void {
  const wanted = view.layout === "journey";
  const autoplay = snapshot.state !== "error";
  show(runnerHost, wanted);
  if (runner && (!wanted || runnerAutoplay !== autoplay)) {
    runner.stop();
    runner = null;
  }
  if (wanted && !runner) {
    runner = new BootRunner(runnerHost, { autoplay });
    runnerAutoplay = autoplay;
    runner.start();
  }
}

/** Repaints everything the clock can change; snapshots and the one-second tick both call it. */
function paint(view: JourneyView): void {
  const state = snapshot.state;
  const failed = state === "error";
  const alive = ALIVE.has(state);
  root.dataset.layout = view.layout;
  root.dataset.state = state;
  root.dataset.alive = String(alive);
  root.dataset.tone = failed
    ? "danger"
    : state === "product"
      ? "success"
      : state === "skew" || state === "disconnected" || view.stalled
        ? "warning"
        : "progress";
  const heading = titleFor(snapshot, view);
  if (title.textContent !== heading) title.textContent = heading;
  title.setAttribute("aria-live", failed ? "assertive" : "polite");
  const reason = detailFor(snapshot, view);
  detail.textContent = reason ?? "";
  show(detail, reason !== null);
  detail.setAttribute("role", failed ? "alert" : "status");
  const attempt = snapshot.attempt;
  errorCode.textContent = failed
    ? [snapshot.error?.code, attempt ? `attempt ${attempt} of 5` : ""].filter(Boolean).join(" · ")
    : "";
  show(errorCode, failed && errorCode.textContent !== "");
  show(byId("boot-pulse"), view.layout === "center" && alive);
  statusWord.textContent = statusWordFor(snapshot, view);
  show(elapsed, alive);
  elapsed.textContent = clock(view.elapsed);
  elapsed.setAttribute("aria-label", `Elapsed time ${clock(view.elapsed)}`);
  const retrying = state === "starting" && attempt !== undefined && attempt > 1;
  activity.textContent = retrying ? `attempt ${attempt} of 5` : "";
  show(activity, retrying);
  show(panel, view.layout === "journey");
  renderSteps(view);
  renderWork(view);
  renderRunner(view);
}

function render(payload: unknown): void {
  const input = payload && typeof payload === "object" ? (payload as ShellSnapshot) : null;
  snapshot = input && STATES.has(input.state) ? input : { ...input, state: "error" };
  const failed = snapshot.state === "error";
  show(retry, failed);
  show(openLogs, failed || snapshot.state === "skew" || snapshot.state === "disconnected");
  show(quit, failed || snapshot.state === "skew" || snapshot.state === "disconnected");
  show(controls, retry.hidden !== true || openLogs.hidden !== true);
  show(technical, failed || diagnosticsRequested);
  show(diagnose, !diagnosticsLoaded);
  show(copy, diagnosticsLoaded);
  show(exportButton, diagnosticsLoaded);
  if (failed) renderSnapshotDiagnostics();
  setResult("");
  renderVersions();
  paint(journey.observe(snapshot, performance.now()));
}

window.compozyBoot.onState(render);
render({ state: "resolving" });
setInterval(() => paint(journey.view(performance.now())), 1000);
