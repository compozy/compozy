import { Button, ToolCallRow, formatDuration } from "@compozy/ui";
import { ArrowUpRight } from "@compozy/ui/icons";
import { Suspense, lazy, use, useState } from "react";

import { OsShellContext, type OsShellHandle } from "@/systems/os";
import type { TerminalInfo } from "@/systems/terminal/parts";
import { useSessionTerminalCatalogEntry } from "../hooks/use-session-terminal-catalog";
import { useSessionTerminalScope } from "../hooks/use-session-terminal-scope";
import {
  readNonEmptyString,
  readSessionTerminalFacts,
  resolveSessionTerminalRun,
  sessionTerminalOutcome,
  type SessionTerminalFacts,
} from "../lib/session-terminal-tools";
import { getToolIcon, getToolLabel, resolveRegisteredToolName } from "../lib/tool-labels";
import type { UIMessage } from "../types";
import { DetailPre } from "./tool-renderers/detail-pre";

/**
 * The emulator arrives only when a terminal row is opened.
 *
 * A session that never opens a terminal row must not download the emulator, so
 * the screen sits behind a lazy boundary and the plain output stands in until
 * it lands.
 */
const SessionTerminalBlock = lazy(async () => {
  const { SessionTerminalBlock: Block } = await import("@/systems/terminal");
  return { default: Block };
});

type TerminalScope = { workspaceId: string; profile: string };

export interface SessionTerminalToolRowProps {
  message: UIMessage;
  defaultExpanded?: boolean;
  /** The projected part this row renders (`data-part-index`), so find can land on it. */
  partIndex?: number;
  /** A find jump needs this body open; layered over the reader's own toggle. */
  revealOpen?: boolean;
  /** The reader closed a body a jump held open: the hold is theirs to drop. */
  onRevealRelease?: () => void;
  /** The row sits on a work group's rail: the rail's branch is its bullet, so no kind glyph. */
  onRail?: boolean;
}

/**
 * A supervised terminal tool call, in the transcript's tool-row grammar.
 *
 * One calm line — verb, the terminal's name, one outcome, its duration — with
 * the jump to the Terminal window beside it. The screen opens on demand: the
 * recorded output sized to what it printed, or the live terminal while a PTY
 * still runs. The outcome is stated once, exactly as the runtime resolved it.
 */
export function SessionTerminalToolRow(props: SessionTerminalToolRowProps) {
  const facts = readSessionTerminalFacts(props.message);
  const shell = use(OsShellContext);
  // The terminal's scope is not in the tool result — exec and open return a
  // `terminal_id` and nothing about where it lives. It comes from the session
  // this row is inside: its workspace, and *its* profile, read from the session
  // itself rather than from whichever profile the shell happens to be showing.
  // A same-named terminal in another profile is a different terminal.
  const scope = useSessionTerminalScope();
  if (!facts) return null;
  if (!scope) return <TerminalToolRowView {...props} facts={facts} shell={shell} />;
  return <CatalogBackedTerminalToolRow {...props} facts={facts} scope={scope} shell={shell} />;
}

interface TerminalToolRowViewProps extends SessionTerminalToolRowProps {
  facts: SessionTerminalFacts;
  shell: OsShellHandle | null;
  catalog?: TerminalInfo;
  scope?: TerminalScope;
}

function CatalogBackedTerminalToolRow(props: TerminalToolRowViewProps & { scope: TerminalScope }) {
  const catalog = useSessionTerminalCatalogEntry(props.facts.terminalId, props.scope);
  return <TerminalToolRowView {...props} catalog={catalog} />;
}

/** The envelope's duration, else the catalog's own start-to-exit span. */
function runDurationMs(facts: SessionTerminalFacts, catalog: TerminalInfo | undefined) {
  if (facts.durationMs !== null) return facts.durationMs;
  const started = catalog?.created_at ? Date.parse(catalog.created_at) : Number.NaN;
  const ended = catalog?.exit?.at ? Date.parse(catalog.exit.at) : Number.NaN;
  const span = ended - started;
  return Number.isFinite(span) && span > 0 ? span : null;
}

