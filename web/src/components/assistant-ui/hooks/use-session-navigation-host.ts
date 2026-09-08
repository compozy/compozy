import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import {
  type Dispatch,
  type RefObject,
  type SetStateAction,
  useEffect,
  useRef,
  useState,
} from "react";

import {
  type SessionFindModel,
  useSessionFind,
} from "@/systems/session/hooks/use-session-navigation";
import { sessionKeys } from "@/systems/session/lib/query-keys";
import { sessionTranscriptOptions } from "@/systems/session/lib/query-options";
import {
  findShortcutSeed,
  isFindShortcut,
  isSequenceLoaded,
  navigationRefreshKey,
  type SessionSequenceRange,
} from "@/systems/session/lib/session-navigation";
import type { SessionFindJumpHandlers } from "@/systems/session/lib/session-navigation-find-store";
import {
  findMatchesInsideFolds,
  findMatchSource,
  indexTranscriptSequences,
  landingMessageId,
  type SessionSequenceIndex,
  visibleSequenceWindow,
} from "@/systems/session/lib/session-navigation-transcript";
import {
  flattenTranscriptEntries,
  nextTranscriptPageParam,
  type SessionTranscriptData,
} from "@/systems/session/lib/session-transcript-query";
import type { SessionTranscriptSearchMatch } from "@/systems/session/types";
import { locateFindTarget } from "../session-find-ranges";
import type {
  SessionNavigationReveal,
  SessionNavigationTarget,
} from "./session-navigation-target-context";

/** A landed row rests this far under the viewport's top edge (S6/S8 artboards). */
export const NAVIGATION_LANDING_OFFSET_PX = 16;
/** Matched content deep inside a message lands lower, with a line or two of its row above it. */
export const NAVIGATION_CONTENT_OFFSET_PX = 48;
/** While a turn streams, the navigation reads refetch on this cadence — never per token. */
export const NAVIGATION_STREAM_REFRESH_MS = 5_000;
/** The trail rail needs room beside the transcript column (find-trail artboard). */
export const TRAIL_MIN_PANE_WIDTH_PX = 864;
/** Bound on pages one jump may pull before giving up on an unreachable cursor. */
const NAVIGATION_LOAD_OLDER_MAX_PAGES = 500;
const OS_WINDOW_FRAME_SELECTOR = '[data-slot="os-window-frame"]';
const COMPOSER_INPUT_SELECTOR = '[data-testid="composer-input"]';
const COMPOSER_EDITABLE_SELECTOR = `${COMPOSER_INPUT_SELECTOR} [contenteditable="true"]`;
const THREAD_ROOT_SELECTOR = "[data-thread-root]";
const EMPTY_COUNTS: ReadonlyMap<string, number> = new Map();
const EMPTY_RELEASED: ReadonlySet<string> = new Set();

interface VisibleRows {
  first: string | null;
  last: string | null;
}

export interface SessionNavigationHostOptions {
  workspaceId: string;
  sessionId: string;
  viewportRef: RefObject<HTMLDivElement | null>;
  messageCount: number;
  isSessionRunning: boolean;
  readVisibleMessageIds: () => VisibleRows;
  scrollToMessage: (
    messageId: string,
    offsetTop: number,
    locate?: (row: HTMLElement) => Range | null
  ) => Promise<boolean>;
}

export interface SessionNavigationHost {
  find: SessionFindModel;
  findOpen: boolean;
  findFocusKey: number;
  closeFind: () => void;
  isSequenceFolded: (sequence: number) => boolean;
  resolveMatchTime: (match: SessionTranscriptSearchMatch) => number | null;
  /** A jump is loading or landing: far pages must not be released under it. */
  jumpActive: boolean;
  refreshKey: string;
  target: SessionNavigationTarget;
  trail: {
    enabled: boolean;
    paneHeightPx: number;
    viewportTopSequence: number | null;
    visibleRange: SessionSequenceRange | null;
    onJumpToSequence: (sequence: number) => void;
  };
}

function nextFrame(): Promise<void> {
  return new Promise(resolve => requestAnimationFrame(() => resolve()));
}

