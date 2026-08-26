import { notifyUser, type UserFeedbackTone } from "@/lib/user-feedback";

import type { CmdPaletteDispatch } from "./cmd-palette-dispatch";
import { commandForViewAction } from "./cmd-palette-view-action-command";
import type {
  CmdPaletteViewAction,
  CmdPaletteViewEffect,
  CmdPaletteViewEnvelope,
  CmdPaletteViewFrame,
  CmdPaletteViewSessionEvent,
} from "./cmd-palette-types";
import type { PaletteRegistry } from "./cmd-palette-types";
import { finalizeCmdPaletteViewEffects } from "./cmd-palette-view-effects";

export type PendingEffectResult = NonNullable<CmdPaletteViewSessionEvent["effect_result"]>;

export function programViewEnvelope(
  viewId: string,
  title: string,
  frame: CmdPaletteViewFrame,
  payload: CmdPaletteViewEnvelope["payload"],
  /** The lens the daemon opened this view session under — never re-derived here. */
  profileLens: CmdPaletteViewEnvelope["profile_lens"]
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
    profile_lens: profileLens,
  };
}

export async function runSerializedViewAction(
  dispatch: CmdPaletteDispatch,
  viewId: string,
  action: CmdPaletteViewAction,
  values: Readonly<Record<string, unknown>> = {},
  onDismiss?: () => void,
  registry?: PaletteRegistry,
  confirmed = false
): Promise<void> {
  if (!action.action) return;
  const outcome = await dispatch.run(commandForViewAction(viewId, action, registry), {
    args: values,
    confirmed,
  });
  if (outcome.status === "refused") throw new Error(outcome.reason);
  if (
    onDismiss &&
    (outcome.status === "ran" || outcome.status === "invoked") &&
    action.action.kind !== "view"
  ) {
    onDismiss();
  }
}

export async function executeCmdPaletteViewEffect(
  effect: CmdPaletteViewEffect,
  dispatch: CmdPaletteDispatch,
  viewId: string,
  queue: PendingEffectResult[]
): Promise<void> {
  if (effect.toast) {
    notifyUser({ message: effect.toast.message, tone: feedbackTone(effect.toast.tone) });
    return;
  }
  if (effect.copy) {
    await runSerializedViewAction(dispatch, viewId, {
      title: "Copy",
      action: { kind: "copy", args: { content: effect.copy.content } },
    });
    return;
  }
  if (effect.open_url) {
    await runSerializedViewAction(dispatch, viewId, {
      title: "Open link",
      action: { kind: "url", url: effect.open_url.url },
    });
    return;
  }
  if (effect.open_app) {
    await runSerializedViewAction(dispatch, viewId, {
      title: "Open app",
      action: { kind: "navigate", app: effect.open_app.app },
    });
    return;
  }
  if (effect.pick_files) {
    queue.push({
      effect_id: effect.id,
      payload: await pickFiles(effect.pick_files.directories ?? false),
    });
  }
}

export function acknowledgeViewEffects(
  pending: readonly CmdPaletteViewEffect[],
  results: readonly PromiseSettledResult<void>[]
): readonly string[] {
  return finalizeCmdPaletteViewEffects(pending, results);
}

export function viewErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "The view is unavailable.";
}

function feedbackTone(tone: string): UserFeedbackTone {
  return tone === "success" || tone === "warning" || tone === "error" ? tone : "info";
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
