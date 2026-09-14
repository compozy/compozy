export type SessionTimelinePart =
  | SessionTimelineTextPart
  | SessionTimelineReasoningPart
  | SessionTimelineToolPart
  | SessionTimelineDataPart
  | SessionTimelineWorkingPart;

interface SessionTimelineBasePart {
  id: string;
  turnId?: string;
  timestamp?: string;
  state?: string;
  /** Position in the daemon's projected `message.parts`; search results name it (`part_index`). */
  partIndex?: number;
}

export interface SessionTimelineTextPart extends SessionTimelineBasePart {
  kind: "text";
  text: string;
}

export interface SessionTimelineReasoningPart extends SessionTimelineBasePart {
  kind: "reasoning";
  text: string;
}

export interface SessionTimelineToolPart extends SessionTimelineBasePart {
  kind: "tool";
  toolCallId: string;
  toolName: string;
  toolTitle?: string;
  args: Record<string, unknown>;
  result?: unknown;
  isError?: boolean;
  status: "running" | "settled" | "interrupted";
}

export interface SessionTimelineDataPart extends SessionTimelineBasePart {
  kind: "data";
  name: string;
  data: unknown;
}

export interface SessionTimelineWorkingPart extends SessionTimelineBasePart {
  kind: "working";
  startedAt?: number;
}

/** Runtime cancellation spellings share one stopped presentation. */
export function isInterruptedState(state: string | undefined): boolean {
  return state === "interrupted" || state === "cancelled" || state === "canceled";
}
