/** Pause, resume and remove as edits over the given places, so the fixture stays the source. */
import type { PauseReason } from "../stays-on-agents-shared/fixtures";
import type { ProfilePlace } from "./profile-fixtures";

export type PlaceEdit = "paused" | "resumed" | "removed";
export type PlaceEdits = Readonly<Record<string, PlaceEdit>>;

export function canResume(reason: PauseReason | undefined) {
  return reason !== "definition_missing" && reason !== "definition_not_stays_on";
}

function edited(place: ProfilePlace, edit: PlaceEdit | undefined): ProfilePlace {
  if (edit === "paused") {
    return { ...place, state: "paused", pauseReason: "manual", now: undefined, next: undefined };
  }
  if (edit === "resumed") {
    return { ...place, state: "idle", pauseReason: undefined, next: "in 30 min" };
  }
  return place;
}

export function applyPlaceEdits(places: readonly ProfilePlace[], edits: PlaceEdits) {
  return places.flatMap(place => {
    const edit = edits[place.scope];
    return edit === "removed" ? [] : [edited(place, edit)];
  });
}

export function resumeAllEdits(places: readonly ProfilePlace[], edits: PlaceEdits): PlaceEdits {
  const resumable = places.filter(
    place => place.state === "paused" && canResume(place.pauseReason)
  );
  return {
    ...edits,
    ...Object.fromEntries(resumable.map(place => [place.scope, "resumed" as const])),
  };
}
