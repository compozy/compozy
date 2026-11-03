/**
 * Issue #1191: three ways to quiet the session list, against today's row. Each state renders the
 * real `SessionListRowView` twice, built-ins on a gray plate (left) and bare (right).
 */
import { useState, type ReactNode } from "react";

import {
  AgentAvatar,
  Eyebrow,
  SymbolPicker,
  identitySurfaceFor,
  type AgentAvatarAppearance,
  type SymbolBackground,
  type SymbolValue,
} from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { AgentMark } from "@/systems/agent/components/agent-mark";
import { AgentIdentityCatalogContext } from "@/systems/agent/lib/agent-identity-context";
import {
  agentSymbolAppearance,
  agentSymbolBackgroundOf,
  agentSymbolOf,
} from "@/systems/agent/lib/agent-symbol-appearance";
import {
  PROFILE_EMOJIBASE_URL,
  PROFILE_IDENTITY_SWATCHES,
  PROFILE_SPRITE_URL,
} from "@/systems/profiles/lib/profile-identity";
import { SessionLinkCountMark } from "@/systems/session/components/session-linked-work/session-link-count-mark";
import { SessionListBranch } from "@/systems/session/components/session-list/session-list-branch";
import {
  BRANCH_NODE,
  BRANCH_RAIL,
} from "@/systems/session/components/session-list/session-list-branch-rail";
import { SessionListBranchToggle } from "@/systems/session/components/session-list/session-list-branch-toggle";
import { SessionListFolder } from "@/systems/session/components/session-list/session-list-folder";
import { SessionListGroupToggle } from "@/systems/session/components/session-list/session-list-group-toggle";
import { SessionListSubagentRow } from "@/systems/session/components/session-list/session-list-subagent-row";
import { roster } from "@/systems/session/components/subagents/stories/subagent-story-fixtures";
import {
  SessionListRowView,
  type SessionRowLayout,
} from "@/systems/session/components/session-list/session-list-row-view";
import { SessionRowActions } from "@/systems/session/components/session-row-actions";
import { childSessionSignalState } from "@/systems/session/lib/session-hierarchy";
import { sessionSubagentCounts } from "@/systems/session/lib/session-subagent-summary";
import type { SessionThreadSegment } from "@/systems/session/lib/session-thread-branch";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import {
  SESSION_ENTRY_PRELOAD_FIXTURE,
  sessionLifecycleActionsFixture,
} from "@/systems/session/mocks/session-row-fixtures";
import type { SessionPayload } from "@/systems/session/types";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { COMPOZY_ROW } from "../stays-on-agents-shared/sessions-fixtures";
import { StaysOnRow } from "../stays-on-agents-shared/stays-on-row";
import { useIconCatalog } from "../agent-create/hooks/use-icon-catalog";
import { useCanvasTheme } from "../stays-on-agents-shared/use-canvas-theme";
import {
  BUILTIN_GRAY,
  FOLDER_ROWS,
  SESSIONS,
  STARTED_BY_CAPTAIN,
  THREAD_CHILDREN,
  agentCatalog,
  builtinAppearance,
  type IconBackground,
} from "./fixtures";

export const meta = definePrototype({
  title: "Session list declutter (#1191)",
  route: "/sessions",
  states: [
    { id: "before", label: "Today" },
    { id: "trimmed", label: "A · Trimmed two lines" },
    { id: "single-line", label: "B · One line" },
    { id: "single-line-subagents", label: "B · One line, subagents open" },
    { id: "face", label: "C · Face first" },
    { id: "editor", label: "Appearance editor: no background" },
  ],
});

const actions = sessionLifecycleActionsFixture();

/** Wider than today's 264 px default, as the operator asked to review it. */
const SIDEBAR_WIDTH = 360;

/** The orchestrator's direct subagents: one waiting on you, one failed, one working, one done. */
const SUBAGENTS = [roster[2]!, roster[0]!, roster[3]!, roster[5]!];

const CAPTAIN_FACE: AgentAvatarAppearance = { avatar: "generated", seed: "release-captain.2" };

const LAYOUTS: Record<string, SessionRowLayout> = {
  before: "classic",
  trimmed: "trimmed",
  "single-line": "single-line",
  "single-line-subagents": "single-line",
  face: "face",
};

