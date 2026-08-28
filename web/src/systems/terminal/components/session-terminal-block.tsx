"use client";

import { Button, MonoId, Pill, TerminalView, type TerminalEngineLoader } from "@compozy/ui";

import { TerminalSquare } from "lucide-react";

import { TerminalStoreProvider } from "../contexts/terminal-store-context";
import {
  useTerminalAttachment,
  type TerminalAttachmentSocketFactory,
} from "../hooks/use-terminal-attachment";
import { useTerminalReplay } from "../hooks/use-terminal-replay";
import { terminalExitCopy, terminalReplayFailedCopy } from "../lib/terminal-copy";
import { sessionPreviewInstanceKey } from "../lib/terminal-scope-key";
import type { TerminalLeaseView } from "../lib/terminal-lease";
import type { TerminalExit } from "../types";
import { TerminalLeaseBadge } from "./terminal-lease-badge";

export interface SessionTerminalBlockProps {
  terminalId: string;
  /**
   * What makes this block itself — the tool call it renders.
   *
   * Two blocks can name the same terminal id: the same command scrolled past
   * twice, or two profiles that each opened a `dev server`. They are different
   * screens, and sharing one emulator between them would move the host node
   * from one to the other and mix their bytes.
   */
  blockId: string;
  title: string;
  /** The last lines, or the final screen once the command ends. */
  preview: string;
  /**
   * Where the terminal lives, when the caller knows.
   *
   * With it, a command that is still running is *shown* running: the block
   * opens its own read-only attachment and paints from byte zero. Without it
   * there is nothing to attach to — terminals are profile-scoped, and guessing
   * the profile reads as `terminal_not_found` — so the block paints the screen
   * the tool result recorded and says so.
   */
  scope?: { workspaceId: string; profile: string };
  /** Present only when the catalog already knows who holds the terminal. */
  lease?: TerminalLeaseView;
  exit?: TerminalExit | null;
  /** True past the agent's wait budget: the command runs on without it. */
  stillRunning?: boolean;
  durationLabel?: string;
  /** Clock from the catalog's `created_at` when that timestamp exists. */
  startedLabel?: string;
  /** Absent until the Terminal app is reachable; then it focuses the window. */
  onOpenTerminal?: () => void;
  /** Test seam; the browser socket is the default. */
  socketFactory?: TerminalAttachmentSocketFactory;
  /** Replaces the emulator. Tests and playback harnesses only. */
  engineLoader?: TerminalEngineLoader;
}

/**
 * A window into a terminal, inside the conversation.
 *
 * The terminal stays the truth; this block is a view of it. It carries the
 * who-is-in-control chip when the catalog knows it and one jump action, and
 * nothing else — a second transcript would compete with the one that already
 * exists.
 */
export function SessionTerminalBlock(props: SessionTerminalBlockProps) {
  // Its own store, not the Terminal app's. A preview in a transcript is a
  // separate view of the same terminal with its own connection state, and the
  // conversation it lives in has no terminal store to borrow.
  return (
    <TerminalStoreProvider>
      <SessionTerminalBlockBody {...props} />
    </TerminalStoreProvider>
  );
}

function SessionTerminalBlockBody({
  terminalId,
  blockId,
  title,
  preview,
  scope,
  lease,
  exit,
  stillRunning = false,
  durationLabel,
  startedLabel,
  socketFactory,
  engineLoader,
  onOpenTerminal,
}: SessionTerminalBlockProps) {
  // Scoped as fully as the caller can name it: the block, the workspace and
  // profile when known, and the terminal. Length-prefixed so no id containing a
  // separator can collide with another.
  const instanceId = sessionPreviewInstanceKey({
    blockId,
    terminalId,
    ...(scope ? { workspaceId: scope.workspaceId, profile: scope.profile } : {}),
  });
  // A run that has not ended is still producing bytes, and this is a view of
  // that terminal rather than a record of it — so while it runs, it streams.
  const live = scope !== undefined && !exit;
  const replay = useTerminalReplay(instanceId, preview, !live);
  useTerminalAttachment({
    workspaceId: scope?.workspaceId ?? "",
    terminalId,
    scope: { profile: scope?.profile ?? "" },
    // Watching only. Nothing in a transcript may claim the write lease.
    mode: "read",
    handleRef: replay.handleRef,
    socketFactory,
    enabled: live,
  });

  const outcome = exit ? terminalExitCopy(exit) : null;

  return (
    <div
      className="max-w-160 overflow-hidden rounded-md border border-line"
      data-testid={`session-terminal-block-${terminalId}`}
    >
      <div className="flex min-h-8 min-w-0 items-center gap-2 border-line border-b bg-canvas-soft px-2.5">
        <TerminalSquare aria-hidden="true" className="size-deck-glyph flex-none text-subtle" />
        <span className="truncate font-semibold text-fg text-transcript-body">{title}</span>
        <MonoId size="sm" value={terminalId} />
        <div className="ml-auto flex flex-none items-center gap-1.5">
          {lease ? <TerminalLeaseBadge view={lease} /> : null}
          {onOpenTerminal ? (
            <Button
              data-testid={`session-terminal-open-${terminalId}`}
              onClick={onOpenTerminal}
              size="xs"
              type="button"
              variant="ghost"
            >
              Open terminal
            </Button>
          ) : null}
        </div>
      </div>
      <div className="flex max-h-55 flex-col bg-terminal-bg">
        <TerminalView
          aria-label={live ? `${title} — watching` : `${title} — final screen`}
          className="px-3 py-2 font-mono text-transcript-meta tracking-mono"
          {...(engineLoader ? { engineLoader } : {})}
          handleRef={replay.handleRef}
          // A distinct identity from the app's pane: the preview is a second
          // view of the same terminal, never a second claim on its buffer.
          instanceId={instanceId}
          onAttached={replay.onAttached}
          readOnly
          screenReaderMode
        />
      </div>
      <div className="flex min-h-7 items-center gap-2 border-line border-t bg-canvas-soft px-2.5 text-micro text-subtle">
        {replay.writeError ? (
          <span role="status">{terminalReplayFailedCopy()}</span>
        ) : outcome ? (
          <>
            <Pill size="xs" tone={outcome.tone === "success" ? "success" : "neutral"}>
              {outcome.label}
            </Pill>
            <MonoId size="sm" value={outcome.code} />
            {durationLabel ? <span>{durationLabel}</span> : null}
          </>
        ) : (
          <>
            <Pill.Dot aria-hidden="true" pulse size="sm" tone="accent" />
            <span>
              {stillRunning
                ? "still running — the agent continued without waiting"
                : startedLabel
                  ? `running · started ${startedLabel}`
                  : "running"}
            </span>
          </>
        )}
      </div>
    </div>
  );
}
