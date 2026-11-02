/**
 * The screens the linked work prototypes review: the real views in the draft shell, which has no
 * production equivalent that renders without a daemon. The fixtures stand in for the server: they
 * search here because no server answers a prototype.
 */
import { Link2 } from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import {
  Command,
  CommandDialog,
  CommandInput,
  CommandList,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  Icon,
  StateGlyph,
  type TopbarSlotValue,
} from "@compozy/ui";

import { OsPaletteEntitySections } from "@/systems/os/components/os-palette-entity-sections";
import { OsPaletteFooter } from "@/systems/os/components/os-palette-footer";
import type { OsPaletteEntities } from "@/systems/os/hooks/use-os-palette-entities";
import {
  paletteHeadClass,
  paletteInputRailClass,
  paletteListClass,
} from "@/systems/os/lib/palette-view-inset";
import { SessionBoardCardView } from "@/systems/session/components/session-board/session-board-card-view";
import type { SessionBoardCardProps } from "@/systems/session/components/session-board/session-board-card";
import { SessionBoardLinkedWorkCount } from "@/systems/session/components/session-board/session-board-linked-work-count";
import { SessionEnvironmentChip } from "@/systems/session/components/session-environment-chip";
import { SessionLinkCountMark } from "@/systems/session/components/session-linked-work/session-link-count-mark";
import { SessionLinkDialog } from "@/systems/session/components/session-linked-work/session-link-dialog";
import type { SessionLinkActions } from "@/systems/session/components/session-linked-work/session-link-row";
import {
  SessionLinkedWorkNarrowEntry,
  SessionLinkedWorkStatus,
} from "@/systems/session/components/session-linked-work/session-linked-work-header";
import { sessionBadgeSignal } from "@/systems/session/lib/session-badge";
import { getSessionDisplayTitle } from "@/systems/session/lib/session-display-title";
import {
  repositoryForgeName,
  repositoryShortName,
  sessionLinkExactMatch,
  sessionLinkSearch,
  sessionLinkSearchTerm,
  parseSessionLinkInput,
} from "@/systems/session/lib/session-link-input";
import {
  candidateKey,
  leadLink,
  linkedCandidateKeys,
  linkedSessionFacts,
  moreLabel,
  summaryMoreLabel,
  type SessionLinkedWorkItem,
  type SessionRunsIn,
  type SessionWorkCandidate,
} from "@/systems/session/lib/session-linked-work";
import {
  SESSION_LINK_BRANCHES,
  SESSION_LINK_PULL_REQUESTS,
  SESSION_LINK_SETS,
  SESSION_LINKS_DOZEN,
  sessionLinkFromCandidate,
  sessionLinkSummaryFixture,
} from "@/systems/session/mocks/session-links-fixtures";
import { linkScenario, type LinkScenario } from "@/systems/session/mocks/session-links-scenarios";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";

import { useFixtureLinks } from "./hooks/use-fixture-links";
import { KanbanWindow } from "../stays-on-sessions/kanban";
import { liveColumns } from "../stays-on-sessions/kanban-columns";
import { INSTANCES } from "./fixtures";
import { DraftStopAction, SessionsWindow } from "./session-frame";
import { boardSession, GLOBAL_ROW } from "./sessions-fixtures";
import { AgentTurn, DraftComposer, TranscriptColumn, UserTurn } from "./transcript";

/** What the server would answer, so the dialog's states read as they do in production. */
function fixtureSearch(query: string, offline: boolean) {
  const term = sessionLinkSearchTerm(parseSessionLinkInput(query)).toLowerCase();
  const matches = (candidate: SessionWorkCandidate) =>
    [candidate.title, candidate.branch, String(candidate.pr_number ?? "")].some(value =>
      value?.toLowerCase().includes(term)
    );
  return {
    pullRequests: offline ? [] : SESSION_LINK_PULL_REQUESTS.filter(matches),
    branches: SESSION_LINK_BRANCHES.filter(matches),
  };
}

export interface LinkDialogPreset {
  query?: string;
  selected?: readonly number[];
  offline?: boolean;
}

const REPOSITORY = SESSION_LINK_PULL_REQUESTS[0]!.repository;

