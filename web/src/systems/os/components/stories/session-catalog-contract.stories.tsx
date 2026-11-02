import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { fn, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { CenteredSurface } from "@/storybook/story-layout";
import {
  SESSION_LIST_MANUAL_SORT_NOTE,
  type SessionLifecycleActionHandlers,
  type SessionListGroupSpec,
  type SessionListViewModel,
  type SessionPayload,
} from "@/systems/session";
import { SessionList } from "@/systems/session/components/session-list/session-list";
import { sessionListCatalogFixture, sessionListViewFixture } from "@/systems/session/mocks";
import { ownerFromRow } from "@/systems/profiles";
import { USER_SCOPE_SELECTOR, ownerDataScope, workspaceBadgeIdentity } from "@/systems/workspace";

type SessionOwner = Pick<
  SessionPayload,
  "profile_id" | "profile_name" | "profile_color" | "profile_icon" | "profile_archived"
>;

const DEFAULT_OWNER: SessionOwner = {
  profile_id: "00000000000000000000000000",
  profile_name: "default",
  profile_color: "#8a8f98",
  profile_icon: "user-round",
  profile_archived: false,
};

function session(
  id: string,
  title: string,
  workspaceId: string,
  owner: SessionOwner = DEFAULT_OWNER
): SessionPayload {
  return {
    board_column: "done",
    pull_request: null,
    pull_request_lookup: null,
    supervision: null,
    ...owner,
    scope: "workspace",
    effective_workspace_access: "",
    id,
    name: title,
    agent_name: "codex",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: workspaceId,
    workspace_path: `/workspace/${workspaceId}`,
    state: "active",
    badge: "running",
    attachable: true,
    archived_at: null,
    available_commands: [],
    pending_interactions: [],
    created_at: "2026-08-16T12:00:00Z",
    updated_at: "2026-08-16T12:10:00Z",
  };
}

const ACTIONS: SessionLifecycleActionHandlers = {
  pendingAction: null,
  pendingSessionId: null,
  onArchive: fn(),
  onDelete: fn(),
  onRename: fn(),
  onStop: fn(),
  onUnarchive: fn(),
};

type CatalogState =
  | "sort"
  | "all-projects"
  | "grouped-projects"
  | "group-failure"
  | "grouped-profiles"
  | "manual-note"
  | "list-failure"
  | "aggregate"
  | "empty-scoped";

const MARKETING: SessionOwner = {
  profile_id: "01J9MARKETING00000000000000",
  profile_name: "marketing",
  profile_color: "#c26ad6",
  profile_icon: "megaphone",
  profile_archived: false,
};

const WORKSPACE_NAMES = new Map([
  ["workspace-compozy", "compozy"],
  ["workspace-infra", "infra"],
]);

/** Every session the story daemon serves, across projects, User, and profiles. */
const STORY_SESSIONS = [
  session("session-runtime", "Runtime parity pass", "workspace-compozy"),
  session("session-campaign", "Launch campaign", "workspace-compozy", MARKETING),
  session("session-release", "Release gate", "workspace-infra"),
  session("session-notes", "Weekly notes", ""),
];

const badgeOf = (row: SessionPayload) => workspaceBadgeIdentity(row.workspace_id, WORKSPACE_NAMES);

function projectGroup(
  workspaceId: string,
  counts: Pick<SessionListGroupSpec, "total" | "working" | "needsYou">
): SessionListGroupSpec {
  const scope = ownerDataScope(workspaceId);
  return {
    key: scope.kind === "user" ? "user" : `project:${workspaceId}`,
    kind: "project",
    badge: workspaceBadgeIdentity(workspaceId, WORKSPACE_NAMES),
    ...counts,
    scope,
    lens: { profile: "default" },
    pair: { workspaceId: workspaceId || USER_SCOPE_SELECTOR, profile: "default" },
  };
}

function profileGroup(owner: SessionOwner, total: number): SessionListGroupSpec {
  return {
    key: `profile:${owner.profile_id}`,
    kind: "profile",
    owner: ownerFromRow(owner),
    total,
    working: total,
    needsYou: 0,
    scope: { kind: "all" },
    lens: { profile: owner.profile_name },
    pair: null,
  };
}

const PROJECT_GROUPS = [
  projectGroup("workspace-compozy", { total: 1, working: 1, needsYou: 0 }),
  projectGroup("workspace-infra", { total: 4, working: 1, needsYou: 2 }),
  projectGroup("", { total: 1, working: 1, needsYou: 0 }),
];

/** Each group body reads its own page; `failWorkspace` makes one group unreachable. */
function catalogHandler(failWorkspace?: string) {
  return compozyApiMock.get("/api/sessions", ({ request }) => {
    const url = new URL(request.url);
    const selector = url.searchParams.get("workspace_id");
    if (failWorkspace && selector === failWorkspace) {
      return HttpResponse.json({ error: "unreachable" }, { status: 503 });
    }
    const profile = url.searchParams.get("profile");
    const sessions = STORY_SESSIONS.filter(row => {
      if (selector && row.workspace_id !== (selector === "@user" ? "" : selector)) return false;
      return !profile || row.profile_name === profile;
    });
    return HttpResponse.json({
      sessions,
      page: { has_more: false, limit: 100, total: sessions.length },
    });
  });
}

function storyView(state: CatalogState): SessionListViewModel {
  const allProjects = state !== "sort" && state !== "empty-scoped" && state !== "manual-note";
  const view = sessionListViewFixture({
    sort: state === "manual-note" ? "manual" : "attention",
    groupByOptions: allProjects ? ["none", "project"] : ["none"],
    scopeLabel: state === "empty-scoped" ? "Marketing" : "default",
    ownerOf: ownerFromRow,
    badgeOf: allProjects ? badgeOf : undefined,
    catalog: sessionListCatalogFixture({ failed: state === "list-failure" }),
  });
  switch (state) {
    case "sort":
      return { ...view, groupByOptions: ["none", "project", "profile"] };
    case "grouped-projects":
    case "group-failure":
      return {
        ...view,
        catalog: undefined,
        storedGroupBy: "project",
        groupBy: "project",
        groups: PROJECT_GROUPS,
        collapsedGroupKeys:
          state === "grouped-projects" ? new Set(["project:workspace-infra"]) : new Set(),
      };
    case "grouped-profiles":
      return {
        ...view,
        catalog: undefined,
        aggregate: true,
        scopeLabel: null,
        storedGroupBy: "profile",
        groupBy: "profile",
        groupByOptions: ["none", "project", "profile"],
        groups: [profileGroup(DEFAULT_OWNER, 3), profileGroup(MARKETING, 1)],
      };
    case "manual-note":
      return { ...view, sortNote: SESSION_LIST_MANUAL_SORT_NOTE };
    case "aggregate":
      return { ...view, aggregate: true, scopeLabel: null };
    default:
      return view;
  }
}

function storySessions(state: CatalogState): SessionPayload[] {
  switch (state) {
    case "all-projects":
      return STORY_SESSIONS.filter(row => row.profile_name === "default");
    case "aggregate":
      return [
        ...STORY_SESSIONS,
        session("session-archive", "Agency handoff", "workspace-compozy", {
          profile_id: "01J9OLDAGENCY0000000000000",
          profile_name: "old-agency",
          profile_color: "#b58e5f",
          profile_icon: "folder",
          profile_archived: true,
        }),
      ];
    case "sort":
    case "manual-note":
      return STORY_SESSIONS.filter(row => row.workspace_id === "workspace-compozy").slice(0, 1);
    default:
      return [];
  }
}

function SessionCatalogContract({ state }: { state: CatalogState }) {
  return (
    <div className="flex h-160 w-120 overflow-hidden rounded-lg border border-line bg-canvas-soft">
      <SessionList
        sessions={storySessions(state)}
        disconnected={false}
        collapsedThreadIds={[]}
        subagentThreadIds={[]}
        view={storyView(state)}
        onThreadBranchChange={fn()}
        onSelectSession={fn()}
        onNewSession={fn()}
        sessionActions={ACTIONS}
        testIdPrefix="contract-catalog"
      />
    </div>
  );
}

const meta: Meta<typeof SessionCatalogContract> = {
  title: "systems/os/components/SessionCatalogContract",
  component: SessionCatalogContract,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** One View options menu: sort, grouping where one applies, and the folds (#1132). */
export const ViewOptionsMenu: Story = {
  args: { state: "sort" },
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement.ownerDocument.body).getByTestId("contract-catalog-view-options-trigger")
    );
  },
};

