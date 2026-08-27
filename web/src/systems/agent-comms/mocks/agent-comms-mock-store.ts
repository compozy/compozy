/**
 * The dataset behind the agent-comms mocks, and the filtering the daemon does.
 *
 * These do real filtering and real counting rather than returning a canned
 * array. That matters more than usual here: the whole surface's honesty rests
 * on `total` describing the *filtered population* while `items` describes only
 * one page, and a handler that returned `items.length` as the total would make
 * every count test pass against a lie. So the filter runs first, `total` is taken
 * from the filtered set, and only then is a page sliced off.
 *
 * A store instance owns its own copy of the data. Nothing here is module-level,
 * so two consumers — two stories loaded at once, say — can never see each
 * other's writes.
 */
import type { CallMessagePayload, CallPayload } from "../types";

const DEFAULT_LIMIT = 50;
const MAX_LIMIT = 200;

export interface AgentCommsDataset {
  calls?: readonly CallPayload[];
  messages?: readonly CallMessagePayload[];
}

export interface CallsPage {
  items: CallPayload[];
  total: number;
  next_cursor?: string;
}

export interface MessagesPage {
  items: CallMessagePayload[];
  next_cursor?: string;
}

/**
 * Whether this call is still *waiting on someone*.
 *
 * Mirrors the daemon predicate (`global_db_calls_read.go`): a terminal call with
 * no usable answer stays in the attention population only until the operator or
 * another agent addresses the same child again. Reproducing the resolution rule
 * rather than the state list is the whole point — a mock that matched on state
 * alone would show a badge clearing test passing while the badge stayed lit.
 */
function needsAttention(
  call: CallPayload,
  calls: readonly CallPayload[],
  messages: readonly CallMessagePayload[]
): boolean {
  if (call.state !== "invalid-result" && call.state !== "completed-without-result") return false;
  const child = call.child_session_id ?? "";
  if (child === "") return false;
  const since = call.settled_at ?? call.updated_at;
  const laterCall = calls.some(
    other =>
      other.call_id !== call.call_id &&
      (other.child_session_id ?? "") === child &&
      other.created_at > since
  );
  if (laterCall) return false;
  return !messages.some(message => message.to_session_id === child && message.created_at > since);
}

function matchesCallFilters(
  call: CallPayload,
  params: URLSearchParams,
  calls: readonly CallPayload[],
  messages: readonly CallMessagePayload[]
): boolean {
  const states = new Set(
    params
      .getAll("state")
      .flatMap(value => value.split(","))
      .map(value => value.trim())
      .filter(Boolean)
  );
  if (states.size > 0 && !states.has(call.state)) return false;
  if (params.get("attention") === "true" && !needsAttention(call, calls, messages)) return false;
  const caller = normalizedParam(params, "caller");
  if (caller !== null && call.caller.id !== caller) return false;
  const child = normalizedParam(params, "child_session_id");
  if (child !== null && (call.child_session_id ?? "") !== child) return false;
  const root = normalizedParam(params, "root_session_id");
  if (root !== null && call.root_session_id !== root) return false;
  const agent = normalizedParam(params, "agent");
  if (agent !== null && (call.agent ?? "") !== agent) return false;
  return true;
}

const CURSOR_PREFIX = "cursor_1_";

function normalizedParam(params: URLSearchParams, name: string): string | null {
  const value = params.get(name)?.trim() ?? "";
  return value === "" ? null : value;
}

function queryFingerprint(params: URLSearchParams, kind: string): string {
  const canonical = [...params.entries()]
    .filter(([name]) => name !== "cursor" && name !== "limit")
    .map(([name, value]) => [name, value.trim()] as const)
    .sort(([leftName, leftValue], [rightName, rightValue]) =>
      leftName === rightName
        ? leftValue.localeCompare(rightValue)
        : leftName.localeCompare(rightName)
    );
  let hash = 2_166_136_261;
  for (const byte of new TextEncoder().encode(JSON.stringify([kind, canonical]))) {
    hash ^= byte;
    hash = Math.imul(hash, 16_777_619);
  }
  return (hash >>> 0).toString(36);
}

function encodeCursor(offset: number, fingerprint: string): string {
  return `${CURSOR_PREFIX}${fingerprint}_${offset.toString(36)}`;
}

function decodeCursor(value: string, fingerprint: string): number | null {
  const match = value.match(/^cursor_1_([0-9a-z]+)_([0-9a-z]+)$/);
  if (!match || match[1] !== fingerprint) return null;
  const offset = Number.parseInt(match[2] ?? "", 36);
  return Number.isSafeInteger(offset) && offset >= 0 ? offset : null;
}