/** ⌘F belongs to the focused session window; outside the OS shell the viewport is the only one. */
function findShortcutScoped(viewport: HTMLElement): boolean {
  const frame = viewport.closest(OS_WINDOW_FRAME_SELECTOR);
  return frame === null || frame.hasAttribute("data-focused");
}

function seedFromSelection(viewport: HTMLElement): string {
  const selection = window.getSelection();
  if (!selection || selection.isCollapsed) return "";
  const root = viewport.closest(THREAD_ROOT_SELECTOR) ?? viewport;
  const anchor = selection.anchorNode;
  if (!anchor || !root.contains(anchor)) return "";
  return findShortcutSeed(selection.toString());
}

function useStreamTick(active: boolean): number {
  const [tick, setTick] = useState(0);
  useEffect(() => {
    if (!active) return undefined;
    const timer = window.setInterval(
      () => setTick(previous => previous + 1),
      NAVIGATION_STREAM_REFRESH_MS
    );
    return () => window.clearInterval(timer);
  }, [active]);
  return tick;
}

interface NavigationLiveState {
  matches: readonly SessionTranscriptSearchMatch[];
  query: string;
  readVisibleMessageIds: SessionNavigationHostOptions["readVisibleMessageIds"];
  scrollToMessage: SessionNavigationHostOptions["scrollToMessage"];
}

interface NavigationHistoryLoader {
  readIndex: () => SessionSequenceIndex;
  readData: () => SessionTranscriptData | undefined;
  fetchNextPage: () => Promise<{ isError: boolean; error: unknown }>;
  setJumpActive: Dispatch<SetStateAction<boolean>>;
}

interface NavigationLanding {
  readIndex: () => SessionSequenceIndex;
  latest: RefObject<NavigationLiveState>;
  setJumpActive: Dispatch<SetStateAction<boolean>>;
  setReveal: Dispatch<SetStateAction<SessionNavigationReveal | null>>;
  setReleased: Dispatch<SetStateAction<ReadonlySet<string>>>;
}

async function loadNavigationHistory(
  sequence: number,
  { readIndex, readData, fetchNextPage, setJumpActive }: NavigationHistoryLoader
): Promise<boolean> {
  setJumpActive(true);
  let reached = false;
  try {
    for (let pages = 0; pages < NAVIGATION_LOAD_OLDER_MAX_PAGES; pages += 1) {
      const current = readIndex();
      if (isSequenceLoaded(current.range, sequence)) {
        reached = true;
        return true;
      }
      const data = readData();
      const lastPage = data?.pages[data.pages.length - 1];
      if (!lastPage || nextTranscriptPageParam(lastPage) === undefined) return false;
      const result = await fetchNextPage();
      if (result.isError) throw result.error;
      const after = readIndex();
      const progressed =
        after.range !== null && (current.range === null || after.range.from < current.range.from);
      if (!progressed) return false;
    }
    return false;
  } finally {
    // The landing keeps the pin; a failed load releases it here.
    if (!reached) setJumpActive(false);
  }
}

// The reveal names the exact part the daemon matched (`part_index`/`field`)
// when the match carries one; a match without a source (older daemon) names
// the turn only, which opens its fold and claims nothing about a part.
async function landNavigationSequence(
  sequence: number,
  { readIndex, latest, setJumpActive, setReveal, setReleased }: NavigationLanding
): Promise<void> {
  setJumpActive(true);
  try {
    const current = readIndex();
    const entry = current.bySequence.get(sequence);
    const match = latest.current.matches.find(candidate => candidate.sequence === sequence);
    const source = match ? findMatchSource(match, entry) : null;
    let partIndex: number | null = null;
    if (entry) {
      partIndex = source?.partIndex ?? null;
      setReveal(previous => ({
        field: source?.field ?? null,
        key: (previous?.key ?? 0) + 1,
        messageId: entry.messageId,
        opensBody: source?.opensBody ?? false,
        partIndex,
        toolCallId: source?.toolCallId ?? null,
        turnId: match?.turn_id ?? entry.turnId,
      }));
      setReleased(EMPTY_RELEASED);
    }
    const landing = landingMessageId(current, sequence);
    if (landing === null) throw new Error("That part of the history is not loaded.");
    // Rows for a page that just landed, and the disclosures the reveal opens,
    // commit on the next frames; the landing loop keeps waiting for them too.
    await nextFrame();
    await nextFrame();
    const needle = latest.current.query;
    const landed = await latest.current.scrollToMessage(
      landing,
      partIndex === null ? NAVIGATION_LANDING_OFFSET_PX : NAVIGATION_CONTENT_OFFSET_PX,
      row => locateFindTarget(row, partIndex, needle)
    );
    if (!landed) throw new Error("That message did not come into view.");
  } finally {
    setJumpActive(false);
  }
}

