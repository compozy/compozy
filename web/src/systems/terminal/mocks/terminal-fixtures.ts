import type { TerminalGrant } from "../lib/terminal-grant";
import type {
  TerminalExit,
  TerminalInfo,
  TerminalInputRequest,
  TerminalJournalEntry,
} from "../types";

/**
 * The design set's data story, and only it.
 *
 * The boards fix one project and one cast so every surface can be compared
 * against them without inventing a second world. Minting extra fixtures would
 * quietly break that comparison.
 */
export const TERMINAL_FIXTURE_WORKSPACE = "ws-atlas";
export const TERMINAL_FIXTURE_PROFILE = "work";
export const TERMINAL_FIXTURE_PROFILE_ID = "01JB4Z2K9QW8XR3TFN6VYD5HAC";
export const TERMINAL_FIXTURE_VIEWER = "pedro";

function baseTerminal(overrides: Partial<TerminalInfo>): TerminalInfo {
  return {
    id: "term-4f21c9a03b7e",
    workspace_id: "ws-atlas",
    profile_id: TERMINAL_FIXTURE_PROFILE_ID,
    profile_name: TERMINAL_FIXTURE_PROFILE,
    title: "dev server",
    shell: "/bin/zsh",
    cwd: "~/dev/atlas-api",
    mode: "pty",
    state: "running",
    controller: { kind: "human", id: TERMINAL_FIXTURE_VIEWER },
    lease: "human_owned",
    viewers: 2,
    bound_run: null,
    capabilities: { interactive: true },
    created_at: "2026-08-25T12:40:00Z",
    exit: null,
    ...overrides,
  };
}

/** `dev server` — a pty terminal this operator controls. */
export const DEV_SERVER_TERMINAL = baseTerminal({});

/** `psql` — controlled by the agent, and the one asking a question. */
export const PSQL_TERMINAL = baseTerminal({
  id: "term-9cd7e14b2a66",
  title: "psql",
  controller: { kind: "agent", id: "Claude Code" },
  lease: "agent_owned",
  viewers: 1,
});

/** `make gate` — exec-born, so a read-only log rather than a prompt. */
export const MAKE_GATE_TERMINAL = baseTerminal({
  id: "term-a03b558d21f0",
  title: "make gate",
  mode: "pipe",
  controller: { kind: "agent", id: "Claude Code" },
  lease: "agent_owned",
  viewers: 0,
});

/** `ssh staging` — finished, still readable through its exit retention. */
export const SSH_STAGING_TERMINAL = baseTerminal({
  id: "term-1e8f7a55c402",
  title: "ssh staging",
  cwd: "~/dev",
  state: "exited",
  lease: "available",
  controller: null,
  viewers: 0,
  exit: { cause: "exited", code: 0, signal: null, at: "2026-08-25T12:52:00Z" },
});

/**
 * The same finished terminal, ended a different way.
 *
 * One cast, four outcomes: the exit bar's states are a property of how the
 * program ended, not of a second project.
 */
export function exitedTerminal(exit: TerminalExit): TerminalInfo {
  const suffix = exit.signal ?? exit.code ?? exit.cause;
  return baseTerminal({
    id: `term-1e8f7a55c402-${suffix}`,
    title: "ssh staging",
    cwd: "~/dev",
    state: "exited",
    lease: "available",
    controller: null,
    viewers: 0,
    exit,
  });
}

/** `dev server`, but another person is typing in it. */
export const CONTESTED_TERMINAL = baseTerminal({
  controller: { kind: "human", id: "marina" },
  lease: "human_owned",
});

export const TERMINAL_FIXTURES: readonly TerminalInfo[] = [
  DEV_SERVER_TERMINAL,
  MAKE_GATE_TERMINAL,
  PSQL_TERMINAL,
  SSH_STAGING_TERMINAL,
];

/** The eight-terminal cap, so overflow can be exercised at its real boundary. */
export const TERMINAL_FIXTURES_AT_CAP: readonly TerminalInfo[] = [
  DEV_SERVER_TERMINAL,
  PSQL_TERMINAL,
  MAKE_GATE_TERMINAL,
  SSH_STAGING_TERMINAL,
  baseTerminal({ id: "term-77c1d0e94ab3", title: "tail api log" }),
  baseTerminal({ id: "term-2c8de1b704f9", title: "worker" }),
  baseTerminal({ id: "term-4aa01f22e6c3", title: "e2e suite" }),
  baseTerminal({ id: "term-8be44d10a25b", title: "bun install" }),
];

/**
 * Remembered terminal decisions, exactly as the daemon stores them.
 *
 * `input_digest` is a `sha256:` digest of the exact tool input — the contract
 * validates that prefix and never returns the input itself. Fixtures that put a
 * terminal id or a command string here would let the UI be built against a
 * shape the runtime cannot produce.
 */
export const TERMINAL_GRANT_FIXTURES: readonly TerminalGrant[] = [
  {
    id: "grant-typing-1",
    kind: "typing",
    inputDigest: "sha256:9f21ac04b7e31d5a8c6f0e2b4d7a19c3e58f6b0d2a4c8e1f3b5d7a9c1e3f5b7d",
    agentName: "Claude Code",
    grantedAt: "2026-08-25T12:44:00Z",
  },
  {
    id: "grant-shape-1",
    kind: "command_shape",
    inputDigest: "sha256:1e8f7a55c4020b3d6e9a2c5f8b1d4e7a0c3f6b9d2e5a8c1f4b7d0e3a6c9f2b5d",
    agentName: "Claude Code",
    grantedAt: "2026-08-25T12:12:00Z",
  },
  {
    id: "grant-shape-2",
    kind: "command_shape",
    agentName: "Claude Code",
    grantedAt: "2026-08-25T11:58:00Z",
  },
];