/** All projects, flat: one query, every row names its project before agent and provider. */
export const AllProjectsFlat: Story = {
  args: { state: "all-projects" },
};

/** Grouped by project from facets: infra is collapsed and still shows its needs-you dot. */
export const GroupedByProject: Story = {
  args: { state: "grouped-projects" },
  parameters: storybookMswParameters({ session: [catalogHandler()] }),
};

/** One unreachable group retries in place while the others render. */
export const GroupFailure: Story = {
  args: { state: "group-failure" },
  parameters: storybookMswParameters({ session: [catalogHandler("workspace-infra")] }),
};

/** Grouped by profile under All profiles: rows omit the profile tag the header names. */
export const GroupedByProfile: Story = {
  args: { state: "grouped-profiles" },
  parameters: storybookMswParameters({ session: [catalogHandler()] }),
};

/** Stored manual order outside one (project, profile) pair: served as last activity. */
export const ManualSortNote: Story = {
  args: { state: "manual-note" },
};

/** A flat list that never loaded states one list-level error with Retry. */
export const ListFailure: Story = {
  args: { state: "list-failure" },
};

/** All profiles is a read lens: mixed-owner rows stay labeled and archived ownership is muted. */
export const AggregateProfiles: Story = {
  args: { state: "aggregate" },
};

/** A scoped empty catalog says which active profile has no sessions. */
export const ScopedProfileEmpty: Story = {
  args: { state: "empty-scoped" },
};