/**
 * The viewport's navigation host (task_08): find and the trail over the
 * daemon's full-history reads, landed through the one scroll owner. Jumps to
 * unloaded history pull older pages through the transcript query's own
 * continuation until the cursor is loaded (page release pauses meanwhile), open
 * the settled fold a hit sits behind, and land the row in free mode. Refreshes
 * are keyed on the durable fences and the streaming edge settling, with a slow
 * tick while a turn streams, so token deltas never launch a search.
 */
export function useSessionNavigationHost({
  workspaceId,
  sessionId,
  viewportRef,
  messageCount,
  isSessionRunning,
  readVisibleMessageIds,
  scrollToMessage,
}: SessionNavigationHostOptions): SessionNavigationHost {
  const queryClient = useQueryClient();
  const transcriptKey = sessionKeys.transcript(workspaceId, sessionId);
  // A second observer on the canonical transcript query: same cache, no reread
  // of its own — only the continuation (`fetchNextPage`) for jump loading.
  const transcript = useInfiniteQuery({
    ...sessionTranscriptOptions(workspaceId, sessionId),
    notifyOnChangeProps: ["data"],
    refetchOnMount: false,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
  });
  const index = indexTranscriptSequences(flattenTranscriptEntries(transcript.data));
  const head = transcript.data?.pages[0];
  const streamTick = useStreamTick(isSessionRunning);
  const refreshKey = navigationRefreshKey({
    entryCount: index.entries.length,
    epoch: head?.epoch ?? null,
    generation: head?.generation ?? null,
    streamTick,
    streaming: isSessionRunning,
  });

  const [findState, setFindState] = useState<{ seed: string; focusKey: number } | null>(null);
  const [jumpActive, setJumpActive] = useState(false);
  const [reveal, setReveal] = useState<SessionNavigationReveal | null>(null);
  const [released, setReleased] = useState<ReadonlySet<string>>(EMPTY_RELEASED);
  const [visible, setVisible] = useState<VisibleRows>({ first: null, last: null });
  const [pane, setPane] = useState({ height: 0, width: 0 });

  // Jumps outlive the render that started them (pages land in between), so the
  // async handlers read the current scroll owner and matches through one ref.
  const latest = useRef({
    matches: [] as readonly SessionTranscriptSearchMatch[],
    query: "",
    readVisibleMessageIds,
    scrollToMessage,
  });
  const readIndex = (): SessionSequenceIndex =>
    indexTranscriptSequences(
      flattenTranscriptEntries(queryClient.getQueryData<SessionTranscriptData>(transcriptKey))
    );

  const loadOlderUntil = (sequence: number) =>
    loadNavigationHistory(sequence, {
      readIndex,
      readData: () => queryClient.getQueryData<SessionTranscriptData>(transcriptKey),
      fetchNextPage: () => transcript.fetchNextPage(),
      setJumpActive,
    });
  const jumpToSequence = (sequence: number) =>
    landNavigationSequence(sequence, { readIndex, latest, setJumpActive, setReveal, setReleased });

  const handlers: SessionFindJumpHandlers = {
    isSequenceLoaded: sequence => isSequenceLoaded(index.range, sequence),
    jumpToSequence,
    loadOlderUntil,
  };

  const find = useSessionFind({
    handlers,
    initialQuery: findState?.seed ?? "",
    open: findState !== null,
    refreshKey,
    sessionId,
    workspaceId,
  });

  useEffect(() => {
    latest.current = {
      matches: find.matches,
      query: find.committedQuery,
      readVisibleMessageIds,
      scrollToMessage,
    };
  });

  // ⌘F / Ctrl+F for the focused session window: opens (or refocuses) the bar,
  // seeded from a selection inside the thread. The browser's own find never runs.
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!isFindShortcut(event) || event.defaultPrevented) return;
      const viewport = viewportRef.current;
      if (!viewport || !findShortcutScoped(viewport)) return;
      event.preventDefault();
      const seed = seedFromSelection(viewport);
      setFindState(previous => ({ focusKey: (previous?.focusKey ?? 0) + 1, seed }));
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [viewportRef]);

  // The rows in view (trail anchor/window) and the pane size, sampled once per
  // frame on scroll/resize and whenever the row list changes.
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;
    let frame = 0;
    const sample = () => {
      frame = 0;
      const next = latest.current.readVisibleMessageIds();
      setVisible(previous =>
        previous.first === next.first && previous.last === next.last ? previous : next
      );
    };
    const schedule = () => {
      if (frame === 0) frame = requestAnimationFrame(sample);
    };
    const observer = new ResizeObserver(() => {
      setPane(previous =>
        previous.height === viewport.clientHeight && previous.width === viewport.clientWidth
          ? previous
          : { height: viewport.clientHeight, width: viewport.clientWidth }
      );
      schedule();
    });
    observer.observe(viewport);
    viewport.addEventListener("scroll", schedule, { passive: true });
    schedule();
    return () => {
      observer.disconnect();
      viewport.removeEventListener("scroll", schedule);
      if (frame !== 0) cancelAnimationFrame(frame);
    };
  }, [messageCount, viewportRef]);

  const closeFind = () => {
    setFindState(null);
    const root = viewportRef.current?.closest(THREAD_ROOT_SELECTOR);
    const editable =
      root?.querySelector<HTMLElement>(COMPOSER_EDITABLE_SELECTOR) ??
      root?.querySelector<HTMLElement>(COMPOSER_INPUT_SELECTOR);
    editable?.focus();
  };

  // "Behind a fold" as the daemon located it: reasoning or transient tool work
  // of a settled agent entry. The live turn never folds, and a fold a jump is
  // holding open is not behind anything right now.
  const liveEntryId = isSessionRunning ? (index.entries.at(-1)?.messageId ?? null) : null;
  const matchBehindFold = (sequence: number): boolean => {
    const entry = index.bySequence.get(sequence);
    const match = find.matches.find(candidate => candidate.sequence === sequence);
    const source = match ? findMatchSource(match, entry) : null;
    if (!entry || !match || !source?.insideFold || entry.messageId === liveEntryId) return false;
    const heldOpen =
      reveal !== null &&
      reveal.turnId === match.turn_id &&
      !released.has(`turn-fold:${match.turn_id}`);
    return !heldOpen;
  };

  const jumpFromTrail = async (sequence: number) => {
    try {
      const loaded =
        isSequenceLoaded(readIndex().range, sequence) || (await loadOlderUntil(sequence));
      if (loaded) await jumpToSequence(sequence);
    } catch (error) {
      console.error("Failed to jump to the message", error);
    }
  };

  const activeEntry =
    find.activeSequence === null ? undefined : index.bySequence.get(find.activeSequence);
  const window_ = visibleSequenceWindow(index, visible.first, visible.last);

  return {
    closeFind,
    find,
    findFocusKey: findState?.focusKey ?? 0,
    findOpen: findState !== null,
    isSequenceFolded: matchBehindFold,
    jumpActive,
    refreshKey,
    resolveMatchTime: match => index.bySequence.get(match.sequence)?.at ?? null,
    target: {
      activeMessageId: activeEntry?.messageId ?? null,
      foldMatchCounts: findState ? findMatchesInsideFolds(find.matches, index) : EMPTY_COUNTS,
      query: findState ? find.committedQuery : "",
      releaseDisclosure: id =>
        setReleased(previous => (previous.has(id) ? previous : new Set([...previous, id]))),
      released,
      reveal,
    },
    trail: {
      enabled: pane.width >= TRAIL_MIN_PANE_WIDTH_PX,
      onJumpToSequence: sequence => void jumpFromTrail(sequence),
      paneHeightPx: pane.height,
      viewportTopSequence: window_.top,
      visibleRange: window_.range,
    },
  };
}