/** Today's row carries the link count and the "Started by" face; the variations drop both. */
function todayMarks(session: SessionPayload) {
  const summary = session.linked_work?.total ? session.linked_work : null;
  const started = STARTED_BY_CAPTAIN.has(session.id);
  if (!summary && !started) return null;
  return (
    <>
      {summary ? <SessionLinkCountMark summary={summary} /> : null}
      {started ? (
        <AgentMark
          appearance={CAPTAIN_FACE}
          label="Started by Release captain"
          quietIdle
          size={16}
          surfaceClassName="ring-sunken"
        />
      ) : null}
    </>
  );
}

function Row({
  session,
  layout,
  branch,
}: {
  session: SessionPayload;
  layout: SessionRowLayout;
  branch?: ReactNode;
}) {
  const marks = layout === "classic" ? todayMarks(session) : null;
  return (
    <SessionListRowView
      layout={layout}
      onSelect={() => undefined}
      preloadHandlers={SESSION_ENTRY_PRELOAD_FIXTURE}
      rowActions={<SessionRowActions actions={actions} session={session} />}
      runtime={boardRuntimeFixture(session)}
      session={session}
      sessionActions={actions}
      testIdPrefix="declutter"
      trailing={
        marks || branch ? (
          <>
            {marks}
            {branch}
          </>
        ) : undefined
      }
    />
  );
}

function Thread({
  layout,
  initialSegment,
}: {
  layout: SessionRowLayout;
  initialSegment: SessionThreadSegment;
}) {
  const [segment, setSegment] = useState<SessionThreadSegment | null>(initialSegment);
  const root = SESSIONS.orchestrator;
  const branchId = `declutter-branch-${layout}`;
  return (
    <div>
      <Row
        branch={
          <SessionListBranchToggle
            branchId={branchId}
            childCount={THREAD_CHILDREN.length}
            childSignal={segment === "children" ? null : childSessionSignalState(THREAD_CHILDREN)}
            onChange={setSegment}
            segment={segment}
            session={root}
            subagentCounts={sessionSubagentCounts(root)}
            testIdPrefix="declutter"
          />
        }
        layout={layout}
        session={root}
      />
      <div id={branchId}>
        <SessionListBranch
          render={shown => (
            <div className={BRANCH_RAIL}>
              {shown === "subagents"
                ? SUBAGENTS.map(subagent => (
                    <SessionListSubagentRow current={false} key={subagent.id} subagent={subagent} />
                  ))
                : THREAD_CHILDREN.map(child => (
                    <div className={BRANCH_NODE} key={child.id}>
                      <Row layout={layout} session={child} />
                    </div>
                  ))}
            </div>
          )}
          segment={segment}
        />
      </div>
    </div>
  );
}

function Folder({ layout }: { layout: SessionRowLayout }) {
  const [expanded, setExpanded] = useState(true);
  return (
    <SessionListFolder
      count={FOLDER_ROWS.length}
      expanded={expanded}
      folder={{ id: `batch-${layout}`, name: "Issue batch #1176–#1182" }}
      onDelete={() => undefined}
      onRename={() => undefined}
      onToggle={() => setExpanded(value => !value)}
      testIdPrefix="declutter"
    >
      {FOLDER_ROWS.map(session => (
        <Row key={session.id} layout={layout} session={session} />
      ))}
    </SessionListFolder>
  );
}

function SessionColumn({
  layout,
  background,
  title,
  threadSegment,
}: {
  layout: SessionRowLayout;
  background: IconBackground | "today";
  title: string;
  threadSegment: SessionThreadSegment;
}) {
  const catalog = agentCatalog(background);
  return (
    <AgentIdentityCatalogContext value={() => catalog}>
      <section className="flex flex-col gap-2">
        <Eyebrow className="px-1">{title}</Eyebrow>
        <aside
          aria-label={`Sessions, ${title}`}
          className="flex flex-col gap-1 rounded-lg bg-sunken px-2.5 py-2"
          style={{ width: SIDEBAR_WIDTH }}
        >
          <SessionListGroupToggle
            collapsed={false}
            count={14}
            identity={
              <WorkspaceBadge identity={{ kind: "project", name: "compozy" }} variant="chrome" />
            }
          />
          <div className="flex flex-col gap-0.5 pl-2.5">
            <StaysOnRow className="px-1 pt-1 pb-1.5" instances={COMPOZY_ROW} scopeName="compozy" />
            <Row layout={layout} session={SESSIONS.captain} />
            <Row layout={layout} session={SESSIONS.question} />
            <Row layout={layout} session={SESSIONS.chip} />
            <Thread initialSegment={threadSegment} layout={layout} />
            <Folder layout={layout} />
            <Row layout={layout} session={SESSIONS.board} />
            <Row layout={layout} session={SESSIONS.review} />
            <Row layout={layout} session={SESSIONS.docs} />
          </div>
        </aside>
      </section>
    </AgentIdentityCatalogContext>
  );
}

