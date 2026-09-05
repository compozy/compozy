import { formatDuration } from "@compozy/ui";
import { Suspense, lazy, use } from "react";

import { OsShellContext, type OsShellHandle } from "@/systems/os";
import { type TerminalInfo, type TerminalSignal } from "@/systems/terminal/parts";
import { useSessionTerminalCatalogEntry } from "../../hooks/use-session-terminal-catalog";
import { useSessionTerminalScope } from "../../hooks/use-session-terminal-scope";
import {
  asRecord,
  readNonEmptyString,
  readSupervisedTerminalId,
  readTerminalEnvelope,
  resolveSessionTerminalRun,
} from "../../lib/session-terminal-tools";
import type { UIMessage } from "../../types";
import { DetailPre } from "./detail-pre";

/**
 * The emulator arrives only when a terminal block is actually on screen.
 *
 * A session that never runs a terminal command must not download the emulator,
 * so the block sits behind a lazy boundary and the plain output stands in until
 * it lands.
 */
const SessionTerminalBlock = lazy(async () => {
  const { SessionTerminalBlock: Block } = await import("@/systems/terminal");
  return { default: Block };
});

interface TerminalToolFacts {
  terminalId: string;
  title: string;
  preview: string;
  exitCode: number | null;
  signal: TerminalSignal | null;
  stillRunning: boolean;
  durationMs: number | null;
}

/**
 * Reads the tool call's own record.
 *
 * The terminal fields ride the raw tool envelope rather than the normalized
 * `ToolUseResult`, so they are read from there and narrowed one field at a time.
 * Nothing is inferred: a field the runtime did not send stays absent.
 */
function readTerminalFacts(message: UIMessage): TerminalToolFacts | null {
  const result = message.toolResult;
  const raw = readTerminalEnvelope(result?.rawOutput);
  const terminalId = readSupervisedTerminalId(result?.rawOutput);
  if (!terminalId) return null;
  const exitCode = typeof raw.exit_code === "number" ? raw.exit_code : null;
  const signal = readTerminalSignal(raw.signal);
  const durationMs =
    typeof raw.duration_ms === "number" && Number.isFinite(raw.duration_ms)
      ? raw.duration_ms
      : null;
  return {
    terminalId,
    title: readNonEmptyString(asRecord(message.toolInput).title) ?? "Terminal",
    preview: readNonEmptyString(raw.output) ?? result?.stdout ?? result?.content ?? "",
    exitCode,
    signal,
    durationMs,
    stillRunning:
      raw.still_running === true ||
      (message.toolName === "compozy__terminal_open" && exitCode === null && signal === null),
  };
}

function readTerminalSignal(value: unknown): TerminalSignal | null {
  switch (value) {
    case "INT":
    case "TERM":
    case "KILL":
    case "HUP":
      return value;
    default:
      return null;
  }
}

const CLOCK_FORMAT = new Intl.DateTimeFormat(undefined, {
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

function formatClock(iso: string): string | null {
  const parsed = Date.parse(iso);
  if (!Number.isFinite(parsed)) return null;
  return CLOCK_FORMAT.format(new Date(parsed));
}

function durationLabelFor(
  durationMs: number | null,
  finishedAt: string | null
): string | undefined {
  const parts: string[] = [];
  if (durationMs !== null && durationMs > 0) parts.push(formatDuration(durationMs));
  const finished = finishedAt ? formatClock(finishedAt) : null;
  if (finished) parts.push(`finished ${finished}`);
  return parts.length > 0 ? parts.join(" · ") : undefined;
}

/**
 * A terminal tool call, rendered as the terminal it ran in.
 *
 * Deliberate terminal use is a window into a live surface, not a second
 * transcript — so the block shows the screen and points at the terminal, and the
 * outcome is stated exactly as the runtime reported it.
 */
export function TerminalContent({ message }: { message: UIMessage }) {
  const facts = readTerminalFacts(message);
  if (!facts) {
    // A pipe exec finishes without a terminal object at all; that is a plain
    // command result and the generic pre keeps it readable rather than dressing
    // it as a window that never existed. Catalog hooks stay off this path.
    return <PipeTerminalOutput message={message} />;
  }
  return <SupervisedTerminalContent facts={facts} message={message} />;
}

function PipeTerminalOutput({ message }: { message: UIMessage }) {
  const raw = readTerminalEnvelope(message.toolResult?.rawOutput);
  const output =
    readNonEmptyString(raw.output) ??
    message.toolResult?.stdout ??
    message.toolResult?.content ??
    "";
  return output ? <DetailPre data-testid="terminal-content-plain">{output}</DetailPre> : null;
}

function SupervisedTerminalContent({
  facts,
  message,
}: {
  facts: TerminalToolFacts;
  message: UIMessage;
}) {
  const shell = use(OsShellContext);
  // The terminal's scope is not in the tool result — exec and open return a
  // `terminal_id` and nothing about where it lives. It comes from the session
  // this block is inside: its workspace, and *its* profile, read from the
  // session itself rather than from whichever profile the shell happens to be
  // showing. A conversation opened from a link, or read under the all-profiles
  // view, still belongs to the profile that started it, and a same-named
  // terminal in another profile is a different terminal.
  const scope = useSessionTerminalScope();
  if (!scope) {
    return <SessionTerminalPreview facts={facts} message={message} shell={shell} />;
  }
  return (
    <CatalogBackedTerminalPreview facts={facts} message={message} scope={scope} shell={shell} />
  );
}

function CatalogBackedTerminalPreview({
  facts,
  message,
  scope,
  shell,
}: {
  facts: TerminalToolFacts;
  message: UIMessage;
  scope: { workspaceId: string; profile: string };
  shell: OsShellHandle | null;
}) {
  const catalog = useSessionTerminalCatalogEntry(facts.terminalId, scope);
  return (
    <SessionTerminalPreview
      catalog={catalog}
      facts={facts}
      message={message}
      scope={scope}
      shell={shell}
    />
  );
}

function SessionTerminalPreview({
  catalog,
  facts,
  message,
  scope,
  shell,
}: {
  catalog?: TerminalInfo;
  facts: TerminalToolFacts;
  message: UIMessage;
  scope?: { workspaceId: string; profile: string };
  shell: OsShellHandle | null;
}) {
  const title = readNonEmptyString(catalog?.title) ?? facts.title;
  const run = resolveSessionTerminalRun(facts, catalog);
  const durationLabel = durationLabelFor(facts.durationMs, run.exit?.at ?? null);
  const startedLabel = catalog?.created_at ? formatClock(catalog.created_at) : undefined;
  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="terminal-content">
      <Suspense
        fallback={<DetailPre data-testid="terminal-content-fallback">{facts.preview}</DetailPre>}
      >
        <SessionTerminalBlock
          blockId={message.id}
          exit={run.exit}
          preview={facts.preview}
          stillRunning={run.stillRunning}
          terminalId={facts.terminalId}
          title={title}
          {...(durationLabel ? { durationLabel } : {})}
          {...(startedLabel ? { startedLabel } : {})}
          {...(shell
            ? {
                onOpenTerminal: () => {
                  void shell.coordinator.userOpen({
                    app: "terminal",
                    instanceKey: facts.terminalId,
                    route: {
                      pathname: `/terminal/${encodeURIComponent(facts.terminalId)}`,
                      search: {},
                    },
                  });
                },
              }
            : {})}
          {...(scope ? { scope } : {})}
        />
      </Suspense>
    </div>
  );
}
