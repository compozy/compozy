/**
 * The Call compose flow, plus what this definition has been asked lately.
 *
 * Both reads are scoped by the daemon's `agent` filter, so "recent calls for
 * reviewer" and "how many reviewers are working" are server-side questions with
 * server-side answers — not a workspace page narrowed in the browser.
 *
 * The instance count deliberately reports `queued,running` rather than "sessions
 * that exist": a parked child is not an instance doing work, and counting it
 * would make an idle roster look busy.
 */
import { useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";

import { agentCommsErrorCode, isAgentCommsApiError } from "../adapters/agent-comms-api";
import { callCreateFailureCopy } from "../lib/call-failure-copy";
import { parseExpectDraft } from "../lib/expect-draft";
import { callsListOptions } from "../lib/query-options";
import { useAgentCommsScope } from "./use-agent-comms-scope";
import { useCallCount } from "./use-call-counts";
import { useCallMutations } from "./use-call-mutations";
import type { CallPayload } from "../types";

/** Rows on the agent-detail "recent calls" list. */
const RECENT_CALLS_LIMIT = 10;

const ACTIVE_STATES = "queued,running";

export interface UseAgentCallComposeOptions {
  live?: boolean;
  /**
   * Continue this exact child instead of starting a fresh one.
   *
   * Call again on a terminal call with a living child aims here, so the helper
   * revives holding everything it already worked out. An expired or pruned child
   * leaves this null and the call targets the definition instead.
   */
  targetSessionId?: string | null;
  /**
   * Seed text — the exact prompt of the call being repeated.
   *
   * Arrives asynchronously, since the full ask is its own fetch. It seeds once
   * and never overwrites what the operator has since typed.
   */
  initialPrompt?: string;
  /** The prior call had a contract that cannot be reconstructed from its digest. */
  contractRequired?: boolean;
  /**
   * Recent-calls list and active-instance count. Agent detail needs them;
   * call-again does not.
   */
  roster?: boolean;
}

export interface AgentCallComposeModel {
  prompt: string;
  setPrompt: (next: string) => void;
  expect: string;
  setExpect: (next: string) => void;
  submit: () => void;
  pending: boolean;
  failure: { code: string; message: string } | null;
  accepted: { callId: string; childSessionId: string | null } | null;
  /** Calls made to this definition lately, newest first. */
  recentCalls: CallPayload[];
  recentCallsPending: boolean;
  recentCallsError: string | null;
  /** Helpers of this definition working right now. Undefined until counted. */
  activeInstances: number | undefined;
}

export function useAgentCallCompose(
  agentName: string,
  options: UseAgentCallComposeOptions = {}
): AgentCallComposeModel {
  const {
    live = false,
    targetSessionId = null,
    initialPrompt,
    contractRequired = false,
    roster = true,
  } = options;
  const scope = useAgentCommsScope();
  const mutations = useCallMutations(scope);
  const [prompt, setPrompt] = useState("");
  const [expect, setExpect] = useState("");
  const [localFailure, setLocalFailure] = useState<{ code: string; message: string } | null>(null);
  const [seededFrom, setSeededFrom] = useState<string | undefined>(undefined);

  // Adjusting state while rendering, which is what React prescribes for "a prop
  // changed and derived state must follow". The guard is the seed value itself,
  // so a later edit by the operator is never clobbered by the same fetch.
  if (initialPrompt !== undefined && seededFrom !== initialPrompt) {
    setSeededFrom(initialPrompt);
    setPrompt(initialPrompt);
  }

  const rosterEnabled = roster && agentName !== "";
  const recent = useInfiniteQuery(
    callsListOptions(scope, { agent: agentName, limit: RECENT_CALLS_LIMIT }, live, rosterEnabled)
  );
  const activeInstances = useCallCount(
    scope,
    { agent: agentName, state: ACTIVE_STATES },
    { live, enabled: rosterEnabled }
  );

  const remoteFailure = (() => {
    const error = mutations.create.error;
    if (!error) return null;
    const raw = isAgentCommsApiError(error) ? error.code : null;
    return {
      code: raw ?? "call_rejected",
      message: callCreateFailureCopy(agentCommsErrorCode(error)),
    };
  })();

  // `createCall` already narrowed the response to a single acceptance, so this
  // reads it directly rather than re-asserting the shape.
  const accepted = mutations.create.data
    ? {
        callId: mutations.create.data.call_id,
        childSessionId: mutations.create.data.child_session_id ?? null,
      }
    : null;

  return {
    prompt,
    setPrompt: (next: string) => {
      setLocalFailure(null);
      mutations.create.reset();
      setPrompt(next);
    },
    expect,
    setExpect: (next: string) => {
      setLocalFailure(null);
      mutations.create.reset();
      setExpect(next);
    },
    submit: () => {
      const parsed = parseExpectDraft(expect);
      if (!parsed.ok) {
        // Caught locally because the operator can see the caret; anything that
        // parses goes to the daemon, which owns whether the shape is usable.
        setLocalFailure({ code: "call_expect_invalid", message: parsed.message });
        return;
      }
      if (contractRequired && parsed.value === undefined) {
        // Never submit a call the operator believes is checked when it is not.
        setLocalFailure({
          code: "call_expect_required",
          message:
            "The earlier call checked its answer. Write the contract again, or the answer comes back unchecked.",
        });
        return;
      }
      setLocalFailure(null);
      mutations.create.mutate({
        // A living child is addressed by session so it revives with its context;
        // otherwise the definition, which starts a helper that knows nothing.
        target: targetSessionId ? { session_id: targetSessionId } : { agent: agentName },
        prompt,
        ...(parsed.value === undefined ? {} : { expect: parsed.value }),
      });
    },
    pending: mutations.create.isPending,
    failure: localFailure ?? remoteFailure,
    accepted,
    recentCalls: (recent.data?.pages ?? []).flatMap(page => page.items),
    recentCallsPending: recent.isPending,
    recentCallsError: recent.isError
      ? recent.error instanceof Error
        ? recent.error.message
        : "The calls request failed."
      : null,
    activeInstances,
  };
}