function FixtureLinkDialog({
  items,
  preset = {},
  onLink,
  onOpenChange,
}: {
  items: readonly SessionLinkedWorkItem[];
  preset?: LinkDialogPreset;
  onLink: (added: readonly SessionLinkedWorkItem[]) => void;
  onOpenChange: (open: boolean) => void;
}) {
  const [query, setQuery] = useState(preset.query ?? "");
  const offline = preset.offline === true;
  const { pullRequests, branches } = fixtureSearch(query, offline);
  const [picked, setPicked] = useState<readonly SessionWorkCandidate[]>(() =>
    SESSION_LINK_PULL_REQUESTS.filter(row => preset.selected?.includes(row.pr_number ?? 0))
  );
  const search = sessionLinkSearch({
    query,
    repository: REPOSITORY,
    pullRequests,
    branches,
    searching: false,
    refused: false,
  });
  const exact = sessionLinkExactMatch(query, pullRequests);
  const selected =
    exact && !picked.some(row => candidateKey(row) === candidateKey(exact))
      ? [...picked, exact]
      : picked;
  return (
    <SessionLinkDialog
      error={null}
      forge={repositoryForgeName(REPOSITORY)}
      hasMore={false}
      linked={linkedCandidateKeys(items)}
      loading={false}
      offline={offline}
      onConfirm={() => {
        onLink(selected.map(sessionLinkFromCandidate));
        onOpenChange(false);
      }}
      onLoadMore={() => undefined}
      onOpenChange={onOpenChange}
      onQueryChange={setQuery}
      onToggle={candidate =>
        setPicked(current =>
          current.some(row => candidateKey(row) === candidateKey(candidate))
            ? current.filter(row => candidateKey(row) !== candidateKey(candidate))
            : [...current, candidate]
        )
      }
      pending={false}
      project={repositoryShortName(REPOSITORY)}
      query={query}
      search={search}
      selected={selected}
    />
  );
}

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

