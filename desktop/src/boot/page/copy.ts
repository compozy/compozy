import type { ShellSnapshot, ShellStateName } from "../../state/app-state";
import { publicSafeText } from "../../state/public-safe-text";
import type { JourneyView, StepId } from "./journey";
import { readinessLine, waitLine } from "./readiness-copy";

const TITLES: Record<ShellStateName, string> = {
  resolving: "Looking for CompozyOS",
  provisioning: "Setting up CompozyOS",
  starting: "Starting CompozyOS",
  attaching: "Connecting to CompozyOS",
  product: "CompozyOS is ready",
  updating: "Updating CompozyOS",
  disconnected: "CompozyOS is unavailable",
  skew: "CompozyOS needs an update",
  error: "CompozyOS could not start",
};

const WORK_NAMES: Record<StepId, string> = {
  download: "Downloading the runtime",
  verify: "Verifying the runtime",
  install: "Installing the runtime",
  migrate: "Upgrading data",
  start: "Starting the runtime",
  connect: "Connecting",
};

/** The headline: a journey that replaces a runtime reads as an update from start to finish. */
export function titleFor(snapshot: ShellSnapshot, view: JourneyView): string {
  const inJourney = view.layout === "journey" && snapshot.state !== "error";
  if (inJourney && snapshot.from_version) return "Updating CompozyOS";
  if (inJourney && snapshot.state !== "updating") return TITLES.provisioning;
  return TITLES[snapshot.state];
}

function skewMessage(snapshot: ShellSnapshot): string {
  const runtime = publicSafeText(snapshot.runtime, "");
  const needed = publicSafeText(snapshot.needed, "");
  return snapshot.newer === true
    ? `Update CompozyOS to ${needed} or newer before reconnecting to runtime ${runtime}.`
    : `Runtime ${runtime} does not satisfy ${needed}. Repair or update the runtime before reconnecting.`;
}

/** The one-line reason under the headline; null when the headline says enough. */
export function detailFor(
  snapshot: ShellSnapshot & { message?: unknown },
  view: JourneyView
): string | null {
  const message = publicSafeText(snapshot.message, "");
  if (message) return message;
  const target = snapshot.to_version ? ` for ${snapshot.to_version}` : "";
  switch (snapshot.state) {
    case "error":
      return publicSafeText(snapshot.error?.safe_message, "") || null;
    case "skew":
      return skewMessage(snapshot);
    case "disconnected":
      return "The runtime stopped answering. Reconnecting.";
    case "updating":
      return "Your sessions pause for a moment and continue after the update.";
    default:
      break;
  }
  const waiting = waitLine(snapshot.boot);
  if (waiting) return waiting;
  if (view.current === "migrate") {
    return snapshot.work === "sessions"
      ? `Upgrading your session history${target}. This runs once.`
      : `Upgrading your data${target}. This runs once.`;
  }
  if (view.layout === "journey" && view.current === "install") {
    return snapshot.from_version
      ? "Putting the new runtime in place. Your data stays where it is."
      : "Putting the runtime in place. Nothing else on your computer changes.";
  }
  const step = readinessLine(snapshot.boot);
  if (step) return step;
  if (view.layout === "journey" && (view.current === "start" || view.current === "connect")) {
    return "Starting the runtime with your data.";
  }
  if (snapshot.state === "starting" && (snapshot.attempt ?? 1) > 1) {
    return "The runtime did not answer the first time. Trying again.";
  }
  return null;
}

export function workNameFor(step: StepId, unit: "sessions" | "records" | undefined): string {
  if (step === "migrate" && unit === "sessions") return "Upgrading sessions";
  return WORK_NAMES[step];
}

/** Remaining time in plain words, or a holding line while the rate is unknown. */
export function estimateFor(minutesLeft: number | null, total: number, stalled: boolean): string {
  if (stalled) return "Waiting for the runtime";
  if (total <= 0) return "Counting what needs an upgrade";
  if (minutesLeft === null) return "Estimating time left";
  if (minutesLeft < 1) return "Less than a minute left";
  if (minutesLeft < 90) return `About ${Math.round(minutesLeft)} min left`;
  return `About ${Math.round(minutesLeft / 60)} h left`;
}

/** `m:ss` under an hour, `h:mm:ss` above: a clock reads as "still counting" at a glance. */
export function clock(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = String(seconds % 60).padStart(2, "0");
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, "0")}:${rest}` : `${minutes}:${rest}`;
}

/** The status line's word; the state mark beside it carries the same tone. */
export function statusWordFor(snapshot: ShellSnapshot, view: JourneyView): string {
  switch (snapshot.state) {
    case "product":
      return "Ready";
    case "disconnected":
      return "Reconnecting";
    case "skew":
      return "Needs you";
    case "error":
      return "Failed";
    default:
      // The clock beside the word is total boot time, so a stall reads as waiting, not a duration.
      return view.stalled || snapshot.boot?.wait ? "Waiting" : "Working";
  }
}
