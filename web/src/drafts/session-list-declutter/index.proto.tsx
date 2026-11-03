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
import {
  SessionListRowView,
  type SessionRowLayout,
} from "@/systems/session/components/session-list/session-list-row-view";
import { SessionRowActions } from "@/systems/session/components/session-row-actions";
import { childSessionSignalState } from "@/systems/session/lib/session-hierarchy";
import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "@/systems/session/lib/session-sidebar-width";
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
    { id: "face", label: "C · Face first" },
    { id: "editor", label: "Appearance editor: no background" },
  ],
});

const actions = sessionLifecycleActionsFixture();

const CAPTAIN_FACE: AgentAvatarAppearance = { avatar: "generated", seed: "release-captain.2" };

const LAYOUTS: Record<string, SessionRowLayout> = {
  before: "classic",
  trimmed: "trimmed",
  "single-line": "single-line",
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

function Thread({ layout }: { layout: SessionRowLayout }) {
  const [open, setOpen] = useState(true);
  const root = SESSIONS.orchestrator;
  const branchId = `declutter-branch-${layout}`;
  return (
    <div>
      <Row
        branch={
          <SessionListBranchToggle
            branchId={branchId}
            childCount={THREAD_CHILDREN.length}
            childSignal={open ? null : childSessionSignalState(THREAD_CHILDREN)}
            onChange={next => setOpen(next !== null)}
            segment={open ? "children" : null}
            session={root}
            subagentCounts={null}
            testIdPrefix="declutter"
          />
        }
        layout={layout}
        session={root}
      />
      <div id={branchId}>
        <SessionListBranch
          render={() => (
            <div className={BRANCH_RAIL}>
              {THREAD_CHILDREN.map(child => (
                <div className={BRANCH_NODE} key={child.id}>
                  <Row layout={layout} session={child} />
                </div>
              ))}
            </div>
          )}
          segment={open ? "children" : null}
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
}: {
  layout: SessionRowLayout;
  background: IconBackground | "today";
  title: string;
}) {
  const catalog = agentCatalog(background);
  return (
    <AgentIdentityCatalogContext value={() => catalog}>
      <section className="flex flex-col gap-2">
        <Eyebrow className="px-1">{title}</Eyebrow>
        <aside
          aria-label={`Sessions, ${title}`}
          className="flex flex-col gap-1 rounded-lg bg-sunken px-2.5 py-2"
          style={{ width: SESSION_SIDEBAR_DEFAULT_WIDTH }}
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
            <Thread layout={layout} />
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

function SessionLists({ layout }: { layout: SessionRowLayout }) {
  if (layout === "classic") {
    return <SessionColumn background="today" layout={layout} title="Today (generated faces)" />;
  }
  return (
    <div className="flex flex-wrap items-start gap-8">
      <SessionColumn background="plate" layout={layout} title="Built-ins on a gray plate" />
      <SessionColumn background="none" layout={layout} title="Built-ins without background" />
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
      {layout === undefined ? <AppearanceEditor /> : <SessionLists layout={layout} />}
    </main>
  );
}
