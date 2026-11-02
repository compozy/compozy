/**
 * The full screen every direction sits in: the real shell, sessions rail, head, transcript and
 * composer. A direction only decides what goes in the head and where the list lives.
 */
import { Link2 } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  Icon,
  StateGlyph,
  type TopbarSlotValue,
} from "@compozy/ui";

import { SessionEnvironmentChip } from "@/systems/session/components/session-environment-chip";
import { sessionBadgeSignal } from "@/systems/session/lib/session-badge";
import { getSessionDisplayTitle } from "@/systems/session/lib/session-display-title";
import {
  SESSION_LINKS_DOZEN,
  SESSION_LINK_CANDIDATES,
  type SessionLink,
  type SessionRunsIn,
} from "@/systems/session/mocks/session-links-fixtures";

import { DraftStopAction, SessionsWindow } from "../stays-on-agents-shared/session-frame";
import { boardSession } from "../stays-on-agents-shared/sessions-fixtures";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
} from "../stays-on-agents-shared/transcript";
import { INSTANCES } from "../stays-on-agents-shared/fixtures";
import { GLOBAL_ROW } from "../stays-on-agents-shared/sessions-fixtures";
import { LinkCountMark } from "./link-count-mark";
import { LinkDialog, type LinkDialogPreset } from "./link-dialog";
import type { LinkScenario } from "./scenarios";

function EnvironmentControl({
  place,
  open,
  onLink,
}: {
  place: SessionRunsIn;
  open: boolean;
  onLink: () => void;
}) {
  return (
    <SessionEnvironmentChip
      defaultMenuOpen={open}
      label={place.name}
      menu={
        <>
          <DropdownMenuGroup>
            <DropdownMenuLabel className="tracking-normal normal-case">
              This session runs in {place.name}
            </DropdownMenuLabel>
            <DropdownMenuItem>
              {place.kind === "worktree" ? "Change worktree…" : "Use worktree…"}
            </DropdownMenuItem>
            <DropdownMenuItem>Fork into a worktree…</DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem className="whitespace-nowrap" onClick={onLink}>
            <Icon as={Link2} size="sm" />
            Link a pull request or branch…
          </DropdownMenuItem>
        </>
      }
      state={place.kind === "worktree" ? "worktree" : "root"}
    />
  );
}

const COMPOZY_GROUP = {
  name: "compozy",
  count: 4,
  rows: ["s1"],
  threads: { s1: ["s4", "s2", "s9"] },
  instances: [INSTANCES.orchestratorCompozy, INSTANCES.triageCompozy],
};

export interface LinkStageProps {
  scenario: LinkScenario;
  links: readonly SessionLink[];
  /** The head's status zone. */
  status: ReactNode;
  /** Beside the stop action; the head keeps this side when it clips the status zone. */
  headAction?: ReactNode;
  /** The composer's environment menu starts open. */
  environmentOpen?: boolean;
  /** The link dialog is showing from the start, with this search in it. */
  dialog?: LinkDialogPreset;
  dialogOpen: boolean;
  onDialogOpenChange: (open: boolean) => void;
  onAdd: (added: readonly SessionLink[]) => void;
  overlay?: ReactNode;
}

export function LinkStage({
  scenario,
  links,
  status,
  headAction,
  environmentOpen = false,
  dialog,
  dialogOpen,
  onDialogOpenChange,
  onAdd,
  overlay,
}: LinkStageProps) {
  const session = boardSession(scenario.sessionId);
  const topbar: TopbarSlotValue = {
    glyph: <StateGlyph size="sm" state={sessionBadgeSignal(session.badge).state} />,
    glyphPresentation: "state",
    crumb: getSessionDisplayTitle(session),
    status,
    actions: (
      <>
        {headAction}
        <DraftStopAction />
      </>
    ),
  };
  const rowExtra = (id: string) => {
    if (id === scenario.sessionId) return <LinkCountMark links={links} />;
    return id === "s1" ? <LinkCountMark links={SESSION_LINKS_DOZEN} /> : null;
  };
  const screen = (
    <SessionsWindow
      overlay={
        <>
          {overlay}
          {dialogOpen ? (
            <LinkDialog
              candidates={SESSION_LINK_CANDIDATES}
              linked={links}
              onLink={onAdd}
              onOpenChange={onDialogOpenChange}
              preset={dialog}
            />
          ) : null}
        </>
      }
      rail={{
        groups: [{ ...COMPOZY_GROUP, currentRow: scenario.sessionId }],
        top: GLOBAL_ROW,
      }}
      railClosed={scenario.narrow}
      rowExtra={rowExtra}
      title={getSessionDisplayTitle(session)}
      topbar={topbar}
    >
      <div className="flex min-h-0 min-w-0 flex-1">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <TranscriptColumn>
            <UserTurn>{scenario.user}</UserTurn>
            <AgentTurn>{scenario.agent}</AgentTurn>
          </TranscriptColumn>
          <DraftComposer
            environmentControl={
              <EnvironmentControl
                onLink={() => onDialogOpenChange(true)}
                open={environmentOpen}
                place={scenario.runsIn}
              />
            }
            placeholder="Message Claude"
          />
        </div>
      </div>
    </SessionsWindow>
  );
  return scenario.narrow ? <div className="mx-auto h-dvh max-w-160">{screen}</div> : screen;
}
