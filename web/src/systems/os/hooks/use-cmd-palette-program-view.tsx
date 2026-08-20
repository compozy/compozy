import { useContext, useEffect, useEffectEvent, useRef, useState } from "react";
import type { RefObject } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { createStreamEventSource } from "@/lib/ticketed-event-source";
import { notifyUser, type UserFeedbackTone } from "@/lib/user-feedback";
import { useActiveWorkspace } from "@/systems/workspace";

import {
  admitCmdPaletteViewSessionEvent,
  closeCmdPaletteViewSession,
  cmdPaletteViewSessionStreamURL,
  CmdPaletteApiError,
  openCmdPaletteViewSession,
} from "../adapters/cmd-palette-api";
import {
  OsPaletteProgramBand,
  OsPaletteProgramFailure,
  OsPaletteProgramReloaded,
} from "../components/os-palette-program-status";
import { CmdPaletteRegistryContext } from "../contexts/cmd-palette-registry-context-value";
import type { WindowManagerAttachedClientView } from "../lib/window-manager-types";
import type {
  CmdPaletteViewAction,
  CmdPaletteViewEffect,
  CmdPaletteViewEnvelope,
  CmdPaletteViewFrame,
} from "../lib/cmd-palette-types";
import type { PaletteViewContent } from "../lib/palette-view-registry";
import {
  cmdPaletteViewProgramLogic,
  type CmdPaletteViewProgramPhase,
  programHandlerIsLive,
  VIEW_BUSY_BUDGET_MS,
  VIEW_DEGRADED_BUDGET_MS,
} from "../stores/cmd-palette-view-program-store";
import {
  commandForViewAction,
  contentForEnvelope,
  emptyFrame,
  extensionName,
  viewDefinition,
  type CmdPaletteDeclarativeViewModel,
} from "./use-cmd-palette-declarative-view";
import type { CmdPaletteDispatch } from "./use-cmd-palette-dispatch";

interface ViewSessionIdentity {
  readonly epoch: number;
  readonly streamToken: string;
  readonly viewSession: string;
}

interface PendingEffectResult {
  readonly effect_id: string;
  readonly payload?: unknown;
}

export interface CmdPaletteProgramViewModel extends CmdPaletteDeclarativeViewModel {
  readonly declarative: boolean;
}

