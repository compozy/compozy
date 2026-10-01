import { Button, Spinner } from "@compozy/ui";

import { AgentCommandSelect, type AgentPayload } from "@/systems/agent";
import { GLOBAL_SCOPE_COPY } from "@/systems/workspace";

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
  /** Global scope has no project to start a session in. */
  hasProject: boolean;
  /** A session is starting; the composer rests and the status says so. */
  starting: boolean;
  /** This project's unsent-prompt slot ({@link emptyDesktopDraftId}). */
  draftId: string;
  /** Receives the sent prompt; `release` clears it once a session owns it. */
  onSubmit: OsEmptyDesktopComposerProps["onSubmit"];
  agents: AgentPayload[];
  /** The agent the prompt starts with; `null` until one is known. */
  agentName: string | null;
  onAgentChange: (agentName: string | null) => void;
  agentsLoading?: boolean;
  agentsError?: string | null;
  /** Opens the project picker — the way forward from Global scope. */
  onPickProject?: () => void;
}

/**
 * VC-10 — an empty desktop is a place to start: one plain question and the
 * session composer under it, with the palette as the quiet way to open
 * anything else. No card, no modal; the desk itself is the surface.
 */
export function OsEmptyDesktopView({
  desktopName,
  paletteShortcutLabel,
  hasProject,
  starting,
  draftId,
  onSubmit,
  agents,
  agentName,
  onAgentChange,
  agentsLoading = false,
  agentsError = null,
  onPickProject,
}: OsEmptyDesktopViewProps) {
  const control = hasProject ? (
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
  ) : onPickProject ? (
    <Button type="button" variant="secondary" onClick={onPickProject}>
      Pick a project
    </Button>
  ) : null;

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
          canPrompt={hasProject && !starting}
          placeholder={PROMPT_PLACEHOLDER}
          inactivePlaceholder={
            hasProject ? STARTING_LABEL : GLOBAL_SCOPE_COPY.newSessionNeedsProject
          }
          control={control}
          typeToFocus
        />
        <p
          role="status"
          className="flex min-h-5 items-center gap-1.5 px-4 text-eyebrow text-muted"
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
  /** Global scope has no project to start a session in. */
  hasProject: boolean;
  /** Opens the project picker — the way forward from Global scope. */
  onPickProject?: () => void;
}

/**
 * The empty desktop, live: sending the first message starts a session with
 * it and opens that session here, so the new window fills this desktop.
 */
export function OsEmptyDesktop({
  desktopName,
  paletteShortcutLabel,
  hasProject,
  onPickProject,
}: OsEmptyDesktopProps) {
  const session = useEmptyDesktopSession();
  return (
    <OsEmptyDesktopView
      desktopName={desktopName}
      paletteShortcutLabel={paletteShortcutLabel}
      hasProject={hasProject}
      starting={session.pending}
      draftId={emptyDesktopDraftId(session.workspaceId)}
      onSubmit={session.start}
      agents={session.agents}
      agentName={session.agentName}
      onAgentChange={session.selectAgent}
      agentsLoading={session.agentsLoading}
      agentsError={session.agentsError}
      onPickProject={onPickProject}
    />
  );
}
