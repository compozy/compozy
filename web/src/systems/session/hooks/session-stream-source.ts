import { createStreamEventSource } from "@/lib/ticketed-event-source";

const TRANSCRIPT_SNAPSHOT_EVENT = "transcript_snapshot";
const TRANSCRIPT_DELTA_EVENT = "transcript_delta";
const GOAL_SNAPSHOT_CHANGED_EVENT = "goal_snapshot_changed";
const SESSION_COMMANDS_CHANGED_EVENT = "session_commands_changed";
const SESSION_STOPPED_EVENT = "session_stopped";
const SESSION_DONE_EVENT = "done";

export interface SessionStreamEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener?: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

export type SessionStreamEventSourceFactory = (url: string) => SessionStreamEventSource;

export interface SessionStreamListeners {
  commandsChanged: EventListener;
  delta: EventListener;
  goalSnapshot: EventListener;
  snapshot: EventListener;
  terminal: EventListener;
}

export function defaultEventSourceFactory(url: string): SessionStreamEventSource {
  return createStreamEventSource(url);
}

export function attachSessionStreamSource(
  source: SessionStreamEventSource,
  handleError: (event: Event) => void,
  listeners: SessionStreamListeners
): () => void {
  source.onmessage = null;
  source.onerror = handleError;
  source.addEventListener(TRANSCRIPT_SNAPSHOT_EVENT, listeners.snapshot);
  source.addEventListener(TRANSCRIPT_DELTA_EVENT, listeners.delta);
  source.addEventListener(GOAL_SNAPSHOT_CHANGED_EVENT, listeners.goalSnapshot);
  source.addEventListener(SESSION_COMMANDS_CHANGED_EVENT, listeners.commandsChanged);
  source.addEventListener(SESSION_STOPPED_EVENT, listeners.terminal);
  source.addEventListener(SESSION_DONE_EVENT, listeners.terminal);
  return () => {
    if (!source.removeEventListener) return;
    source.removeEventListener(TRANSCRIPT_SNAPSHOT_EVENT, listeners.snapshot);
    source.removeEventListener(TRANSCRIPT_DELTA_EVENT, listeners.delta);
    source.removeEventListener(GOAL_SNAPSHOT_CHANGED_EVENT, listeners.goalSnapshot);
    source.removeEventListener(SESSION_COMMANDS_CHANGED_EVENT, listeners.commandsChanged);
    source.removeEventListener(SESSION_STOPPED_EVENT, listeners.terminal);
    source.removeEventListener(SESSION_DONE_EVENT, listeners.terminal);
  };
}
