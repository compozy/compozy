/**
 * The quarantine entry reader (US-024 AC-1): the daemon retains a sanitized,
 * self-contained repair record per quarantined node — the classified attempt
 * chain, the remediation hint, the involved target, and the node's input
 * reference. The payload crosses the wire as an opaque JSON blob
 * (`node_controls[].quarantine_entry`), so every field is read through a guard:
 * a malformed or partial entry degrades to whatever it does declare instead of
 * throwing on the run page.
 *
 */

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function str(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function stringField(record: Record<string, unknown>, key: string): string {
  return str(record[key]);
}

function numberField(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function array(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

/** One classified attempt in the quarantine chain — the repair evidence. */
export interface LoopQuarantineAttempt {
  attempt: number;
  failureClass?: string;
  failureCode?: string;
  cause?: string;
  hint?: string;
  target?: string;
  disposition?: string;
  startedAt?: string;
  endedAt?: string;
}

/** One quarantine episode: the attempt chain that ended in a set-aside. */
export interface LoopQuarantineEpisode {
  generation: number;
  quarantinedAt?: string;
  attempts: LoopQuarantineAttempt[];
}

/** One recorded requeue: who re-entered the node after repair, and why. */
export interface LoopQuarantineRequeue {
  actorKind: string;
  actorId: string;
  reason?: string;
  requestedAt?: string;
  generation: number;
}

export interface LoopQuarantineEntry {
  nodeId: string;
  inputRef: string;
  target: string;
  episodes: LoopQuarantineEpisode[];
  requeues: LoopQuarantineRequeue[];
  /** The daemon dropped older episodes/requeues to stay inside the entry size cap. */
  truncated: boolean;
  /** Total attempts across every retained episode (the "4 attempts" headline). */
  attemptCount: number;
  /** The newest attempt's hint — the "What to try" line when the daemon recorded one. */
  hint: string;
  /** The newest episode's quarantine stamp. */
  quarantinedAt?: string;
}

function readAttempt(value: unknown): LoopQuarantineAttempt | null {
  const record = asRecord(value);
  if (!record) return null;
  const attempt = numberField(record, "attempt");
  return {
    attempt: attempt ?? 0,
    failureClass: stringField(record, "failure_class") || undefined,
    failureCode: stringField(record, "failure_code") || undefined,
    cause: stringField(record, "cause") || undefined,
    hint: stringField(record, "hint") || undefined,
    target: stringField(record, "target") || undefined,
    disposition: stringField(record, "disposition") || undefined,
    startedAt: stringField(record, "started_at") || undefined,
    endedAt: stringField(record, "ended_at") || undefined,
  };
}

function readEpisode(value: unknown): LoopQuarantineEpisode | null {
  const record = asRecord(value);
  if (!record) return null;
  const attempts: LoopQuarantineAttempt[] = [];
  for (const item of array(record.attempts)) {
    const attempt = readAttempt(item);
    if (attempt) attempts.push(attempt);
  }
  return {
    generation: numberField(record, "generation") ?? 0,
    quarantinedAt: stringField(record, "quarantined_at") || undefined,
    attempts,
  };
}

function readRequeue(value: unknown): LoopQuarantineRequeue | null {
  const record = asRecord(value);
  if (!record) return null;
  const actorKind = stringField(record, "actor_kind");
  const actorId = stringField(record, "actor_id");
  if (actorKind === "" && actorId === "") return null;
  return {
    actorKind,
    actorId,
    reason: stringField(record, "reason") || undefined,
    requestedAt: stringField(record, "requested_at") || undefined,
    generation: numberField(record, "generation") ?? 0,
  };
}

/**
 * Reads one durable quarantine entry. Returns null when the payload is absent or
 * carries no node identity — the sheet then renders nothing rather than an empty
 * shell that implies the daemon recorded a repair record it did not.
 */
export function readQuarantineEntry(value: unknown): LoopQuarantineEntry | null {
  const record = asRecord(value);
  if (!record) return null;
  const nodeId = stringField(record, "node_id");
  if (nodeId === "") return null;
  const episodes: LoopQuarantineEpisode[] = [];
  for (const item of array(record.episodes)) {
    const episode = readEpisode(item);
    if (episode) episodes.push(episode);
  }
  const requeues: LoopQuarantineRequeue[] = [];
  for (const item of array(record.requeues)) {
    const requeue = readRequeue(item);
    if (requeue) requeues.push(requeue);
  }
  const attempts = episodes.flatMap(episode => episode.attempts);
  const newest = attempts[attempts.length - 1];
  return {
    nodeId,
    inputRef: stringField(record, "input_ref"),
    target: stringField(record, "target"),
    episodes,
    requeues,
    truncated: record.truncated === true,
    attemptCount: attempts.length,
    hint: newest?.hint ?? "",
    quarantinedAt: episodes[episodes.length - 1]?.quarantinedAt,
  };
}

/**
 * Flattens the chain into render order (oldest attempt first) and marks where a
 * requeue opened a new episode, so the sheet can draw the episode boundary the
 * daemon actually recorded.
 */
export interface LoopQuarantineChainRow extends LoopQuarantineAttempt {
  key: string;
  episodeIndex: number;
  generation: number;
  /** The requeue that opened this attempt's episode; absent on the first episode. */
  openedBy?: LoopQuarantineRequeue;
}

export function quarantineChainRows(entry: LoopQuarantineEntry): LoopQuarantineChainRow[] {
  const rows: LoopQuarantineChainRow[] = [];
  entry.episodes.forEach((episode, episodeIndex) => {
    episode.attempts.forEach((attempt, attemptIndex) => {
      rows.push({
        ...attempt,
        key: `${episodeIndex}-${attemptIndex}-${attempt.attempt}`,
        episodeIndex,
        generation: episode.generation,
        // A requeue precedes every episode after the first; index n-1 opened episode n.
        openedBy:
          attemptIndex === 0 && episodeIndex > 0 ? entry.requeues[episodeIndex - 1] : undefined,
      });
    });
  });
  return rows;
}
