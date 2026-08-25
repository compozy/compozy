/**
 * View model for the call-detail location.
 *
 * Three things happen here that the system layer deliberately does not do:
 *
 * - **Cost is read from the child session.** A call has no cost field and needs
 *   none — its work happened in the child, and wake-consuming deliveries account
 *   on the same owner-keyed substrate as every other activation. One set of
 *   books, so the rail reads the child's usage.
 * - **The counterpart is checked.** Retention prunes sessions while call records
 *   are kept indefinitely, so an old call can name a child that no longer
 *   exists. Asking first is what lets the jump link degrade honestly instead of
 *   dangling. The check can only run once the record names the child, so the
 *   view is rebuilt here with the answer rather than re-running the call query.
 * - **Call again and Message child are composed, not fired.** Neither is a
 *   one-click repeat: calling again means deciding whether to revive the same
 *   helper or start a fresh one, and repeating the ask verbatim means fetching
 *   the whole prompt rather than reusing a bounded preview. Both open an inline
 *   compose that states what it is about to do.
 */
import { useState } from "react";

import { useNavigate } from "@tanstack/react-router";

import {
  buildCallDetailView,
  isAgentCommsApiError,
  useAgentCallCompose,
  useCallDetail,
  useCallMutations,
  type AgentCallTarget,
} from "@/systems/agent-comms";
import { useSession, useSessionUsage } from "@/systems/session";

import { useWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";

export function useAgentCall(callId: string, windowId: string) {
  const live = useWindowLiveDataEnabled(windowId);
  const navigate = useNavigate({ from: "/agents" });
  const [composing, setComposing] = useState<"call-again" | "message" | null>(null);
  const [messageText, setMessageText] = useState("");

  const detail = useCallDetail(callId, { live });
  const call = detail.call;
  const childSessionId = call?.child_session_id ?? "";
  const childSession = useSession(childSessionId);
  const childUsage = useSessionUsage(childSessionId, null, null, {
    enabled: childSessionId !== "",
  });

  // Undefined while the lookup is in flight, so the link stays offered rather
  // than flickering away and back.
  const counterpartExists =
    childSessionId === "" ? true : childSession.isPending ? undefined : !childSession.isError;

  const view = call
    ? buildCallDetailView({
        call,
        ...(counterpartExists === undefined ? {} : { counterpartExists }),
      })
    : null;

  const mutations = useCallMutations(detail.scope);

  // A living child is revived by name; an expired or pruned one is gone, so the
  // repeat targets the definition and the compose says so.
  const childReachable = Boolean(view?.controls.messageChild) && counterpartExists !== false;
  const agentName = view?.agentName ?? "";
  const callAgainTarget: AgentCallTarget =
    childReachable && childSessionId !== ""
      ? { kind: "session", sessionId: childSessionId, agentName }
      : { kind: "agent" };

  const compose = useAgentCallCompose(agentName, {
    live: false,
    targetSessionId: callAgainTarget.kind === "session" ? childSessionId : null,
    // Seeds only once the whole ask has been fetched. Until then the box stays
    // empty rather than holding a truncated preview that would be sent verbatim.
    ...(detail.fullPrompt === undefined ? {} : { initialPrompt: detail.fullPrompt }),
    contractRequired: Boolean(view?.expectDigest),
  });

  return {
    ...detail,
    view,
    childUsage: {
      status: childUsage.data?.cost_status ?? null,
      source: childUsage.data?.cost_source ?? null,
      amount: childUsage.data?.total_cost ?? null,
      currency: childUsage.data?.cost_currency ?? null,
    },
    cancel: () => {
      mutations.cancel.mutate({ callId, reason: "canceled from the call record" });
    },
    cancelPending: mutations.cancel.isPending,
    // Every cancel answers, and every answer is said out loud. The daemon's
    // reply is idempotent and always carries the real terminal state, so a
    // cancel that lost a race is not an error — it is a different outcome than
    // the operator expected, and naming it is what US-005 EC-1 asks for.
    // Reporting only the surprising ones would leave an ordinary cancel silent,
    // which is the failure US-028 EC-4 forbids.
    cancelOutcome: mutations.cancel.data
      ? { state: mutations.cancel.data.state, stale: mutations.cancel.data.state !== "canceled" }
      : null,

    composing,
    callAgain: () => {
      setComposing("call-again");
      // The repeat has to carry the exact ask, and the record only holds a
      // bounded preview of it.
      detail.fetchFullPrompt();
    },
    callAgainTarget,
    compose,
    closeCompose: () => setComposing(null),

    messageChild: () => setComposing("message"),
    messageText,
    setMessageText: (next: string) => {
      mutations.message.reset();
      setMessageText(next);
    },
    sendMessage: () => {
      if (childSessionId === "" || messageText.trim() === "") return;
      mutations.message.mutate({
        invalidateCallId: callId,
        body: {
          call_id: callId,
          to: { session_id: childSessionId },
          text: messageText,
        },
      });
    },
    messagePending: mutations.message.isPending,
    // Read through the typed guard, not a cast: the compose prints this code
    // verbatim, so a refusal the adapter did not actually classify must surface
    // as "no code" rather than as whatever happened to be on the error object.
    messageFailureCode: isAgentCommsApiError(mutations.message.error)
      ? mutations.message.error.code
      : null,
    messageAccepted: mutations.message.data
      ? {
          messageId: mutations.message.data.message_id,
          delivery: mutations.message.data.delivery,
        }
      : null,

    openCaller: () => {
      const callerId = call?.caller.id;
      if (!callerId) return;
      void navigate({ to: "/session/$id", params: { id: callerId } });
    },
    openChildSession: () => {
      if (!childSessionId) return;
      void navigate({ to: "/session/$id", params: { id: childSessionId } });
    },
    openCall: (nextCallId: string) => {
      void navigate({ to: "/agents/calls/$callId", params: { callId: nextCallId } });
    },
    openActivity: () => {
      void navigate({ to: "/agents/activity" });
    },
  };
}
