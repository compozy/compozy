import {
  Ban,
  Check,
  Circle,
  CircleOff,
  CircleSlash,
  Clock,
  FileX,
  Hourglass,
  Moon,
  TimerOff,
  X,
  type LucideIcon,
} from "lucide-react";

import type { CallDelivery, CallState, ChildState } from "../types";

export const CALL_STATE_GLYPH = {
  queued: Clock,
  running: Circle,
  completed: Check,
  "invalid-result": FileX,
  "completed-without-result": CircleSlash,
  failed: X,
  canceled: Ban,
  timeout: TimerOff,
  expired: Hourglass,
} as const satisfies Record<CallState, LucideIcon>;

export const CALL_DELIVERY_GLYPH = {
  "delivered-into-turn": Check,
  woke: Check,
  queued: Clock,
  failed: X,
} as const satisfies Record<CallDelivery, LucideIcon>;

export const CHILD_STATE_GLYPH = {
  running: Circle,
  parked: Moon,
  gone: CircleOff,
} as const satisfies Record<ChildState, LucideIcon>;
