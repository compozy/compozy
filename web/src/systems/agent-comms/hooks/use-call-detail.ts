/**
 * One call's view model, plus its three unbounded payloads on demand.
 *
 * None of the three is fetched eagerly. List and detail already carry bounded
 * previews, and a payload can run to megabytes — so each fetch is an explicit
 * act, enabled only once someone asks for it, and its cache entry lives forever
 * afterwards because a settled call's payloads cannot change.
 *
 * The prompt has a second caller besides the disclosure: Call again has to
 * repeat the *exact* ask, and a truncated preview is not that.
 */
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { buildCallDetailView, type CallDetailView } from "../lib/call-detail-view-model";
import {
  callDetailOptions,
  callPromptOptions,
  callResultOptions,
  callSupersededOptions,
} from "../lib/query-options";
import { useAgentCommsScope } from "./use-agent-comms-scope";
import type { AgentCommsScope } from "../lib/agent-comms-scope";
import type { CallPayload } from "../types";

export interface UseCallDetailOptions {
  live: boolean;
  /**
   * Whether the child session still resolves. Callers that have not checked
   * leave it undefined, and the jump link stays offered rather than being hidden
   * on a guess.
   */
  counterpartExists?: boolean;
}

export interface CallDetailModel {
  scope: AgentCommsScope;
  /**
   * The raw record.
   *
   * Exposed because whether the child session still resolves can only be learned
   * *after* the record names it — so a caller that wants that check has to read
   * `call.child_session_id`, look it up, and rebuild the view with the answer.
   * Re-invoking this hook to pass the flag would issue the same query twice.
   */
  call: CallPayload | undefined;
  view: CallDetailView | null;
  isPending: boolean;
  error: Error | null;
  /** Undefined until the operator asks for the whole payload. */
  fullPayload: unknown;
  fullPayloadPending: boolean;
  fullPayloadError: Error | null;
  fetchFullPayload: () => void;
  /** The exact ask, once fetched. Call again needs this, not the preview. */
  fullPrompt: string | undefined;
  fullPromptPending: boolean;
  fullPromptError: Error | null;
  fetchFullPrompt: () => void;
  /** Late evidence, fetched only when the bounded preview is not enough. */
  supersededPayload: unknown;
  supersededPending: boolean;
  supersededError: Error | null;
  fetchSuperseded: () => void;
  refetch: () => void;
}

export function useCallDetail(callId: string, options: UseCallDetailOptions): CallDetailModel {
  const scope = useAgentCommsScope();
  const [wantsFullPayload, setWantsFullPayload] = useState(false);
  const [wantsFullPrompt, setWantsFullPrompt] = useState(false);
  const [wantsSuperseded, setWantsSuperseded] = useState(false);

  const detail = useQuery(callDetailOptions(scope, callId, options.live));
  const result = useQuery(callResultOptions(scope, callId, wantsFullPayload));
  const prompt = useQuery(callPromptOptions(scope, callId, wantsFullPrompt));
  const superseded = useQuery(callSupersededOptions(scope, callId, wantsSuperseded));

  return {
    scope,
    call: detail.data,
    view: detail.data
      ? buildCallDetailView({
          call: detail.data,
          ...(options.counterpartExists === undefined
            ? {}
            : { counterpartExists: options.counterpartExists }),
        })
      : null,
    isPending: detail.isPending,
    error: detail.error,
    fullPayload: result.data?.result,
    fullPayloadPending: wantsFullPayload && result.isPending,
    fullPayloadError: result.error,
    fetchFullPayload: () => {
      if (wantsFullPayload) {
        void result.refetch();
      } else {
        setWantsFullPayload(true);
      }
    },
    fullPrompt: prompt.data?.prompt,
    fullPromptPending: wantsFullPrompt && prompt.isPending,
    fullPromptError: prompt.error,
    fetchFullPrompt: () => {
      if (wantsFullPrompt) {
        void prompt.refetch();
      } else {
        setWantsFullPrompt(true);
      }
    },
    supersededPayload: superseded.data?.result,
    supersededPending: wantsSuperseded && superseded.isPending,
    supersededError: superseded.error,
    fetchSuperseded: () => {
      if (wantsSuperseded) {
        void superseded.refetch();
      } else {
        setWantsSuperseded(true);
      }
    },
    refetch: () => {
      void detail.refetch();
    },
  };
}