/** Opaque cursors: validated here, exhausted exactly once. */
function slice<T>(matched: readonly T[], params: URLSearchParams, kind: string) {
  const rawLimit = Number(params.get("limit") ?? DEFAULT_LIMIT);
  const limit =
    Number.isFinite(rawLimit) && rawLimit > 0
      ? Math.min(Math.floor(rawLimit), MAX_LIMIT)
      : DEFAULT_LIMIT;
  const fingerprint = queryFingerprint(params, kind);
  const cursor = params.get("cursor");
  const start = cursor === null ? 0 : decodeCursor(cursor, fingerprint);
  if (start === null || start >= matched.length) return { items: [] };
  const items = matched.slice(start, start + limit);
  const nextStart = start + items.length;
  return {
    items,
    ...(nextStart < matched.length ? { next_cursor: encodeCursor(nextStart, fingerprint) } : {}),
  };
}

export interface AgentCommsMockStore {
  pageCalls(workspaceId: string, url: URL): CallsPage;
  pageMessages(workspaceId: string, url: URL): MessagesPage;
  findCall(workspaceId: string, url: URL, callId: string): CallPayload | undefined;
  /** Cancel is idempotent: a settled call answers with its terminal state. */
  cancelCall(workspaceId: string, url: URL, callId: string): CallPayload["state"] | undefined;
  addCall(call: CallPayload): void;
  addMessage(message: CallMessagePayload): void;
  setCalls(next: readonly CallPayload[]): void;
  setMessages(next: readonly CallMessagePayload[]): void;
  snapshotCalls(): readonly CallPayload[];
  nextCallId(): string;
  nextMessageId(): string;
  resetSequences(): void;
}

export function createAgentCommsMockStore(dataset: AgentCommsDataset = {}): AgentCommsMockStore {
  let calls: CallPayload[] = [...(dataset.calls ?? [])];
  let messages: CallMessagePayload[] = [...(dataset.messages ?? [])];
  let callSequence = 0;
  let messageSequence = 0;

  return {
    pageCalls(workspaceId, url) {
      const params = url.searchParams;
      const scoped = calls.filter(call => matchesOwner(call, workspaceId, params));
      const scopedMessages = messages.filter(message => matchesOwner(message, workspaceId, params));
      const matched = scoped.filter(call =>
        matchesCallFilters(call, params, scoped, scopedMessages)
      );
      // Count the whole filtered set, then page it — the same order the daemon
      // uses (`SELECT COUNT(*)` over the filter, cursor applied afterwards).
      return { total: matched.length, ...slice(matched, params, "calls") };
    },
    pageMessages(workspaceId, url) {
      const params = url.searchParams;
      const session = normalizedParam(params, "session");
      const matched = messages
        .filter(message => matchesOwner(message, workspaceId, params))
        .filter(message => session === null || message.to_session_id === session);
      // No `total`: the mailbox page is uncounted by contract.
      return slice(matched, params, "call-messages");
    },
    findCall(workspaceId, url, callId) {
      return calls.find(
        call => call.call_id === callId && matchesOwner(call, workspaceId, url.searchParams)
      );
    },
    cancelCall(workspaceId, url, callId) {
      const call = calls.find(
        item => item.call_id === callId && matchesOwner(item, workspaceId, url.searchParams)
      );
      if (!call) return undefined;
      const state = call.state === "queued" || call.state === "running" ? "canceled" : call.state;
      calls = calls.map(item => (item.call_id === callId ? { ...item, state } : item));
      return state;
    },
    addCall(call) {
      calls = [call, ...calls];
    },
    addMessage(message) {
      messages = [...messages, message];
    },
    setCalls(next) {
      calls = [...next];
    },
    setMessages(next) {
      messages = [...next];
    },
    snapshotCalls() {
      return calls;
    },
    nextCallId() {
      callSequence += 1;
      return `call_mock_${callSequence}`;
    },
    nextMessageId() {
      messageSequence += 1;
      return `msg_mock_${messageSequence}`;
    },
    resetSequences() {
      callSequence = 0;
      messageSequence = 0;
    },
  };
}

function matchesOwner(
  record:
    | Pick<CallPayload, "workspace_id" | "profile_name">
    | Pick<CallMessagePayload, "workspace_id" | "profile_name">,
  workspaceId: string,
  params: URLSearchParams
): boolean {
  if ((record.workspace_id ?? "") !== workspaceId) return false;
  if (params.get("all_profiles") === "true") return true;
  const profile = params.get("profile") ?? "default";
  return record.profile_name === profile;
}
