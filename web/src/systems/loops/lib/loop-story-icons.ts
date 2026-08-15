import type { LucideIcon } from "lucide-react";
import {
  Ban,
  Bell,
  Check,
  CircleCheck,
  CopyX,
  Eye,
  Hourglass,
  Pause,
  Play,
  Radio,
  Redo2,
  RotateCcw,
  Send,
  ShieldAlert,
  Square,
  TriangleAlert,
  VolumeOff,
  X,
  Zap,
  ZapOff,
} from "lucide-react";

import type { LoopStoryIcon } from "./loop-run-story-types";

/**
 * One Lucide glyph per story concept (DESIGN-BACKLOG §2.1 / L7).
 * `stopped` stays until Packet G retires the deleted `stop` verb.
 */
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
  stopped: Square,
  done: Check,
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
};
