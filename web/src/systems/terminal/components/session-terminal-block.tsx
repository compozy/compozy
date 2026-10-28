"use client";

import { TerminalView, type TerminalEngineLoader } from "@compozy/ui";
import { useState } from "react";

import { TerminalStoreProvider } from "../contexts/terminal-store-context";
import {
  useTerminalAttachment,
  type TerminalAttachmentSocketFactory,
} from "../hooks/use-terminal-attachment";
import { useTerminalReplay } from "../hooks/use-terminal-replay";
import { useSessionTerminalSizing } from "../hooks/use-session-terminal-sizing";
import { terminalReplayFailedCopy } from "../lib/terminal-copy";
import { SESSION_TERMINAL_MAX_ROWS } from "../lib/session-terminal-rows";
import { sessionPreviewInstanceKey } from "../lib/terminal-scope-key";
import type { TerminalExit } from "../types";

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
  /** Names the screen for assistive technology. */
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
   * the tool result recorded.
   */
  scope?: { workspaceId: string; profile: string };
  exit?: TerminalExit | null;
  /** Test seam; the browser socket is the default. */
  socketFactory?: TerminalAttachmentSocketFactory;
  /** Replaces the emulator. Tests and playback harnesses only. */
  engineLoader?: TerminalEngineLoader;
}

/**
 * The screen of a terminal, inside an opened transcript row.
 *
 * The row that hosts it states the title, outcome and the jump to the
 * terminal; this is only the screen — sized to its content while it is a
 * record, and a fixed window while it still streams.
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
  exit,
  socketFactory,
  engineLoader,
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
  // A screen that streamed here keeps what it streamed once the run ends: the
  // recorded preview is older than those bytes, and replaying it would wipe
  // the final screen the reader just watched.
  const [streamed, setStreamed] = useState(false);

  const replay = useTerminalReplay(instanceId, preview, !live && !streamed);
  useTerminalAttachment({
    workspaceId: scope?.workspaceId ?? "",
    terminalId,
    scope: { profile: scope?.profile ?? "" },
    // Presentation only. Nothing in a transcript may write to the terminal.
    mode: "read",
    handleRef: replay.handleRef,
    socketFactory,
    enabled: live,
    displayRows: SESSION_TERMINAL_MAX_ROWS,
    onOutputPainted: () => setStreamed(true),
  });

  const sizing = useSessionTerminalSizing({
    instanceId,
    handleRef: replay.handleRef,
    fullWindow: live || streamed,
    preview,
  });
  const onAttached = () => {
    sizing.onAttached();
    replay.onAttached();
  };

  return (
    <div
      className="max-w-160 overflow-hidden rounded-lg border border-line-soft bg-terminal-bg"
      data-live={live ? "true" : "false"}
      data-testid={`session-terminal-block-${terminalId}`}
    >
      <TerminalView
        aria-label={live ? `${title} — watching` : `${title} — final screen`}
        className="px-2.5 py-2 font-mono text-transcript-meta tracking-mono"
        {...(engineLoader ? { engineLoader } : {})}
        handleRef={replay.handleRef}
        // A distinct identity from the app's pane: the preview is a second
        // view of the same terminal, never a second owner of its buffer.
        instanceId={instanceId}
        onAttached={onAttached}
        onProposeDimensions={sizing.onProposeDimensions}
        readOnly
        screenReaderMode
      />
      {replay.writeError ? (
        <p
          className="border-line-soft border-t px-2.5 py-1 font-mono text-eyebrow text-subtle"
          role="status"
        >
          {terminalReplayFailedCopy()}
        </p>
      ) : null}
    </div>
  );
}
