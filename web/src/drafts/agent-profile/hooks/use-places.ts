import { useState } from "react";

import { applyPlaceEdits, resumeAllEdits, type PlaceEdit, type PlaceEdits } from "../place-edits";
import type { ProfilePlace } from "../profile-fixtures";

export interface PlaceActions {
  onToggle: (scope: string) => void;
  onArchive: (scope: string) => void;
}

/** The places as the person left them, derived from the given ones on every render. */
export function usePlaces(given: readonly ProfilePlace[]) {
  const [edits, setEdits] = useState<PlaceEdits>({});
  const places = applyPlaceEdits(given, edits);
  const edit = (scope: string, next: PlaceEdit) =>
    setEdits(current => ({ ...current, [scope]: next }));
  const actions: PlaceActions = {
    onToggle: scope => {
      const paused = places.find(place => place.scope === scope)?.state === "paused";
      edit(scope, paused ? "resumed" : "paused");
    },
    onArchive: scope => edit(scope, "removed"),
  };
  return { places, actions, resumeAll: () => setEdits(resumeAllEdits(places, edits)) };
}
