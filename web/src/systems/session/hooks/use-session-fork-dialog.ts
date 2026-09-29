import { useState } from "react";

import type { ForkSessionRequest } from "../adapters/session-derive-api";
import { SessionApiError } from "../adapters/session-api-errors";
import type { SessionForkPoint } from "../contexts/session-fork-context-value";
import { sessionForkPointQuote } from "../lib/session-derive-view";
import type { SessionPayload } from "../types";
import {
  landDerivedSession,
  type SessionDerivePlacement,
  type SessionDerivePlacementHandlers,
  useSessionDeriveIdempotencyKey,
  useSessionDerivePreview,
  useSessionFork,
} from "./use-session-derive";

export const SESSION_FORK_TURN_UNSETTLED = "That turn hasn't settled yet.";
export const SESSION_FORK_FENCE_CONFLICT =
  "Transcript changed — reopen to fork from the current state.";

const CHILD_DELETED_MESSAGE =
  "The session this request already created was deleted. Nothing new was created.";

/** Why the last submit was refused: the two drawn refusals, or the daemon's own words. */
type ForkRefusal =
  | { kind: "turn_unsettled" }
  | { kind: "fence_conflict" }
  | { kind: "message"; text: string };

function refusalText(refusal: ForkRefusal | null): string | null {
  if (refusal?.kind === "fence_conflict") return SESSION_FORK_FENCE_CONFLICT;
  if (refusal?.kind === "message") return refusal.text;
  return null;
}

function forkRefusal(error: Error): ForkRefusal {
  if (error instanceof SessionApiError) {
    if (error.code === "session_turn_in_progress") return { kind: "turn_unsettled" };
    if (error.code === "session_fence_conflict") return { kind: "fence_conflict" };
  }
  return { kind: "message", text: error.message };
}

export interface UseSessionForkDialogInput {
  source: SessionPayload;
  workspaceId: string;
  open: boolean;
  /** Fork from here; `null` forks the whole session. */
  point: SessionForkPoint | null;
  onClose: () => void;
  placement: SessionDerivePlacementHandlers;
}

/**
 * Fork-dialog state: the locked agent, the fork point, the measured preview
 * (with the daemon's cut and native decision), where the child opens, and the
 * submit. The transcript fences the preview was read at go with the request,
 * so the child is forked from what the dialog showed or refused as changed.
 */
export function useSessionForkDialog({
  source,
  workspaceId,
  open,
  point,
  onClose,
  placement: handlers,
}: UseSessionForkDialogInput) {
  const [placement, setPlacement] = useState<SessionDerivePlacement>("new-window");
  const [refusal, setRefusal] = useState<ForkRefusal | null>(null);
  const idempotency = useSessionDeriveIdempotencyKey();
  const messageId = point?.messageId.trim() ?? "";
  const preview = useSessionDerivePreview(workspaceId, source.id, {
    enabled: open,
    ...(messageId ? { messageId } : {}),
  });
  const mutation = useSessionFork();

  const measured = preview.view.state === "ready" || preview.view.state === "truncated";
  const cutUnsettled =
    preview.preview?.cut?.turn_settled === false || refusal?.kind === "turn_unsettled";
  const fenceConflict = refusal?.kind === "fence_conflict";
  const isSubmitting = mutation.isPending;
  const canSubmit = !isSubmitting && measured && !cutUnsettled && !fenceConflict;
  // Only the whole-session point can clone natively; the daemon already says
  // false for a cut, the dialog does not second-guess either way.
  const nativeClone = measured && preview.preview?.native_fork_possible === true;

  const submit = () => {
    const measuredPreview = preview.preview;
    if (!canSubmit || !measuredPreview) return;
    // A running source moves its fences with every event; the snapshot already
    // leaves the in-flight turn out, so pinning them would refuse every submit.
    const fences = measuredPreview.source_turn_in_progress
      ? {}
      : {
          expected_epoch: measuredPreview.transcript.epoch,
          expected_generation: measuredPreview.transcript.generation,
          expected_max_sequence: measuredPreview.transcript.max_sequence,
        };
    const request: ForkSessionRequest = {
      idempotency_key: idempotency.key,
      ...(messageId ? { message_id: messageId } : {}),
      ...fences,
    };
    setRefusal(null);
    mutation.mutate(
      { workspaceId, sourceId: source.id, request },
      {
        onSuccess: result => {
          if (result.derived.child_deleted || !result.session) {
            setRefusal({ kind: "message", text: CHILD_DELETED_MESSAGE });
            return;
          }
          onClose();
          landDerivedSession(result, placement, handlers);
        },
        onError: error => {
          idempotency.settleFailure(error);
          setRefusal(forkRefusal(error));
        },
      }
    );
  };

  return {
    agentName: source.agent_name.trim(),
    provider: source.runtime.effective?.provider.trim() ?? "",
    pointQuote: point ? sessionForkPointQuote(point.messageText) : null,
    previewView: preview.view,
    nativeClone,
    previewRefusal: cutUnsettled ? SESSION_FORK_TURN_UNSETTLED : null,
    submitError: refusalText(refusal),
    fenceConflict,
    placement,
    onPlacementChange: setPlacement,
    canChoosePlacement: handlers.openInThisWindow !== undefined,
    isSubmitting,
    canSubmit,
    submit,
  };
}

export type SessionForkDialogModel = ReturnType<typeof useSessionForkDialog>;