function Stage({
  scenario,
  items,
  status,
  headAction,
  environmentOpen = false,
  dialog,
  dialogOpen,
  onDialogOpenChange,
  onAdd,
  overlay,
}: {
  scenario: LinkScenario;
  items: readonly SessionLinkedWorkItem[];
  status: ReactNode;
  headAction?: ReactNode;
  environmentOpen?: boolean;
  dialog?: LinkDialogPreset;
  dialogOpen: boolean;
  onDialogOpenChange: (open: boolean) => void;
  onAdd: (added: readonly SessionLinkedWorkItem[]) => void;
  overlay?: ReactNode;
}) {
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
    if (id === scenario.sessionId) {
      return <SessionLinkCountMark summary={sessionLinkSummaryFixture(items)} />;
    }
    return id === "s1" ? (
      <SessionLinkCountMark summary={sessionLinkSummaryFixture(SESSION_LINKS_DOZEN)} />
    ) : null;
  };
  const screen = (
    <SessionsWindow
      overlay={
        <>
          {overlay}
          {dialogOpen ? (
            <FixtureLinkDialog
              items={items}
              onLink={onAdd}
              onOpenChange={onDialogOpenChange}
              preset={dialog}
            />
          ) : null}
        </>
      }
      rail={{ groups: [{ ...COMPOZY_GROUP, currentRow: scenario.sessionId }], top: GLOBAL_ROW }}
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

/** The session head at one reviewed state. */
export function SessionLinksHeaderStage({ state }: { state: string }) {
  const scenario = linkScenario(state);
  const { items, unlink, add } = useFixtureLinks(scenario.items);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [clipped, setClipped] = useState(false);
  const lead = leadLink(items);
  const more = moreLabel(items);
  const actions: SessionLinkActions = {
    controllable: true,
    refreshing: false,
    onLink: () => setDialogOpen(true),
    onUnlink: unlink,
    onOpenSession: () => undefined,
    onRefresh: () => undefined,
    onPrompt: () => undefined,
  };
  const list = {
    actions,
    agentLabel: "Claude",
    items,
    listOpen: scenario.open && !scenario.narrow,
    openDetailFor: scenario.openDetailFor,
    openMenuFor: scenario.openMenuFor,
    runsIn: scenario.runsIn,
  };
  return (
    <Stage
      dialogOpen={dialogOpen}
      environmentOpen={state === "none"}
      headAction={
        clipped ? <SessionLinkedWorkNarrowEntry {...list} listOpen={scenario.open} /> : undefined
      }
      items={items}
      onAdd={add}
      onDialogOpenChange={setDialogOpen}
      scenario={scenario}
      status={
        lead ? (
          <SessionLinkedWorkStatus
            {...list}
            chipMenuOpen={items.length === 1 && scenario.open}
            lead={lead}
            leadIsOwn={false}
            more={more}
            onClippedChange={setClipped}
            ownChip={{}}
            ownFacts={{ branch: "" }}
            worktreeAction={() => () => undefined}
          />
        ) : null
      }
    />
  );
}

const LINKS_BY_SESSION: Record<string, readonly SessionLinkedWorkItem[]> = {
  s1: SESSION_LINKS_DOZEN,
  s2: SESSION_LINK_SETS.mixed,
};

function LinksCard({ actions: _actions, session, ...props }: SessionBoardCardProps) {
  const items = LINKS_BY_SESSION[session.id] ?? [];
  const summary = items.length > 0 ? sessionLinkSummaryFixture(items) : null;
  const more = summary ? summaryMoreLabel(summary) : null;
  const linked = summary ? { ...session, linked_work: summary } : session;
  return (
    <SessionBoardCardView
      {...props}
      metaExtra={more ? <SessionBoardLinkedWorkCount label={more} /> : undefined}
      runtime={boardRuntimeFixture(session)}
      session={linkedSessionFacts(linked)}
    />
  );
}

/** The real board: a card keeps its lead PR and gains `+N PRs`. */
export function SessionLinksBoardStage() {
  return (
    <KanbanWindow
      card={LinksCard}
      lanes={[{ name: "compozy", count: 6, columns: liveColumns() }]}
      top={GLOBAL_ROW}
    />
  );
}

const DIALOG_SCENARIO = { ...linkScenario("mixed-rest"), items: SESSION_LINK_SETS.mixed };

/** The link dialog over the session it links to; every entry opens this same dialog. */
export function SessionLinksDialogStage({ preset }: { preset: LinkDialogPreset }) {
  const { items, add } = useFixtureLinks(SESSION_LINK_SETS.mixed);
  const [open, setOpen] = useState(true);
  return (
    <Stage
      dialog={preset}
      dialogOpen={open}
      items={items}
      onAdd={add}
      onDialogOpenChange={setOpen}
      scenario={DIALOG_SCENARIO}
      status={null}
    />
  );
}

const FOUND: OsPaletteEntities = {
  sessions: [
    {
      sessionId: "s4",
      title: "Topbar PR chip — web slice",
      agentName: "claude",
      pullRequestLabel: "#944",
      workspaceLabel: "compozy",
      busy: false,
      workspaceId: "ws-compozy",
      route: { pathname: "/sessions/s4", search: {} },
    },
  ],
  sessionTotal: 1,
  tabs: [],
  tabTotal: 0,
  worktrees: [],
  worktreeTotal: 0,
  selectWorktree: () => undefined,
};

/** The real palette rows: a PR number finds its session, and linking is one command away. */
export function SessionLinksPaletteStage({ mode }: { mode: "find" | "link" }) {
  const { items, add } = useFixtureLinks(DIALOG_SCENARIO.items);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [open, setOpen] = useState(true);
  const entities: OsPaletteEntities =
    mode === "find"
      ? FOUND
      : { ...FOUND, sessions: [], sessionTotal: 0, linkSessionId: DIALOG_SCENARIO.sessionId };
  return (
    <Stage
      dialogOpen={dialogOpen}
      items={items}
      onAdd={add}
      onDialogOpenChange={setDialogOpen}
      overlay={
        <CommandDialog
          className="sm:max-w-160"
          onOpenChange={setOpen}
          open={open}
          title="Command palette"
        >
          <Command className={paletteInputRailClass} shouldFilter={false}>
            <div className={paletteHeadClass}>
              <CommandInput
                placeholder="Search apps, sessions, and actions…"
                value={mode === "find" ? "#944" : "link"}
                variant="quiet"
              />
            </div>
            <CommandList
              className={`${paletteListClass} **:data-[testid=os-palette-section-sessions]:mt-0 **:data-[testid=os-palette-section-sessions]:border-t-0 **:data-[testid=os-palette-section-sessions]:pt-0 **:data-[testid=os-palette-section-session-link]:mt-0 **:data-[testid=os-palette-section-session-link]:border-t-0 **:data-[testid=os-palette-section-session-link]:pt-0`}
            >
              <OsPaletteEntitySections
                destination={false}
                entities={entities}
                onGoToTab={() => undefined}
                onLinkSession={() => {
                  setOpen(false);
                  setDialogOpen(true);
                }}
                onOpenSession={() => undefined}
                onSelectWorktree={() => undefined}
              />
            </CommandList>
            <OsPaletteFooter enterHint="open" />
          </Command>
        </CommandDialog>
      }
      scenario={DIALOG_SCENARIO}
      status={null}
    />
  );
}
