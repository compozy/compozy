import { useEffect, useEffectEvent, useRef, useState } from "react";
import type { RefObject } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { createStreamEventSource } from "@/lib/ticketed-event-source";
import { useActiveWorkspace } from "@/systems/workspace";

import {
  admitCmdPaletteViewSessionEvent,
  closeCmdPaletteViewSession,
  cmdPaletteViewSessionStreamURL,
  CmdPaletteApiError,
  openCmdPaletteViewSession,
} from "../adapters/cmd-palette-api";
import { PaletteConfirmation } from "../components/os-palette-confirmation";
import { programViewContentForPhase } from "../lib/cmd-palette-program-content";
import type { WindowManagerRegisteredClientView } from "../lib/window-manager-types";
import type { CmdPaletteViewAction, CmdPaletteViewFrame } from "../lib/cmd-palette-types";
import type { CmdPaletteEventSourceFactory } from "../lib/cmd-palette-stream";
import {
  acknowledgeViewEffects,
  claimPendingEffectResult,
  executeCmdPaletteViewEffect,
  programViewEnvelope,
  restorePendingEffectResult,
  runSerializedViewAction,
  viewErrorMessage,
  type PendingEffectResult,
} from "../lib/cmd-palette-program-view-runtime";
import type { PaletteViewContent } from "../lib/palette-view-registry";
import {
  cmdPaletteViewProgramLogic,
  programHandlerIsLive,
  VIEW_BUSY_BUDGET_MS,
  VIEW_DEGRADED_BUDGET_MS,
} from "../stores/cmd-palette-view-program-store";
import {
  contentForEnvelope,
  emptyFrame,
  hostFiltersLocally,
  viewActionCommandID,
  viewDefinition,
  type CmdPaletteDeclarativeViewModel,
} from "./use-cmd-palette-declarative-view";
import type { CmdPaletteDispatch } from "./use-cmd-palette-dispatch";
import { usePaletteRegistry } from "./use-palette-registry";

interface ViewSessionIdentity {
  readonly epoch: number;
  readonly streamToken: string;
  readonly viewSession: string;
}

export interface CmdPaletteProgramViewModel extends CmdPaletteDeclarativeViewModel {
  readonly declarative: boolean;
}

