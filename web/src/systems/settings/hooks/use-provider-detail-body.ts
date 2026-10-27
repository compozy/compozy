import { useDeferredValue } from "react";

import type { ProviderDraft, SettingsProviderEntry } from "../types";

export type ProviderDetailMode = "inspect" | "edit" | "create";

/** What the provider dialog's body renders from: one mode and the data that mode needs. */
export interface ProviderDetailBodyView {
  mode: ProviderDetailMode;
  provider: SettingsProviderEntry | null;
  draft: ProviderDraft | null;
}

/** A view has something to draw when its mode's data is present; a closed dialog has none. */
function hasContent(view: ProviderDetailBodyView): boolean {
  return view.mode === "inspect" ? view.provider !== null : view.draft !== null;
}

/**
 * The view the dialog body shows. The provider inspector lives in an external store, whose updates
 * always render urgently, and an urgent render never animates. So when the mode flips between the
 * overview and the form, the body keeps the previous view for that urgent render, and React then
 * re-renders it with the new one as a transition, which is the render `<ViewTransition>` animates.
 * Every other change passes straight through, so typing in the form is never deferred, and the
 * dialog opening or closing (a view with nothing to draw) swaps at once.
 */
export function useProviderDetailBody(live: ProviderDetailBodyView): ProviderDetailBodyView {
  const deferred = useDeferredValue(live);
  const flipping = deferred.mode !== live.mode && hasContent(deferred) && hasContent(live);
  return flipping ? deferred : live;
}
