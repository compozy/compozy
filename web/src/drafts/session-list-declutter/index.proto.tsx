/**
 * Issue #1191: the quiet session list. Line 2 writes the model, or only the needs-you words in its
 * place; the agent's name rides on its face's resting-hover tooltip. Built-in agents wear gray
 * icons without a plate.
 */
import { useId, useState, type ReactNode } from "react";

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
import { SessionListRowView } from "@/systems/session/components/session-list/session-list-row-view";
import { SessionRowActions } from "@/systems/session/components/session-row-actions";
import { childSessionSignalState } from "@/systems/session/lib/session-hierarchy";
import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "@/systems/session/lib/session-sidebar-width";
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
  THREAD_CHILDREN,
  agentCatalog,
  builtinAppearance,
} from "@/systems/session/mocks/session-list-declutter-fixtures";

export const meta = definePrototype({
  title: "Session list declutter (#1191)",
  route: "/sessions",
  states: [
    { id: "children", label: "Child sessions open" },
    { id: "subagents", label: "Subagents open" },
    { id: "editor", label: "Appearance editor: no background" },
  ],
});

const actions = sessionLifecycleActionsFixture();

/** The orchestrator's direct subagents: one waiting on you, one failed, one working, one done. */
const SUBAGENTS = [roster[2]!, roster[0]!, roster[3]!, roster[5]!];

function Row({ session, branch }: { session: SessionPayload; branch?: ReactNode }) {
  return (
    <SessionListRowView
      onSelect={() => undefined}
      preloadHandlers={SESSION_ENTRY_PRELOAD_FIXTURE}
      rowActions={<SessionRowActions actions={actions} session={session} />}
      runtime={boardRuntimeFixture(session)}
      session={session}
      sessionActions={actions}
      testIdPrefix="declutter"
      trailing={branch}
    />
  );
}

function Thread({ initialSegment }: { initialSegment: SessionThreadSegment }) {
  const [segment, setSegment] = useState<SessionThreadSegment | null>(initialSegment);
  const root = SESSIONS.orchestrator;
  const branchId = useId();
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
                      <Row session={child} />
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

function Folder() {
  const [expanded, setExpanded] = useState(true);
  return (
    <SessionListFolder
      count={FOLDER_ROWS.length}
      expanded={expanded}
      folder={{ id: "batch", name: "Issue batch #1176–#1182" }}
      onDelete={() => undefined}
      onRename={() => undefined}
      onToggle={() => setExpanded(value => !value)}
      testIdPrefix="declutter"
    >
      {FOLDER_ROWS.map(session => (
        <Row key={session.id} session={session} />
      ))}
    </SessionListFolder>
  );
}

function SessionColumn({ threadSegment }: { threadSegment: SessionThreadSegment }) {
  const catalog = agentCatalog();
  return (
    <AgentIdentityCatalogContext value={() => catalog}>
      <section className="flex flex-col gap-2">
        <aside
          aria-label="Sessions"
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
            <Row session={SESSIONS.captain} />
            <Row session={SESSIONS.question} />
            <Row session={SESSIONS.chip} />
            <Thread initialSegment={threadSegment} key={threadSegment} />
            <Folder />
            <Row session={SESSIONS.board} />
            <Row session={SESSIONS.review} />
            <Row session={SESSIONS.docs} />
          </div>
        </aside>
      </section>
    </AgentIdentityCatalogContext>
  );
}

function SessionLists({ threadSegment }: { threadSegment: SessionThreadSegment }) {
  return <SessionColumn threadSegment={threadSegment} />;
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
  return (
    <main className="min-h-dvh bg-canvas p-8">
      {state === "editor" ? (
        <AppearanceEditor />
      ) : (
        <SessionLists threadSegment={state === "subagents" ? "subagents" : "children"} />
      )}
    </main>
  );
}
