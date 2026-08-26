/**
 * Terminal wire shapes corresponding to `_dx.md` and the generated OpenAPI
 * contract. A list entry is byte-identical to `get`'s projection, which is why
 * one type serves both.
 */

/** Closed everywhere it appears — `_dx.md` → *Native Tools*. */
export type TerminalLeaseState = "agent_owned" | "human_owned" | "available";

export type TerminalActorKind = "human" | "agent" | "system";

/** A pty terminal is watchable and writable; a pipe terminal is a log. */
export type TerminalMode = "pty" | "pipe";

export type TerminalRunState = "running" | "exited";

/** Never fabricated: a cause the daemon could not verify stays `unknown`. */
export type TerminalExitCause = "exited" | "signaled" | "unknown";

export type TerminalSignal = "INT" | "TERM" | "KILL" | "HUP";

export interface TerminalActor {
  kind: TerminalActorKind;
  id: string;
}

export interface TerminalBoundRun {
  session_id: string;
  run_id: string;
}

export interface TerminalExit {
  cause: TerminalExitCause;
  code?: number | null;
  signal?: string | null;
  at: string;
}

export interface TerminalExitNotice {
  cause: TerminalExitCause;
  code: number | null;
  signal: string | null;
}

export interface TerminalCapabilities {
  interactive: boolean;
}

/** The one public projection, shared by list, get, and `terminal.created`. */
export interface TerminalInfo {
  id: string;
  workspace_id: string;
  profile_id: string;
  profile_name: string;
  title: string;
  shell: string;
  cwd: string;
  mode: TerminalMode;
  state: TerminalRunState;
  controller: TerminalActor | null;
  lease: TerminalLeaseState;
  viewers: number;
  bound_run: TerminalBoundRun | null;
  capabilities: TerminalCapabilities;
  created_at: string;
  exit?: TerminalExit | null;
}

export interface TerminalAttachTicket {
  ticket: string;
  expires_at: string;
}

export type TerminalAttachMode = "read" | "write";

/** Watchers drop frames they cannot keep up with; writers return credit. */
export type TerminalFlowMode = "drop" | "ack";

export interface CreateTerminalInput {
  cwd?: string;
  shell?: string;
  cols?: number;
  rows?: number;
  title?: string;
}

/** Pending prompts, workspace- and profile-scoped. */
export interface TerminalInputRequest {
  id: string;
  terminal_id: string;
  profile_id: string;
  profile_name: string;
  reason: string;
  prompt_excerpt: string;
  redacted: boolean;
  requested_at: string;
}

/** The frozen outcome enum; the boards' "Declined" is copy for `rejected`. */
export type TerminalInputOutcome = "answered" | "rejected" | "superseded" | "expired";

export interface TerminalInputAnswerResult {
  delivered_bytes: number;
  redacted: boolean;
}

export interface TerminalInputRejectResult {
  outcome: "rejected";
}

/** How a command boundary was established. `idle` is the heuristic one. */
export type TerminalDetectedBy = "exact" | "marker" | "idle";

/** Which permission covered the run. */
export type TerminalApproval =
  | "approved_once"
  | "approved_always"
  | "allowlisted"
  | "human"
  | "none";

export interface TerminalJournalEntry {
  command_id: string;
  terminal_id: string | null;
  profile_id: string;
  profile_name: string;
  actor: TerminalActor;
  command: string;
  argv_digest?: string | null;
  cwd: string;
  started_at: string;
  duration_ms: number | null;
  exit_code: number | null;
  signal: string | null;
  exit_cause: TerminalExitCause;
  detected_by: TerminalDetectedBy;
  approval: TerminalApproval;
  output_bytes: number;
  truncated: boolean;
  recording?: string | null;
}

/**
 * No total: retention is unbounded, so a truthful count would mean reading the
 * whole history. The panel states rows loaded instead.
 */
export interface TerminalJournalPage {
  entries: TerminalJournalEntry[];
  next: string | null;
}

export interface TerminalJournalFilters {
  actor?: TerminalActorKind | null;
  since?: string | null;
  failed?: boolean;
  terminalId?: string | null;
  limit?: number;
}

export interface TerminalRecording {
  id: string;
  state: "recording" | "saved";
}

/** Rendered screen, stream tail, or a scrollback line range. */
export type TerminalReadView = "screen" | "tail" | "lines";

export interface TerminalReadResult {
  content: string;
  seq: number;
  truncated: boolean;
  busy: boolean;
  untrusted: boolean;
}

/** The two selectors are mutually exclusive — `profile_selection_conflict`. */
export type TerminalScopeParams =
  | { profile: string; all_profiles?: never }
  | { profile?: never; all_profiles: true };

export type TerminalProfileScopeParams = Extract<TerminalScopeParams, { profile: string }>;

/** Cache and stream identity. A read without both halves cannot be scoped. */
export interface TerminalScopeKey {
  workspaceId: string;
  profileKey: string;
}
