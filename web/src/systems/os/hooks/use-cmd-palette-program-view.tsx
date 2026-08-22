import { useEffect, useEffectEvent, useState } from "react";
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
import type {
  CmdPaletteViewAction,
  CmdPaletteViewEnvelope,
  CmdPaletteViewFrame,
  CmdPaletteViewSessionEvent,
} from "../lib/cmd-palette-types";
import type { CmdPaletteEventSourceFactory } from "../lib/cmd-palette-stream";
import {
  acknowledgeViewEffects,
  executeCmdPaletteViewEffect,
  programViewEnvelope,
  runSerializedViewAction,
  viewErrorMessage,
} from "../lib/cmd-palette-program-view-runtime";
import type { PaletteViewContent } from "../lib/palette-view-registry";
import {
  createCmdPaletteViewProgramLogic,
  programHandlerIsLive,
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
import { useProfileReadScope } from "@/systems/profiles";

interface ViewSessionIdentity {
  readonly epoch: number;
  readonly streamToken: string;
  readonly viewSession: string;
  /** The profile lens the daemon resolved for this session, carried verbatim. */
  readonly profileLens: CmdPaletteViewEnvelope["profile_lens"];
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
  const [programLogic] = useState(() =>
    createCmdPaletteViewProgramLogic({ query, catalogRevision: registry.catalogRevision })
  );
  const store = useStore(programLogic);
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
  const attachmentToken = client?.attachmentToken ?? "";
  const programChrome = state.payload?.chrome;
  // The view-session identity gains the profile lens: a switch changes this
  // string, which aborts the open, closes the session daemon-side, and reopens
  // under the new lens — teardown and re-scope in one move (ADR-016).
  const { destination, key: profileKey } = useProfileReadScope();
  const fallbackKey = `${runtimeWorkspaceId ?? ""}\u0000${profileKey}\u0000${viewId}\u0000${attachmentToken}`;
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
    void openCmdPaletteViewSession(
      workspace,
      destination,
      viewId,
      attachmentToken,
      {},
      controller.signal
    )
      .then(response => {
        if (controller.signal.aborted) return;
        opened = {
          epoch,
          streamToken: response.stream_token,
          viewSession: response.view_session,
          profileLens: response.profile_lens,
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
    // `fallbackKey` carries the lens, so a switch aborts this open, closes the
    // session daemon-side, and reopens under the new owner.
  }, [
    attachmentToken,
    destination,
    fallbackKey,
    runtimeWorkspaceId,
    state.openEpoch,
    store,
    viewId,
  ]);

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
    if (!declarative) {
      store.trigger.catalogRevisionObserved({ revision: registry.catalogRevision });
    }
  }, [declarative, registry.catalogRevision, store]);

  const sendEvent = (handler: string, args: readonly unknown[], controlled: boolean): void => {
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
    const effectResult = current.pendingEffectResults[0];
    store.trigger.eventSent({
      seq,
      controlled,
      handler,
      args,
      effectResultConsumed: effectResult !== undefined,
    });
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
      if (effectResult) store.trigger.effectResultRestored({ result: effectResult });
      if (error instanceof CmdPaletteApiError && (error.status === 403 || error.status === 410)) {
        store.trigger.crashed({ error: error.message });
        return;
      }
      console.warn("Command palette view event was rejected", error);
    });
  };

  const sendSearchEvent = useEffectEvent(sendEvent);

  useEffect(() => {
    const handler = programChrome?.on_search;
    store.trigger.searchObserved({
      handler: handler ?? null,
      query,
      throttleMs: Math.max(0, programChrome?.throttle_ms ?? 0),
    });
  }, [programChrome?.on_search, programChrome?.throttle_ms, query, store]);

  useEffect(() => {
    const subscription = store.on("searchRequested", event => {
      sendSearchEvent(event.handler, [event.query], true);
    });
    return () => subscription.unsubscribe();
  }, [store]);

  useEffect(() => {
    const searchText = programChrome?.search_text;
    if (
      searchText === undefined ||
      !controlledEchoIsCurrent(programChrome?.event_count, state.eventCount) ||
      state.searchQuery === (searchText ?? "")
    ) {
      return;
    }
    const nextQuery = searchText ?? "";
    store.trigger.searchEchoed({ query: nextQuery });
    onQueryChange(nextQuery);
  }, [onQueryChange, programChrome, state.eventCount, state.searchQuery, store]);

  useEffect(() => {
    const executedEffects = new Set(state.executedEffects);
    const pending = (state.frame?.effects ?? []).filter(effect => !executedEffects.has(effect.id));
    if (pending.length === 0) return;
    store.trigger.effectsExecutionStarted({ ids: pending.map(effect => effect.id) });
    const effectResults: NonNullable<CmdPaletteViewSessionEvent["effect_result"]>[] = [];
    void Promise.allSettled(
      pending.map(effect => executeCmdPaletteViewEffect(effect, dispatch, viewId, effectResults))
    ).then(results => {
      store.trigger.effectsAcknowledged({
        ids: acknowledgeViewEffects(pending, results),
        results: effectResults,
      });
    });
  }, [dispatch, state.executedEffects, state.frame?.effects, store, viewId]);

  useEffect(
    () => () => {
      store.trigger.closed({});
    },
    [store]
  );

  const title =
    registry.commands.find(
      command => command.action.kind === "view" && command.action.view === viewId
    )?.title ?? viewId;
  const envelope =
    state.payload && session
      ? programViewEnvelope(viewId, title, state.frame!, state.payload, session.profileLens)
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