function SessionLists({
  layout,
  threadSegment,
}: {
  layout: SessionRowLayout;
  threadSegment: SessionThreadSegment;
}) {
  if (layout === "classic") {
    return (
      <SessionColumn
        background="today"
        layout={layout}
        threadSegment={threadSegment}
        title="Today (generated faces)"
      />
    );
  }
  return (
    <div className="flex flex-wrap items-start gap-8">
      <SessionColumn
        background="plate"
        layout={layout}
        threadSegment={threadSegment}
        title="Built-ins on a gray plate"
      />
      <SessionColumn
        background="none"
        layout={layout}
        threadSegment={threadSegment}
        title="Built-ins without background"
      />
    </div>
  );
}

function EditorPreview({ appearance }: { appearance: AgentAvatarAppearance }) {
  return (
    <div className="flex items-end gap-4 rounded-lg bg-sunken p-4">
      {[92, 32, 24, 16].map(size => (
        <AgentAvatar
          appearance={appearance}
          key={size}
          label="Code implementer"
          quietIdle
          size={size}
          spriteUrl={PROFILE_SPRITE_URL}
        />
      ))}
    </div>
  );
}

/** The symbol pane of the agent's appearance editor, with the new Background choice. */
function AppearanceEditor() {
  const theme = useCanvasTheme();
  const icons = useIconCatalog();
  const [appearance, setAppearance] = useState<AgentAvatarAppearance>(() =>
    builtinAppearance("code_implementer", "none")
  );
  const color = appearance.color ?? BUILTIN_GRAY;
  const symbol = agentSymbolOf(appearance);
  const background = agentSymbolBackgroundOf(appearance);
  const commit = (next: SymbolValue, nextColor: string, nextBackground: SymbolBackground) =>
    setAppearance(agentSymbolAppearance(next, nextColor, nextBackground));
  return (
    <div className="flex flex-wrap items-start gap-8">
      <div className="flex w-80 flex-col gap-3 rounded-xl bg-popover p-3 shadow-popover">
        <SymbolPicker
          background={background}
          color={color}
          emojibaseUrl={PROFILE_EMOJIBASE_URL}
          icons={icons}
          iconsLoading={icons.length === 0}
          onBackgroundChange={next => commit(symbol, color, next)}
          onColorChange={next => commit(symbol, next, background)}
          onSymbolChange={next => commit(next, color, background)}
          spriteUrl={PROFILE_SPRITE_URL}
          surface={identitySurfaceFor(theme)}
          swatches={PROFILE_IDENTITY_SWATCHES}
          symbol={symbol}
        />
      </div>
      <div className="flex flex-col gap-3">
        <Eyebrow>Preview</Eyebrow>
        <EditorPreview appearance={appearance} />
        <Eyebrow>Same icon, both backgrounds</Eyebrow>
        <EditorPreview appearance={agentSymbolAppearance(symbol, color, "plate")} />
        <EditorPreview appearance={agentSymbolAppearance(symbol, color, "none")} />
      </div>
    </div>
  );
}

export default function SessionListDeclutterDraft({ state }: PrototypeProps) {
  const layout = LAYOUTS[state];
  return (
    <main className="min-h-dvh bg-canvas p-8">
      {layout === undefined ? (
        <AppearanceEditor />
      ) : (
        <SessionLists
          layout={layout}
          threadSegment={state === "single-line-subagents" ? "subagents" : "children"}
        />
      )}
    </main>
  );
}