export function useCmdPaletteProgramView({
  client,
  dispatch,
  eventSourceFactory,
  onDismiss,
  onQueryChange,
  query,
  viewId,
}: {
  client: WindowManagerRegisteredClientView | null;
  dispatch: CmdPaletteDispatch;
  eventSourceFactory?: CmdPaletteEventSourceFactory;
  onDismiss: () => void;
  onQueryChange: (query: string) => void;
  query: string;
  viewId: string;
}): CmdPaletteProgramViewModel {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const registry = usePaletteRegistry();
  const store = useStore(cmdPaletteViewProgramLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const [session, setSession] = useState<ViewSessionIdentity | null>(null);
  const [declarativeKey, setDeclarativeKey] = useState<string | null>(null);
  const [localChip, setLocalChip] = useState<{
    eventCount: number;
    value: string | null;
    viewId: string;
  } | null>(null);
  const [selectedRow, setSelectedRow] = useState("");
  const [pendingConfirm, setPendingConfirm] = useState<{
    action: CmdPaletteViewAction;
    values: Readonly<Record<string, unknown>>;
  } | null>(null);
  const queryRef = useRef(query);
  const catalogRevisionRef = useRef(registry.catalogRevision);
  const timersRef = useRef<{
    hard: ReturnType<typeof setTimeout>;
    soft: ReturnType<typeof setTimeout>;
  } | null>(null);
  const executedEffectsRef = useRef(new Set<string>());
  const effectResultsRef = useRef<PendingEffectResult[]>([]);
  const attachmentToken = client?.attachmentToken ?? "";
  const programChrome = state.payload?.chrome;
  const fallbackKey = `${runtimeWorkspaceId ?? ""}\u0000${viewId}\u0000${attachmentToken}`;
  const declarative = declarativeKey === fallbackKey;

  useEffect(() => {
    const workspace = runtimeWorkspaceId;
    store.trigger.openStarted({ preserve: store.getSnapshot().context.payload !== null });
    const epoch = store.getSnapshot().context.openEpoch;
    if (!workspace || attachmentToken === "") {
      store.trigger.openFailed({ error: "This browser is not attached to the workspace." });
      return undefined;
    }
    const controller = new AbortController();
    let opened: ViewSessionIdentity | null = null;
    void openCmdPaletteViewSession(workspace, viewId, attachmentToken, {}, controller.signal)
      .then(response => {
        if (controller.signal.aborted) return;
        opened = {
          epoch,
          streamToken: response.stream_token,
          viewSession: response.view_session,
        };
        setSession(opened);
        store.trigger.openSucceeded({ frame: response.first_frame });
      })
      .catch(error => {
        if (controller.signal.aborted) return;
        if (error instanceof CmdPaletteApiError && error.status === 422) {
          setDeclarativeKey(fallbackKey);
          return;
        }
        store.trigger.openFailed({ error: viewErrorMessage(error) });
      });
    return () => {
      controller.abort();
      setSession(null);
      const current = opened;
      if (current) {
        void closeCmdPaletteViewSession(current.viewSession, attachmentToken).catch(error => {
          console.warn("Failed to close command palette view session", error);
        });
      }
    };
  }, [attachmentToken, fallbackKey, runtimeWorkspaceId, state.openEpoch, store, viewId]);

  useEffect(() => {
    if (!session || session.epoch !== state.openEpoch || declarative) return undefined;
    const source = (eventSourceFactory ?? createStreamEventSource)(
      cmdPaletteViewSessionStreamURL(session.viewSession, session.streamToken)
    );
    const identity = session;
    const onFrame = (event: Event) => {
      if (!(event instanceof MessageEvent)) return;
      const current = store.getSnapshot().context;
      if (identity.epoch !== current.openEpoch) return;
      try {
        const frame = JSON.parse(event.data) as CmdPaletteViewFrame;
        if (frame.view_session !== identity.viewSession) return;
        store.trigger.frameReceived({ frame });
      } catch (error) {
        if (identity.epoch !== store.getSnapshot().context.openEpoch) return;
        store.trigger.crashed({ error: viewErrorMessage(error) });
      }
    };
    source.addEventListener("cmd_palette.view.frame", onFrame);
    source.onerror = () => {
      if (identity.epoch !== store.getSnapshot().context.openEpoch) return;
      store.trigger.crashed({ error: "The extension process stopped responding." });
    };
    return () => {
      source.removeEventListener("cmd_palette.view.frame", onFrame);
      source.close();
    };
  }, [declarative, eventSourceFactory, session, state.openEpoch, store]);

  useEffect(() => {
    if (catalogRevisionRef.current === registry.catalogRevision) return;
    catalogRevisionRef.current = registry.catalogRevision;
    if (!declarative) store.trigger.reloadRequested({});
  }, [declarative, registry.catalogRevision, store]);

  useEffect(() => {
    if (state.pendingSeq !== null) return;
    clearViewTimers(timersRef);
  }, [state.pendingSeq]);

  const sendEvent = useEffectEvent(
    (handler: string, args: readonly unknown[], controlled: boolean): void => {
      const current = store.getSnapshot().context;
      if (
        !session ||
        session.epoch !== current.openEpoch ||
        attachmentToken === "" ||
        !current.frame ||
        !programHandlerIsLive(current, handler)
      ) {
        return;
      }
      const seq = current.nextSeq + 1;
      const eventCount = controlled ? current.eventCount + 1 : current.eventCount;
      const resolvedArgs = controlled ? [...args, eventCount] : [...args];
      const effectResult = claimPendingEffectResult(effectResultsRef.current);
      store.trigger.eventSent({ seq, controlled, handler, args });
      clearViewTimers(timersRef);
      timersRef.current = {
        soft: setTimeout(() => store.trigger.softBudgetElapsed({ seq }), VIEW_BUSY_BUDGET_MS),
        hard: setTimeout(() => store.trigger.hardBudgetElapsed({ seq }), VIEW_DEGRADED_BUDGET_MS),
      };
      void admitCmdPaletteViewSessionEvent(session.viewSession, attachmentToken, {
        handler,
        args: resolvedArgs,
        revision: current.frame.revision,
        seq,
        ...(current.acknowledgedEffects.length > 0
          ? { ack_effects: [...current.acknowledgedEffects] }
          : {}),
        ...(effectResult ? { effect_result: effectResult } : {}),
      }).catch(error => {
        restorePendingEffectResult(effectResultsRef.current, effectResult);
        if (error instanceof CmdPaletteApiError && (error.status === 403 || error.status === 410)) {
          store.trigger.crashed({ error: error.message });
          return;
        }
        console.warn("Command palette view event was rejected", error);
      });
    }
  );

  useEffect(() => {
    const handler = programChrome?.on_search;
    if (!handler || queryRef.current === query) return;
    queryRef.current = query;
    const throttle = Math.max(0, programChrome?.throttle_ms ?? 0);
    if (throttle === 0) {
      sendEvent(handler, [query], true);
      return;
    }
    const timer = window.setTimeout(() => sendEvent(handler, [query], true), throttle);
    return () => window.clearTimeout(timer);
  }, [programChrome, query]);

  useEffect(() => {
    const searchText = programChrome?.search_text;
    if (
      searchText === undefined ||
      !controlledEchoIsCurrent(programChrome?.event_count, state.eventCount) ||
      queryRef.current === (searchText ?? "")
    ) {
      return;
    }
    const nextQuery = searchText ?? "";
    queryRef.current = nextQuery;
    onQueryChange(nextQuery);
  }, [onQueryChange, programChrome, state.eventCount]);

  const executeEffect = useEffectEvent(
    async (effect: Parameters<typeof executeCmdPaletteViewEffect>[0]): Promise<void> => {
      await executeCmdPaletteViewEffect(effect, dispatch, viewId, effectResultsRef.current);
    }
  );

  useEffect(() => {
    const pending = (state.frame?.effects ?? []).filter(
      effect => !executedEffectsRef.current.has(effect.id)
    );
    if (pending.length === 0) return;
    for (const effect of pending) executedEffectsRef.current.add(effect.id);
    void Promise.allSettled(pending.map(executeEffect)).then(results => {
      store.trigger.effectsAcknowledged({ ids: acknowledgeViewEffects(pending, results) });
    });
  }, [state.frame?.effects, store]);

  useEffect(
    () => () => {
      clearViewTimers(timersRef);
      store.trigger.closed({});
    },
    [store]
  );

  const title =
    registry.commands.find(
      command => command.action.kind === "view" && command.action.view === viewId
    )?.title ?? viewId;
  const envelope = state.payload
    ? programViewEnvelope(viewId, title, state.frame!, state.payload)
    : null;
  const retry = () => {
    if (state.phase === "unavailable") {
      store.trigger.reopenRequested({});
      return;
    }
    store.trigger.retryRequested({});
    const last = state.lastEvent;
    if (last) sendEvent(last.handler, last.args, last.controlled);
  };
  const echoedChip = programChrome?.active_chip;
  const activeChip =
    echoedChip !== undefined &&
    controlledEchoIsCurrent(
      programChrome?.event_count,
      Math.max(state.eventCount, localChip?.viewId === viewId ? localChip.eventCount : 0)
    )
      ? echoedChip
      : localChip?.viewId === viewId
        ? localChip.value
        : null;
  const executeAction = async (
    action: CmdPaletteViewAction,
    values: Readonly<Record<string, unknown>>,
    confirmed = false
  ) => {
    if (action.handler) {
      sendEvent(action.handler, Object.keys(values).length === 0 ? [] : [values], false);
      return;
    }
    await runSerializedViewAction(dispatch, viewId, action, values, onDismiss, registry, confirmed);
  };
  const runAction = async (
    action: CmdPaletteViewAction | undefined,
    values: Readonly<Record<string, unknown>> = {}
  ) => {
    if (!action) return;
    const cataloged = registry.byId.has(viewActionCommandID(viewId, action));
    if (action.confirmation && (action.handler || !cataloged)) {
      setPendingConfirm({ action, values });
      return;
    }
    await executeAction(action, values);
  };
  const setChip = (id: string | null) => {
    const handler = programChrome?.on_chip;
    setLocalChip({ eventCount: state.eventCount + (handler ? 1 : 0), value: id, viewId });
    if (handler) sendEvent(handler, [id], true);
  };
  const setSelection = (id: string) => {
    setSelectedRow(id);
    const handler = state.payload?.chrome?.on_selection;
    if (handler) sendEvent(handler, [id], true);
  };
  const baseContent = envelope
    ? contentForEnvelope({
        activeChip,
        envelope,
        query,
        runAction,
        selectedRow,
        setActiveChip: setChip,
        setSelectedRow: setSelection,
        filterLocally: hostFiltersLocally(envelope.payload.chrome),
        runHandler: sendEvent,
      })
    : emptyFrame("Opening view…");
  return {
    declarative,
    definition: viewDefinition(viewId, envelope ?? undefined),
    content: withProgramConfirm(
      programViewContentForPhase(
        baseContent,
        state.phase,
        state.reloaded,
        title,
        viewId,
        state.error,
        retry
      ),
      pendingConfirm,
      {
        onCancel: () => setPendingConfirm(null),
        onConfirm: () => {
          const pending = pendingConfirm;
          setPendingConfirm(null);
          if (pending) void executeAction(pending.action, pending.values, true);
        },
      }
    ),
    error: state.phase === "unavailable" ? state.error : null,
    loading: state.phase === "opening",
    timedOut: state.phase === "degraded" || state.phase === "circuit-open",
    retry,
  };
}

export function controlledEchoIsCurrent(
  echoedEventCount: number | undefined,
  localEventCount: number
): boolean {
  return (echoedEventCount ?? 0) >= localEventCount;
}

export { hostFiltersLocally };

function withProgramConfirm(
  content: PaletteViewContent,
  pending: { action: CmdPaletteViewAction; values: Readonly<Record<string, unknown>> } | null,
  handlers: { onCancel: () => void; onConfirm: () => void }
): PaletteViewContent {
  const confirmation = pending?.action.confirmation;
  if (!confirmation) return content;
  return {
    ...content,
    header: (
      <>
        <PaletteConfirmation
          confirmation={confirmation}
          destructive={pending.action.destructive ?? false}
          invalidatedReason=""
          onCancel={handlers.onCancel}
          onConfirm={handlers.onConfirm}
        />
        {content.header}
      </>
    ),
  };
}

function clearViewTimers(
  ref: RefObject<{
    hard: ReturnType<typeof setTimeout>;
    soft: ReturnType<typeof setTimeout>;
  } | null>
): void {
  if (!ref.current) return;
  clearTimeout(ref.current.soft);
  clearTimeout(ref.current.hard);
  ref.current = null;
}
