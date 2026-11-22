import { FolderOpen, Plus, Send, SquareKanban } from "@compozy/ui/icons";
import { useMemo, useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Textarea,
  type TopbarSlotValue,
} from "@compozy/ui";

import { ComputerMetaLabel } from "@/systems/computers/components/computer-chip";
import { RunOnPicker, type RunOnTarget } from "@/systems/computers/components/run-on-picker";
import { COMPUTERS_PRO, LOCAL_COMPUTER } from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  projectsForTarget,
  REMOTE_SESSION_ENTRIES,
  resolveAutoComputer,
  toBoardSessionPayload,
  type RemoteSessionRowEntry,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import { RuntimeSelector } from "@/systems/runtime/components/runtime-selector";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime/components/runtime-selector";
import type { SessionBoardCardProps } from "@/systems/session/components/session-board/session-board-card";
import { SessionBoardCardSlot } from "@/systems/session/components/session-board/session-board-card-slot";
import { SessionBoardCardView } from "@/systems/session/components/session-board/session-board-card-view";
import { SessionViewToggle } from "@/systems/session/components/session-board/session-view-toggle";
import { SessionsBoardColumnsRow } from "@/systems/session/components/session-board/sessions-board-columns-row";
import {
  SESSIONS_BOARD_FRAME,
  SESSIONS_BOARD_SCROLLER,
} from "@/systems/session/components/session-board/sessions-board-frame";
import type { SessionsBoardColumnView } from "@/systems/session/hooks/use-sessions-board";
import type { SessionBoardColumnId } from "@/systems/session/lib/session-board-contract";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";

import { DraftShell } from "../always-on-agents-shared/draft-shell";
import { RUNTIME_MODELS, RUNTIME_PROVIDERS } from "../always-on-agents-shared/sessions-fixtures";

const ENTRY_BY_ID = new Map<string, RemoteSessionRowEntry>(
  REMOTE_SESSION_ENTRIES.map(entry => [entry.id, entry])
);

function KanbanBackdropCard({ actions: _actions, ...props }: SessionBoardCardProps) {
  const meta = ENTRY_BY_ID.get(props.session.id);
  const offline = Boolean(meta?.computerOffline);
  return (
    <div className={offline ? "opacity-60" : undefined}>
      <SessionBoardCardView
        {...props}
        metaExtra={
          meta && !meta.isLocal ? (
            <ComputerMetaLabel
              className="ml-1"
              iconKind={meta.iconKind}
              name={meta.computerName}
              offline={offline}
              platform={meta.platform}
            />
          ) : undefined
        }
        runtime={boardRuntimeFixture(props.session)}
      />
    </div>
  );
}

const COLUMN_DEFS: ReadonlyArray<{ id: SessionBoardColumnId; label: string }> = [
  { id: "needs_you", label: "Needs you" },
  { id: "working", label: "Working" },
  { id: "in_review", label: "In review" },
  { id: "done", label: "Done" },
];

function buildColumns(): SessionsBoardColumnView[] {
  const payloads = REMOTE_SESSION_ENTRIES.map(toBoardSessionPayload);
  return COLUMN_DEFS.map(def => {
    const sessions = payloads.filter(s => s.board_column === def.id);
    return {
      id: def.id,
      label: def.label,
      count: sessions.length,
      sessions,
      loading: false,
      failure: null,
      retrying: false,
      fetching: false,
      hasMore: false,
      loadingMore: false,
      loadMore: () => undefined,
      retry: () => undefined,
      total: sessions.length,
    };
  });
}

/**
 * Screen 05 Host B — Kanban "New session" dialog with compact `RunOnPicker`
 * immediately before Project and Runtime controls (`_uiux.md` U4).
 */