function terminalRowPresentation(
  message: UIMessage,
  facts: SessionTerminalFacts,
  catalog: TerminalInfo | undefined,
  scope: TerminalScope | undefined
) {
  const run = resolveSessionTerminalRun(facts, catalog);
  const outcome = sessionTerminalOutcome(run, message.toolError === true);
  const title = readNonEmptyString(catalog?.title) ?? facts.title;
  const registryTool = resolveRegisteredToolName(message.toolName ?? "compozy__terminal_exec");
  // An exec still running reads "Running in terminal"; an open terminal was
  // opened — that it is still alive is the spinner's to say, not the verb's.
  const tense =
    outcome.status === "running" && registryTool !== "compozy__terminal_open" ? "active" : "past";
  const verb = getToolLabel(registryTool, tense).replace(/…$/u, "");
  const durationMs = run.exit ? runDurationMs(facts, catalog) : null;
  const statText = [outcome.mark, durationMs !== null ? formatDuration(durationMs) : null]
    .filter(Boolean)
    .join(" · ");
  // A running PTY the catalog confirms streams live once opened; everything
  // else paints the screen the tool result recorded.
  const live = scope !== undefined && catalog?.mode === "pty" && catalog.state === "running";
  return { run, outcome, title, registryTool, verb, statText, live };
}

function TerminalToolRowView({
  message,
  facts,
  catalog,
  scope,
  shell,
  defaultExpanded = false,
  partIndex,
  revealOpen = false,
  onRevealRelease,
  onRail = false,
}: TerminalToolRowViewProps) {
  const [ownExpanded, setOwnExpanded] = useState(defaultExpanded);
  const { run, outcome, title, registryTool, verb, statText, live } = terminalRowPresentation(
    message,
    facts,
    catalog,
    scope
  );
  const expanded = ownExpanded || revealOpen;
  // An open screen that streamed stays a screen when the run ends — its bytes
  // are the final screen. Closing it lets go: a later open paints the record.
  const [streamedOpen, setStreamedOpen] = useState(false);
  if (live && expanded && !streamedOpen) setStreamedOpen(true);
  if (!expanded && streamedOpen) setStreamedOpen(false);
  const hasScreen = live || streamedOpen || facts.preview.trim() !== "";

  return (
    <div
      data-testid="tool-call-row"
      data-part-index={partIndex}
      data-terminal-id={facts.terminalId}
    >
      <ToolCallRow
        toolName={verb}
        icon={onRail ? null : getToolIcon(registryTool)}
        preview={title ? <span title={title}>{title}</span> : undefined}
        previewVariant="chip"
        status={outcome.status}
        bodyVariant={hasScreen ? "plain" : "card"}
        expanded={expanded}
        onExpandedChange={next => {
          if (!next && revealOpen) onRevealRelease?.();
          setOwnExpanded(next);
        }}
        statLabel={statText || undefined}
        stat={statText ? <span className="text-subtle">{statText}</span> : undefined}
        actions={shell ? <OpenTerminalAction shell={shell} terminalId={facts.terminalId} /> : null}
      >
        {hasScreen ? (
          <Suspense
            fallback={
              <DetailPre data-testid="terminal-content-fallback">{facts.preview}</DetailPre>
            }
          >
            <SessionTerminalBlock
              blockId={message.id}
              exit={run.exit}
              preview={facts.preview}
              terminalId={facts.terminalId}
              title={title ?? "Terminal"}
              // Kept while a streamed screen stays open: the scope is part of
              // the emulator's identity, and dropping it would remount it blank.
              {...((live || streamedOpen) && scope ? { scope } : {})}
            />
          </Suspense>
        ) : (
          <p
            className="font-mono text-transcript-caption text-subtle"
            data-testid="terminal-content-empty"
          >
            {run.exit ? "No output recorded" : "No output yet"}
          </p>
        )}
      </ToolCallRow>
    </div>
  );
}

function OpenTerminalAction({ shell, terminalId }: { shell: OsShellHandle; terminalId: string }) {
  return (
    <Button
      aria-label="Open terminal"
      className="text-subtle hover:text-fg"
      data-testid={`session-terminal-open-${terminalId}`}
      onClick={() => {
        void shell.coordinator.userOpen({
          app: "terminal",
          instanceKey: terminalId,
          route: {
            pathname: `/terminal/${encodeURIComponent(terminalId)}`,
            search: {},
          },
        });
      }}
      size="icon-xs"
      title="Open terminal"
      type="button"
      variant="ghost"
    >
      <ArrowUpRight aria-hidden="true" className="size-3" />
    </Button>
  );
}
