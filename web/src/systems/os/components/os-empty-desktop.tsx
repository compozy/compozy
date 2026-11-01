import type { ReactNode } from "react";
import { SessionDraftRuntimeProvider, SessionPromptRuntimeSelector } from "@/systems/session";
import { Spinner } from "@compozy/ui";

import { AgentCommandSelect, type AgentPayload } from "@/systems/agent";

import { emptyDesktopDraftId } from "../hooks/use-empty-desktop-prompt-runtime";
import { useEmptyDesktopSession } from "../hooks/use-empty-desktop-session";
import {
  OsEmptyDesktopComposer,
  type OsEmptyDesktopComposerProps,
} from "./os-empty-desktop-composer";
import { OsShortcutChords } from "./os-shortcut-chords";

const PROMPT_PLACEHOLDER = "Describe a task or ask a question";
const STARTING_LABEL = "Starting a session…";

export interface OsEmptyDesktopViewProps {
  desktopName: string;
  /** Live palette chord ("⌘K"); omitted until the keymap is known. */
  paletteShortcutLabel: string | null;
  /** A session can start here: the destination has settled (a project, User, or All projects asking inline). */
  canStart: boolean;
  /** The agent is chosen in New session with the project (All projects): no agent control here. */
  asksTarget?: boolean;
  /** A session is starting; the composer rests and the status says so. */
  starting: boolean;
  /** This project's unsent-prompt slot ({@link emptyDesktopDraftId}). */
  draftId: string;
  /** Receives the sent prompt; `release` clears it once a session owns it. */
  onSubmit: OsEmptyDesktopComposerProps["onSubmit"];
  runtimeControl?: ReactNode;
  agents: AgentPayload[];
  /** The agent the prompt starts with; `null` until one is known. */
  agentName: string | null;
  onAgentChange: (agentName: string | null) => void;
  agentsLoading?: boolean;
  agentsError?: string | null;
}

/**
 * VC-10 — an empty desktop is a place to start: one plain question and the
 * session composer under it, with the palette as the quiet way to open
 * anything else. No card, no modal; the desk itself is the surface.
 */
export function OsEmptyDesktopView({
  desktopName,
  paletteShortcutLabel,
  canStart,
  starting,
  draftId,
  onSubmit,
  agents,
  agentName,
  onAgentChange,
  agentsLoading = false,
  agentsError = null,
  runtimeControl,
  asksTarget = false,
}: OsEmptyDesktopViewProps) {
  const control =
    asksTarget || !canStart ? null : (
      <AgentCommandSelect
        agents={agents}
        value={agentName}
        onChange={onAgentChange}
        disabled={starting}
        loading={agentsLoading}
        error={agentsError}
        placeholder="Choose an agent"
        triggerTestId="os-desk-agent-select"
        variant="composer"
      />
    );

  return (
    <section
      aria-label={desktopName}
      data-testid="os-desk-hint"
      className="absolute inset-0 flex flex-col items-center overflow-y-auto px-2 pt-12 pb-24"
    >
      <div className="my-auto flex w-full max-w-3xl flex-col items-center">
        <h2 className="mb-3 px-4 text-center text-display-2xl font-medium tracking-empty-h1 text-balance text-fg">
          What should we work on?
        </h2>
        <OsEmptyDesktopComposer
          key={draftId}
          draftId={draftId}
          onSubmit={onSubmit}
          canPrompt={canStart && !starting}
          placeholder={PROMPT_PLACEHOLDER}
          inactivePlaceholder={canStart ? STARTING_LABEL : PROMPT_PLACEHOLDER}
          control={
            <div className="flex min-w-0 flex-wrap items-center gap-1">
              {control}
              {runtimeControl}
            </div>
          }
          typeToFocus
        />
        <p
          role="status"
          className="flex min-h-5 items-center gap-1.5 px-4 text-eyebrow text-muted-foreground"
          data-testid="os-desk-status"
        >
          {starting ? (
            <>
              <Spinner aria-hidden="true" className="size-3" />
              {STARTING_LABEL}
            </>
          ) : paletteShortcutLabel ? (
            <>
              Press <OsShortcutChords label={paletteShortcutLabel} /> to open anything
            </>
          ) : null}
        </p>
      </div>
    </section>
  );
}

export interface OsEmptyDesktopProps {
  desktopName: string;
  /** Live palette chord ("⌘K"); omitted until the keymap is known. */
  paletteShortcutLabel: string | null;
  /** A session can start here: the destination has settled (a project, User, or All projects asking inline). */
  canStart: boolean;
}

/**
 * The empty desktop, live: sending the first message starts a session with
 * it and opens that session here, so the new window fills this desktop.
 */
export function OsEmptyDesktop({
  desktopName,
  paletteShortcutLabel,
  canStart,
}: OsEmptyDesktopProps) {
  const session = useEmptyDesktopSession();
  return (
    <SessionDraftRuntimeProvider
      draftId={emptyDesktopDraftId(session.workspaceId)}
      workspaceId={session.workspaceId ?? ""}
      agentName={session.agentName ?? ""}
      canPrompt={canStart && !session.pending}
    >
      <OsEmptyDesktopView
        desktopName={desktopName}
        paletteShortcutLabel={paletteShortcutLabel}
        canStart={canStart}
        asksTarget={session.asksTarget}
        starting={session.pending}
        draftId={emptyDesktopDraftId(session.workspaceId)}
        onSubmit={session.start}
        runtimeControl={
          canStart && !session.asksTarget ? (
            <SessionPromptRuntimeSelector canPrompt={!session.pending} />
          ) : undefined
        }
        agents={session.agents}
        agentName={session.agentName}
        onAgentChange={session.selectAgent}
        agentsLoading={session.agentsLoading}
        agentsError={session.agentsError}
      />
    </SessionDraftRuntimeProvider>
  );
}