export function RunOnKanbanNewScene({
  initialTarget = "cmp_1c9d0a2b",
  pickerDefaultOpen = false,
}: {
  initialTarget?: RunOnTarget;
  pickerDefaultOpen?: boolean;
}) {
  const [target, setTarget] = useState<RunOnTarget>(initialTarget);
  const [prompt, setPrompt] = useState(
    "Benchmark the new Metal tokenizer batch path against baseline and post the diff."
  );
  const [runtime, setRuntime] = useState<RuntimeSelectorValue>({
    provider: "claude",
    model: "claude-opus-5-5",
    reasoning_effort: "",
  });
  const [reconnectingIds, setReconnectingIds] = useState<string[]>([]);

  const columns = useMemo(buildColumns, []);
  const projects = useMemo(() => projectsForTarget(target), [target]);
  const [projectName, setProjectName] = useState(projects[0]?.name ?? "api");
  const currentProject = projects.find(p => p.name === projectName) ?? projects[0] ?? null;

  const autoResolution = useMemo(
    () =>
      currentProject
        ? resolveAutoComputer(currentProject.name, LOCAL_COMPUTER, COMPUTERS_PRO)
        : null,
    [currentProject]
  );

  const selectedRemote = COMPUTERS_PRO.find(c => c.id === target) ?? null;
  const isOffline = selectedRemote?.status === "offline";
  const isNoEligible = target === "auto" && !autoResolution;

  const topbar: TopbarSlotValue = {
    crumb: "Sessions",
    overflow: <SessionViewToggle onValueChange={() => undefined} value="kanban" />,
    actions: (
      <Button size="sm" type="button" variant="secondary">
        <Plus aria-hidden="true" data-icon="inline-start" />
        New session
      </Button>
    ),
  };

  const overlay = (
    <Dialog onOpenChange={() => undefined} open>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="kanban-new-session-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          eyebrow="Kanban"
          icon={Plus}
          onClose={() => undefined}
          title="New session"
        />

        <EntityDialogBody className="flex flex-col gap-3.5">
          <div className="flex flex-wrap items-center gap-2">
            <RunOnPicker
              allowAuto
              autoResolution={autoResolution}
              computers={COMPUTERS_PRO}
              defaultOpen={pickerDefaultOpen}
              loadBalancingEnabled
              localComputer={LOCAL_COMPUTER}
              onAddComputer={() => undefined}
              onChange={next => {
                setTarget(next);
                const nextProjects = projectsForTarget(next);
                if (nextProjects[0]) setProjectName(nextProjects[0].name);
              }}
              onReconnect={id => setReconnectingIds(prev => [...prev, id])}
              reconnectingIds={reconnectingIds}
              value={target}
              variant="compact"
            />

            <span className="inline-flex h-7 items-center gap-1.5 rounded-md border border-line bg-surface-2/60 px-2.5 text-eyebrow text-fg">
              <FolderOpen aria-hidden="true" className="size-3.5 text-subtle" />
              <span className="font-medium">{currentProject?.name ?? "api"}</span>
            </span>

            <RuntimeSelector
              models={RUNTIME_MODELS as unknown as RuntimeModelOption[]}
              onChange={setRuntime}
              providers={RUNTIME_PROVIDERS as RuntimeProviderOption[]}
              value={runtime}
              variant="composer"
            />
          </div>

          <Textarea
            aria-label="Prompt"
            onChange={event => setPrompt(event.target.value)}
            placeholder="Describe the work to start…"
            rows={4}
            value={prompt}
          />

          {currentProject ? (
            <p className="font-mono text-badge text-faint">
              {selectedRemote ? `${selectedRemote.name}:` : ""}
              {currentProject.path}
            </p>
          ) : null}
        </EntityDialogBody>

        <EntityDialogFooter
          hint={
            <span className="text-form-hint text-muted-foreground">
              {isOffline && selectedRemote
                ? `${selectedRemote.name} is offline`
                : isNoEligible
                  ? "No computer can run this project"
                  : `Starts in Working on ${
                      target === "auto"
                        ? (autoResolution?.computerName ?? "Auto")
                        : (selectedRemote?.name ?? "This computer")
                    }`}
            </span>
          }
          onCancel={() => undefined}
          primaryDisabled={isOffline || isNoEligible || prompt.trim().length === 0}
          primaryIcon={Send}
          primaryLabel="Send"
        />
      </DialogContent>
    </Dialog>
  );

  return (
    <DraftShell
      app="sessions"
      glyph={<SquareKanban />}
      overlay={overlay}
      scope="All projects"
      title="Sessions"
      topbar={topbar}
    >
      <SessionBoardCardSlot value={KanbanBackdropCard}>
        <div className={SESSIONS_BOARD_FRAME}>
          <div className={SESSIONS_BOARD_SCROLLER}>
            <SessionsBoardColumnsRow
              collapsedColumns={{}}
              columns={columns}
              currentSessionId={null}
              layout="fill"
              onColumnCollapsedChange={() => undefined}
              onOpen={() => undefined}
              workspaceNameOf={session => ENTRY_BY_ID.get(session.id)?.project ?? null}
            />
          </div>
        </div>
      </SessionBoardCardSlot>
    </DraftShell>
  );
}
