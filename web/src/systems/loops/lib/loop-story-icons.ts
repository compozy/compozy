import type { LucideIcon } from "lucide-react";
import {
  Ban,
  Bell,
  Check,
  CircleAlert,
  CircleCheck,
  CircleSlash,
  CopyX,
  Eye,
  GitBranch,
  GitFork,
  Hourglass,
  Minus,
  Pause,
  Pencil,
  Play,
  Radio,
  Redo2,
  RotateCcw,
  Send,
  ShieldAlert,
  TriangleAlert,
  VolumeOff,
  X,
  Zap,
  ZapOff,
} from "lucide-react";

/**
 * The story's glyph vocabulary, one fixed icon per concept across the domain
 * (DESIGN-LESSONS L7). The union lives beside the map it keys so a new beat
 * cannot name a glyph that has no icon behind it.
 */
export type LoopStoryIcon =
  | "round"
  | "check-pass"
  | "check-warn"
  | "node-done"
  | "node-failed"
  | "approval"
  | "watching"
  | "paused"
  | "resumed"
  | "started"
  | "done"
  | "circle-slash"
  | "retry"
  | "canceled"
  | "killed"
  | "quarantined"
  | "requeued"
  | "waiting"
  | "attention"
  | "attention-silence"
  | "attention-cleared"
  | "effect"
  | "suppressed"
  | "breaker-open"
  | "breaker-closed"
  | "request-opened"
  | "request-answered"
  | "request-expired"
  | "route-taken"
  | "route-default"
  | "pruned"
  | "amended"
  | "forked";

export const LOOP_STORY_ICONS: Record<LoopStoryIcon, LucideIcon> = {
  round: Play,
  "check-pass": Check,
  "check-warn": TriangleAlert,
  "node-done": Check,
  "node-failed": X,
  approval: Bell,
  watching: Eye,
  paused: Pause,
  resumed: Play,
  started: Play,
  done: Check,
  "circle-slash": CircleSlash,
  retry: RotateCcw,
  canceled: Ban,
  killed: Zap,
  quarantined: ShieldAlert,
  requeued: Redo2,
  waiting: Hourglass,
  attention: TriangleAlert,
  "attention-silence": VolumeOff,
  "attention-cleared": CircleCheck,
  effect: Send,
  suppressed: CopyX,
  "breaker-open": ZapOff,
  "breaker-closed": Radio,

  "request-opened": TriangleAlert,
  "request-answered": Check,
  "request-expired": CircleAlert,
  "route-taken": GitBranch,
  "route-default": GitBranch,
  pruned: Minus,
  amended: Pencil,
  forked: GitFork,
};
