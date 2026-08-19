import {
  Ban,
  Pause,
  Pencil,
  Play,
  Redo2,
  RotateCcw,
  ShieldAlert,
  SkipForward,
  TimerReset,
  Zap,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import type { ConfirmDialogIconTone } from "@compozy/ui";

import type { LoopNodeVerb, LoopRunVerb } from "./loop-node-controls";

/** One glyph per node verb — shared by the menu and every confirm head well. */
export const LOOP_NODE_VERB_ICONS: Record<LoopNodeVerb, LucideIcon> = {
  pause: Pause,
  resume: Play,
  "resume-reset-attempts": TimerReset,
  "resume-immediate": SkipForward,
  "resume-wait": Play,
  cancel: Ban,
  kill: Zap,
  requeue: Redo2,
  "open-quarantine": ShieldAlert,
  amend: Pencil,
  rerun: RotateCcw,
};

export const LOOP_RUN_VERB_ICONS: Record<Extract<LoopRunVerb, "cancel" | "kill">, LucideIcon> = {
  cancel: Ban,
  kill: Zap,
};

/** Head-well tone: escalation lives here, not on body copy. */
export const LOOP_NODE_VERB_ICON_TONE: Record<LoopNodeVerb, ConfirmDialogIconTone> = {
  pause: "neutral",
  resume: "accent",
  "resume-reset-attempts": "accent",
  "resume-immediate": "accent",
  "resume-wait": "accent",
  cancel: "neutral",
  kill: "danger",
  requeue: "accent",
  "open-quarantine": "danger",
  amend: "accent",
  rerun: "accent",
};

export const LOOP_RUN_VERB_ICON_TONE: Record<
  Extract<LoopRunVerb, "cancel" | "kill">,
  ConfirmDialogIconTone
> = {
  cancel: "neutral",
  kill: "danger",
};
