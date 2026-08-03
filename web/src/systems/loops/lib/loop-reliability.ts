import type { LoopGraph } from "./loop-graph";
import type { LoopDetail } from "../types";

export type LoopEffectiveLifecycle = NonNullable<LoopDetail["effective_lifecycle"]>;

/** Which fixed icon a tile carries — one concept, one icon, across every loops surface. */
export type LoopReliabilityIcon = "retry" | "deadline" | "wait" | "control";

export interface LoopReliabilityTile {
  id: "retry" | "deadline" | "wait" | "control";
  icon: LoopReliabilityIcon;
  title: string;
  body: string;
  /** The DSL keys behind the tile, rendered as mono micro text (never a badge). */
  codes: string[];
}

export interface LoopReliabilityInput {
  effectiveLifecycle: LoopDetail["effective_lifecycle"];
  graph: LoopGraph;
}

/**
 * Plain-language posture of the Spec 1 failure contract for one Loop.
 *
 * Every tile is derived from something the daemon actually reports — the workspace's
 * effective lifecycle envelope or a key the definition authored. A tile whose facts
 * are absent is omitted rather than filled with plausible prose (SD-007): a loop that
 * declares no `deadline`/`timeout` gets no time-budget tile, and a loop with no wait
 * node gets no wait tile. No tile introduces a metric the runtime does not expose.
 */
export function buildLoopReliability(input: LoopReliabilityInput): LoopReliabilityTile[] {
  const tiles: LoopReliabilityTile[] = [];
  const retry = retryTile(input.effectiveLifecycle);
  if (retry) tiles.push(retry);
  const deadline = deadlineTile(input.graph);
  if (deadline) tiles.push(deadline);
  const wait = waitTile(input.graph);
  if (wait) tiles.push(wait);
  tiles.push(controlTile());
  return tiles;
}

function retryTile(lifecycle: LoopDetail["effective_lifecycle"]): LoopReliabilityTile | null {
  if (!lifecycle) return null;
  const attempts = lifecycle.retry_max_attempts;
  if (!Number.isFinite(attempts) || attempts <= 0) return null;
  const base = formatLoopDuration(lifecycle.retry_backoff_base);
  const max = formatLoopDuration(lifecycle.retry_backoff_max);
  const pauses = base && max ? ` with pauses growing from ${base} to ${max}` : "";
  return {
    id: "retry",
    icon: "retry",
    title: "Steps retry on their own",
    body: `Up to ${attempts} ${attempts === 1 ? "attempt" : "attempts"}${pauses}. A step that keeps failing is set aside in quarantine, never lost — requeue it from the run page.`,
    codes: ["retry", "backoff", "quarantine"],
  };
}

function deadlineTile(graph: LoopGraph): LoopReliabilityTile | null {
  const timeout = firstDefined(graph.nodes.map(node => node.timeout));
  const deadline = firstDefined(graph.nodes.map(node => node.deadline));
  if (!timeout && !deadline) return null;
  const clauses: string[] = [];
  if (timeout) clauses.push(`one attempt stops at ${formatLoopDuration(timeout)}`);
  if (deadline) clauses.push(`the whole step at ${formatLoopDuration(deadline)}`);
  return {
    id: "deadline",
    icon: "deadline",
    title: "Steps have a time budget",
    body: `${capitalize(clauses.join("; "))}. Nothing hangs silently.`,
    codes: [timeout ? "timeout" : null, deadline ? "deadline" : null].filter(isPresent),
  };
}

function waitTile(graph: LoopGraph): LoopReliabilityTile | null {
  const waits = graph.nodes.filter(node => node.kind === "wait");
  if (waits.length === 0) return null;
  const expires = firstDefined(waits.map(node => node.waitExpiresAfter));
  const expiry = expires
    ? ` A wait gives up after ${formatLoopDuration(expires)} and takes its fallback path instead of waiting forever.`
    : "";
  return {
    id: "wait",
    icon: "wait",
    title: "Waits stay visible",
    body: `Waiting steps show their age in the run page inventory rather than looking idle.${expiry}`,
    codes: expires ? ["wait", "expires"] : ["wait"],
  };
}

function controlTile(): LoopReliabilityTile {
  return {
    id: "control",
    icon: "control",
    title: "You stay in control",
    body: "Pause, resume, cancel or kill the run and any single step from the run page. Cancel lets in-flight work finish and closes the run as canceled; kill stops it now.",
    codes: ["cancel", "kill"],
  };
}

/**
 * Renders a Go duration (`45m0s`, `2h0m0s`, `1h30m0s`) as the shortest truthful label.
 * Unparseable values pass through untouched — never guessed at, never dropped.
 */
export function formatLoopDuration(value: string | undefined): string {
  if (!value) return "";
  const match = /^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+(?:\.\d+)?)s)?$/.exec(value.trim());
  if (!match) return value.trim();
  const [, hours, minutes, seconds] = match;
  const parts: string[] = [];
  if (hours && hours !== "0") parts.push(`${hours}h`);
  if (minutes && minutes !== "0") parts.push(`${minutes}m`);
  if (seconds && Number(seconds) !== 0) parts.push(`${trimZeros(seconds)}s`);
  return parts.length > 0 ? parts.join(" ") : value.trim();
}

function trimZeros(seconds: string): string {
  return seconds.includes(".") ? seconds.replace(/\.?0+$/, "") : seconds;
}

function firstDefined(values: readonly (string | undefined)[]): string | undefined {
  return values.find(value => value !== undefined && value !== "");
}

function isPresent(value: string | null): value is string {
  return value !== null;
}

function capitalize(value: string): string {
  return value.length === 0 ? value : `${value[0].toUpperCase()}${value.slice(1)}`;
}