export const PASSWORD_REQUEST: TerminalInputRequest = {
  id: "req-3f8a",
  terminal_id: PSQL_TERMINAL.id,
  profile_id: TERMINAL_FIXTURE_PROFILE_ID,
  profile_name: TERMINAL_FIXTURE_PROFILE,
  reason: "I need the staging database password to run the journal migration check.",
  prompt_excerpt: "Password for user atlas:",
  redacted: true,
  requested_at: "2026-08-25T12:44:00Z",
};

export const CONFIRMATION_REQUEST: TerminalInputRequest = {
  id: "req-9c11",
  terminal_id: SSH_STAGING_TERMINAL.id,
  profile_id: TERMINAL_FIXTURE_PROFILE_ID,
  profile_name: TERMINAL_FIXTURE_PROFILE,
  reason: "The migration would overwrite an existing file — your call.",
  prompt_excerpt: "Overwrite migration 00078_terminal_journal.sql? [y/N]",
  redacted: false,
  requested_at: "2026-08-25T12:47:00Z",
};

function journalEntry(overrides: Partial<TerminalJournalEntry>): TerminalJournalEntry {
  return {
    command_id: "cmd-5f0a1e",
    terminal_id: MAKE_GATE_TERMINAL.id,
    profile_id: TERMINAL_FIXTURE_PROFILE_ID,
    profile_name: TERMINAL_FIXTURE_PROFILE,
    actor: { kind: "agent", id: "Claude Code" },
    command: "make gate",
    cwd: "~/dev/atlas-api",
    started_at: "2026-08-25T12:47:00Z",
    duration_ms: 161_000,
    exit_code: 0,
    signal: null,
    exit_cause: "exited",
    detected_by: "marker",
    approval: "allowlisted",
    output_bytes: 184_320,
    truncated: false,
    recording: "rec-9f21ac",
    ...overrides,
  };
}

/** Mixed exact, verified and estimated rows — the board's populated journal. */
export const JOURNAL_FIXTURES: readonly TerminalJournalEntry[] = [
  journalEntry({}),
  journalEntry({
    command_id: "cmd-77c1d0",
    terminal_id: PSQL_TERMINAL.id,
    command: "psql -h staging.internal -U atlas atlas_api",
    started_at: "2026-08-25T12:44:00Z",
    duration_ms: 3100,
    detected_by: "exact",
    approval: "human",
    recording: null,
  }),
  journalEntry({
    command_id: "cmd-1e8f7a",
    terminal_id: DEV_SERVER_TERMINAL.id,
    actor: { kind: "human", id: TERMINAL_FIXTURE_VIEWER },
    command: "git rebase --continue",
    started_at: "2026-08-25T12:41:00Z",
    duration_ms: 900,
    detected_by: "idle",
    approval: "none",
    recording: null,
  }),
  journalEntry({
    command_id: "cmd-4aa01f",
    command: "bun run test --filter terminal",
    started_at: "2026-08-25T12:38:00Z",
    duration_ms: 14_200,
    exit_code: 1,
    detected_by: "marker",
    approval: "approved_once",
  }),
  journalEntry({
    command_id: "cmd-2c8de1",
    terminal_id: SSH_STAGING_TERMINAL.id,
    actor: { kind: "human", id: TERMINAL_FIXTURE_VIEWER },
    command: "ssh staging",
    cwd: "~/dev",
    started_at: "2026-08-25T12:31:00Z",
    duration_ms: 62_000,
    exit_code: null,
    signal: "TERM",
    exit_cause: "signaled",
    detected_by: "idle",
    approval: "none",
    recording: null,
  }),
  journalEntry({
    command_id: "cmd-8be44d",
    terminal_id: "term-77c1d0e94ab3",
    actor: { kind: "human", id: "marina" },
    command: "bun run build",
    started_at: "2026-08-25T12:26:00Z",
    duration_ms: null,
    exit_code: null,
    exit_cause: "unknown",
    detected_by: "idle",
    approval: "none",
    recording: null,
  }),
];

/**
 * A real asciicast v2 artifact for `make gate`, at its real length.
 *
 * The gate takes minutes, so the recording does too — a two-second stand-in
 * would make every transport state look like the end of the file.
 */
export const RECORDING_FIXTURE = [
  JSON.stringify({ version: 2, width: 96, height: 28, title: "make gate" }),
  JSON.stringify([0.0, "o", "$ make gate\r\n"]),
  JSON.stringify([0.4, "o", "gate: classifying diff vs merge-base…\r\n"]),
  JSON.stringify([2.1, "o", "gate: go lane → go-lint + go test -race (scoped)\r\n"]),
  JSON.stringify([38.6, "o", "ok   internal/terminal  2.148s\r\n"]),
  JSON.stringify([44.9, "o", "ok   internal/terminal/journal  0.914s\r\n"]),
  JSON.stringify([50.2, "o", "gate: web lane → turbo --filter …\r\n"]),
  JSON.stringify([131.4, "o", "web:test: 227 passed (14.2s)\r\n"]),
  JSON.stringify([149.0, "o", "gate: PASS\r\n"]),
].join("\n");
