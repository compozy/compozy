import { useState } from "react";

import { toast } from "@compozy/ui";

import { activityFor } from "../checkins-fixtures";
import {
  FLOOR,
  countChanges,
  nextCheckIn,
  type CheckInSettings,
  type Standing,
  type TryPhase,
} from "../checkins-model";

const SLOWED_STOP = 4;

export type CheckInsVariant =
  | "filled"
  | "empty"
  | "too-often"
  | "unsaved"
  | "conflict"
  | "try-running"
  | "try-result"
  | "paused"
  | "failed"
  | "slowed"
  | "nowhere"
  | "advanced";

const SAVED: CheckInSettings = {
  on: true,
  stop: 2,
  hours: [9, 18],
  lunch: true,
  looksAt: ["task", "session_health", "sessions_idle"],
  also: "",
};

export const PLACES = ["compozy", "advos"];

function startingSettings(variant: CheckInsVariant): CheckInSettings {
  switch (variant) {
    case "empty":
    case "nowhere":
      return { ...SAVED, on: false };
    case "paused":
    case "failed":
      return SAVED;
    case "too-often":
      return { ...SAVED, stop: FLOOR };
    case "unsaved":
    case "conflict":
      return { ...SAVED, stop: 1, hours: [8, 18] };
    default:
      return SAVED;
  }
}

function standingOf(variant: CheckInsVariant): Standing {
  if (variant === "paused" || variant === "failed" || variant === "slowed") return variant;
  return variant === "nowhere" ? "nowhere" : "running";
}

export const TRY: Partial<Record<CheckInsVariant, TryPhase>> = {
  "try-running": "running",
  "try-result": "result",
};

/** Check-ins' draft against what's saved, plus the agent's standing in the chosen project. */
export function useCheckInsEditor(variant: CheckInsVariant) {
  const [saved, setSaved] = useState(
    variant === "empty" || variant === "nowhere" ? { ...SAVED, on: false } : SAVED
  );
  const [settings, setSettings] = useState(() => startingSettings(variant));
  const [raw, setRaw] = useState<string | null>(null);
  const [limit, setLimit] = useState(variant === "too-often");
  const [conflict, setConflict] = useState(variant === "conflict");
  const [standing, setStanding] = useState(() => standingOf(variant));
  const [place, setPlace] = useState("compozy");
  const [fresh, setFresh] = useState(false);

  // After a quiet streak the schedule stretches to every 2 hours until the person speaks up.
  const effective = standing === "slowed" ? { ...settings, stop: SLOWED_STOP } : settings;
  const patch = (next: Partial<CheckInSettings>) =>
    setSettings(current => ({ ...current, ...next }));

  return {
    settings,
    saved,
    raw,
    limit,
    conflict,
    standing,
    place,
    never: variant === "empty" && !settings.on && !fresh,
    nowhere: standing === "nowhere",
    paused: standing === "paused" || standing === "failed",
    effective,
    next: nextCheckIn(effective, fresh),
    changes: countChanges(settings, saved) + (raw === null ? 0 : 1),
    activity: activityFor(place, standing),
    patch,
    setRaw,
    setLimit,
    setPlace,
    turnOn: (on: boolean) => {
      patch({ on });
      if (on && !saved.on) setFresh(true);
    },
    keepMine: () => {
      setConflict(false);
      toast.message("Kept your version");
    },
    useTheirs: () => {
      const theirs = { ...SAVED, stop: 3 };
      setSaved(theirs);
      setSettings(theirs);
      setConflict(false);
    },
    resume: () => {
      setStanding("running");
      toast.message("Check-ins are back on");
    },
    discard: () => {
      setSettings(saved);
      setRaw(null);
      setLimit(false);
      setFresh(false);
    },
    save: () => {
      setSaved(settings);
      setRaw(null);
      toast.message("Saved");
    },
  };
}