export function useCmdPaletteProgramView({
  client,
  dispatch,
  onDismiss,
  query,
  viewId,
}: {
  client: WindowManagerAttachedClientView | null;
  dispatch: CmdPaletteDispatch;
  onDismiss: () => void;
  query: string;
  viewId: string;
}): CmdPaletteProgramViewModel {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const registry = useContext(CmdPaletteRegistryContext);
  const store = useStore(cmdPaletteViewProgramLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const [session, setSession] = useState<ViewSessionIdentity | null>(null);
  const [declarativeKey, setDeclarativeKey] = useState<string | null>(null);
  const [activeChip, setActiveChip] = useState("all");
  const [selectedRow, setSelectedRow] = useState("");
  const queryRef = useRef(query);
  const catalogRevisionRef = useRef(registry.catalogRevision);
  const timersRef = useRef<{
    hard: ReturnType<typeof setTimeout>;
    soft: ReturnType<typeof setTimeout>;
  } | null>(null);
  const executedEffectsRef = useRef(new Set<string>());
  const effectResultsRef = useRef<PendingEffectResult[]>([]);
  const attachmentToken = client?.attachmentToken?.trim() ?? "";
  const fallbackKey = `${runtimeWorkspaceId ?? ""}\u0000${viewId}\u0000${attachmentToken}`;
  const declarative = declarativeKey === fallbackKey;

  useEffect(() => {
    const workspace = runtimeWorkspaceId;
    const activeAttachmentToken = client?.attachmentToken?.trim() ?? "";
    store.trigger.openStarted({ preserve: state.openEpoch > 0 });
    if (!workspace || activeAttachmentToken === "") {
      store.trigger.openFailed({ error: "This browser is not attached to the workspace." });
      return undefined;
    }
    const controller = new AbortController();
    let opened: ViewSessionIdentity | null = null;
    void openCmdPaletteViewSession(workspace, viewId, activeAttachmentToken, {}, controller.signal)
      .then(response => {
        if (controller.signal.aborted) return;
        opened = {
          epoch: state.openEpoch,
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
        store.trigger.openFailed({ error: errorMessage(error) });
      });
    return () => {
      controller.abort();
      const current = opened;
      if (current) {
        void closeCmdPaletteViewSession(current.viewSession, activeAttachmentToken).catch(error => {
          console.warn("Failed to close command palette view session", error);
        });
      }
    };
  }, [client?.attachmentToken, fallbackKey, runtimeWorkspaceId, state.openEpoch, store, viewId]);

  useEffect(() => {
    if (!session || session.epoch !== state.openEpoch || declarative) return undefined;
    const source = createStreamEventSource(
      cmdPaletteViewSessionStreamURL(session.viewSession, session.streamToken)
    );
    const onFrame = (event: Event) => {
      if (!(event instanceof MessageEvent)) return;
      try {
        store.trigger.frameReceived({ frame: JSON.parse(event.data) as CmdPaletteViewFrame });
      } catch (error) {
        store.trigger.crashed({ error: errorMessage(error) });
      }
    };
    source.addEventListener("cmd_palette.view.frame", onFrame);
    source.onerror = () => {
      store.trigger.crashed({ error: "The extension process stopped responding." });
    };
    return () => {
      source.removeEventListener("cmd_palette.view.frame", onFrame);
      source.close();
    };
  }, [declarative, session, state.openEpoch, store]);

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
      const attachmentToken = client?.attachmentToken?.trim() ?? "";
      if (
        !session ||
        attachmentToken === "" ||
        !current.frame ||
        !programHandlerIsLive(current, handler)
      ) {
        return;
      }
      const seq = current.nextSeq + 1;
      const eventCount = controlled ? current.eventCount + 1 : current.eventCount;
      const resolvedArgs = controlled ? [...args, eventCount] : [...args];
      const effectResult = effectResultsRef.current[0];
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
      })
        .then(() => {
          if (effectResult) effectResultsRef.current.shift();
        })
        .catch(error => {
          if (
            error instanceof CmdPaletteApiError &&
            (error.status === 403 || error.status === 410)
          ) {
            store.trigger.crashed({ error: error.message });
            return;
          }
          console.warn("Command palette view event was rejected", error);
        });
    }
  );

  useEffect(() => {
    const handler = state.payload?.chrome?.on_search;
    if (!handler || queryRef.current === query) return;
    queryRef.current = query;
    sendEvent(handler, [query], true);
  }, [query, state.payload?.chrome?.on_search]);

  const executeEffect = useEffectEvent(async (effect: CmdPaletteViewEffect): Promise<void> => {
    if (effect.toast) {
      notifyUser({ message: effect.toast.message, tone: feedbackTone(effect.toast.tone) });
    } else if (effect.copy) {
      await copyEffect(effect.copy.content);
    } else if (effect.open_url) {
      await runSerializedAction(dispatch, viewId, {
        title: "Open link",
        action: { kind: "url", url: effect.open_url.url },
      });
    } else if (effect.open_app) {
      await runSerializedAction(dispatch, viewId, {
        title: "Open app",
        action: { kind: "navigate", app: effect.open_app.app },
      });
    } else if (effect.pick_files) {
      effectResultsRef.current.push({
        effect_id: effect.id,
        payload: await pickFiles(effect.pick_files.directories ?? false),
      });
    }
  });

  useEffect(() => {
    const pending = (state.frame?.effects ?? []).filter(
      effect => !executedEffectsRef.current.has(effect.id)
    );
    if (pending.length === 0) return;
    for (const effect of pending) executedEffectsRef.current.add(effect.id);
    void Promise.allSettled(pending.map(executeEffect)).then(() => {
      store.trigger.effectsAcknowledged({ ids: pending.map(effect => effect.id) });
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
    ? programEnvelope(viewId, title, state.frame!, state.payload)
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
  const runAction = async (
    action: CmdPaletteViewAction | undefined,
    values: Readonly<Record<string, unknown>> = {}
  ) => {
    if (!action) return;
    if (action.handler) {
      sendEvent(action.handler, Object.keys(values).length === 0 ? [] : [values], false);
      return;
    }
    await runSerializedAction(dispatch, viewId, action, values, onDismiss);
  };
  const setChip = (id: string) => {
    setActiveChip(id);
    const handler = state.payload?.chrome?.on_chip;
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
        filterLocally: envelope.payload.chrome?.complete === true,
      })
    : emptyFrame("Opening view…");
  return {
    declarative,
    definition: viewDefinition(viewId, envelope ?? undefined),
    content: contentForPhase(
      baseContent,
      state.phase,
      state.reloaded,
      title,
      viewId,
      state.error,
      retry
    ),
    error: state.phase === "unavailable" ? state.error : null,
    loading: state.phase === "opening",
    timedOut: state.phase === "degraded" || state.phase === "circuit-open",
    retry,
  };
}

function contentForPhase(
  content: PaletteViewContent,
  phase: CmdPaletteViewProgramPhase,
  reloaded: boolean,
  title: string,
  viewId: string,
  error: string | null,
  retry: () => void
): PaletteViewContent {
  if (phase === "busy" || phase === "degraded") {
    return {
      ...content,
      header: (
        <>
          {content.header}
          <OsPaletteProgramBand phase={phase} onRetry={retry} />
        </>
      ),
    };
  }
  if (phase === "circuit-open" || phase === "unavailable") {
    return {
      ...emptyFrame(phase),
      empty: (
        <OsPaletteProgramFailure
          error={error}
          phase={phase}
          source={`${title} (${extensionSource(viewId)})`}
        />
      ),
    };
  }
  if (reloaded) {
    return {
      ...content,
      header: (
        <>
          {content.header}
          <OsPaletteProgramReloaded />
        </>
      ),
    };
  }
  return content;
}

function programEnvelope(
  viewId: string,
  title: string,
  frame: CmdPaletteViewFrame,
  payload: CmdPaletteViewEnvelope["payload"]
): CmdPaletteViewEnvelope {
  const kind = payload.form
    ? "form"
    : payload.grid
      ? "grid"
      : payload.detail && !payload.sections?.length
        ? "detail"
        : "list";
  return {
    view_id: viewId,
    title,
    kind,
    revision: frame.revision,
    stream_epoch: `session:${frame.view_session}`,
    payload,
  };
}

async function runSerializedAction(
  dispatch: CmdPaletteDispatch,
  viewId: string,
  action: CmdPaletteViewAction,
  values: Readonly<Record<string, unknown>> = {},
  onDismiss?: () => void
): Promise<void> {
  if (!action.action) return;
  const outcome = await dispatch.run(commandForViewAction(viewId, action), { args: values });
  if (outcome.status === "refused") throw new Error(outcome.reason);
  if (onDismiss && (outcome.status === "ran" || outcome.status === "invoked")) onDismiss();
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

function extensionSource(viewId: string): string {
  const extension = extensionName(viewId);
  return extension ? `ext.${extension}` : viewId;
}

function feedbackTone(tone: string): UserFeedbackTone {
  return tone === "success" || tone === "warning" || tone === "error" ? tone : "info";
}

async function copyEffect(content: string): Promise<void> {
  if (!navigator.clipboard) throw new Error("Clipboard access is unavailable.");
  await navigator.clipboard.writeText(content);
}

async function pickFiles(directories: boolean): Promise<unknown> {
  const method = Reflect.get(
    globalThis,
    directories ? "showDirectoryPicker" : "showOpenFilePicker"
  );
  if (typeof method !== "function") return { unavailable: true };
  try {
    const result = await Reflect.apply(method, globalThis, directories ? [] : [{ multiple: true }]);
    const handles = Array.isArray(result) ? result : [result];
    return { names: handles.map(handle => String(Reflect.get(handle, "name") ?? "")) };
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") return { canceled: true };
    throw error;
  }
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "The view is unavailable.";
}
