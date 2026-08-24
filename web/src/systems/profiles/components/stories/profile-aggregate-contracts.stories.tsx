import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, within } from "storybook/test";

import { Toaster } from "@compozy/ui";

import { notifyUser } from "@/lib/user-feedback";
import { GlobalScopeToggle } from "@/systems/os/components/global-scope-toggle";
import type {
  SessionLifecycleActionHandlers,
  SessionListViewModel,
  SessionPayload,
} from "@/systems/session";
import { SessionList } from "@/systems/session/components/session-list/session-list";

import { createdInProfileToast } from "../../lib/profile-copy";
import { ownerFromRow } from "../../lib/profile-scope";
import { toProfileRows } from "../../lib/profile-rows";
import {
  consultingProfileFixture,
  defaultProfileFixture,
  marketingProfileFixture,
} from "../../mocks/fixtures";
import { ProfileOwnerBanner } from "../profile-owner-banner";
import { ProfileSwitcher } from "../profile-switcher";

const ACTIONS: SessionLifecycleActionHandlers = {
  pendingAction: null,
  pendingSessionId: null,
  onArchive: fn(),
  onDelete: fn(),
  onRename: fn(),
  onStop: fn(),
  onUnarchive: fn(),
};

function session(
  id: string,
  name: string,
  owner: Pick<
    SessionPayload,
    | "profile_id"
    | "profile_name"
    | "profile_color"
    | "profile_icon"
    | "profile_emoji"
    | "profile_archived"
  >
): SessionPayload {
  return {
    ...owner,
    id,
    name,
    agent_name: "codex",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: "workspace-compozy",
    workspace_path: "/workspace/compozy",
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

const DEFAULT_SESSION = session("session-default", "Runtime parity pass", {
  profile_id: defaultProfileFixture.id,
  profile_name: defaultProfileFixture.name,
  profile_color: defaultProfileFixture.color,
  profile_icon: defaultProfileFixture.icon ?? undefined,
  profile_emoji: defaultProfileFixture.emoji ?? undefined,
  profile_archived: false,
});

const MARKETING_SESSION = session("session-marketing", "Launch campaign", {
  profile_id: marketingProfileFixture.id,
  profile_name: marketingProfileFixture.name,
  profile_color: marketingProfileFixture.color,
  profile_icon: marketingProfileFixture.icon ?? undefined,
  profile_emoji: marketingProfileFixture.emoji ?? undefined,
  profile_archived: false,
});

function ProfileScopeComposition({ aggregate }: { aggregate: boolean }) {
  const sessions = aggregate ? [DEFAULT_SESSION, MARKETING_SESSION] : [MARKETING_SESSION];
  const view: SessionListViewModel = {
    scope: "workspace",
    sort: "last_activity",
    archived: false,
    saving: false,
    setScope: fn(),
    setSort: fn(),
    setArchived: fn(),
    aggregate,
    scopeLabel: aggregate ? null : "marketing",
    ownerOf: ownerFromRow,
    workspaceGroups: [],
    collapsedWorkspaceIds: new Set(),
    toggleWorkspace: fn(),
  };
  const rows = toProfileRows(
    [defaultProfileFixture, marketingProfileFixture, consultingProfileFixture],
    "marketing"
  );
  return (
    <div className="flex h-screen w-full flex-col bg-canvas text-fg">
      <header className="flex h-11 items-center justify-end gap-2 border-b border-line px-4">
        <GlobalScopeToggle
          checked
          locked={false}
          onCheckedChange={fn()}
          tooltip="Back to compozy"
        />
        <ProfileSwitcher
          activeName="marketing"
          aggregate={aggregate}
          archivedCount={1}
          onCreate={fn()}
          onOpenSettings={fn()}
          onSelectAggregate={fn()}
          onSelectProfile={fn()}
          quiet={false}
          rows={rows}
        />
      </header>
      <main className="flex min-h-0 flex-1 items-start justify-center px-6 pt-12">
        <div className="h-150 w-120 overflow-hidden rounded-lg border border-line bg-canvas-soft">
          <SessionList
            sessions={sessions}
            disconnected={false}
            collapsedThreadIds={[]}
            view={view}
            onToggleThread={fn()}
            onSelectSession={fn()}
            onNewSession={fn()}
            sessionActions={ACTIONS}
            testIdPrefix="profile-scope-contract"
          />
        </div>
      </main>
    </div>
  );
}

const meta: Meta<typeof ProfileOwnerBanner> = {
  title: "systems/profiles/components/ProfileAggregateContracts",
  component: ProfileOwnerBanner,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** A deep-linked session remains visible and offers one switch to its owner. */
export const ForeignSessionOwner: Story = {
  args: {} as never,
  render: () => (
    <div className="flex h-screen items-start justify-center bg-canvas px-8 pt-28">
      <div className="w-150">
        <ProfileOwnerBanner
          noun="session"
          owner={{
            id: marketingProfileFixture.id,
            name: marketingProfileFixture.name,
            color: marketingProfileFixture.color,
            icon: marketingProfileFixture.icon,
            emoji: marketingProfileFixture.emoji,
            archived: false,
          }}
          onSwitch={fn()}
        />
      </div>
    </div>
  ),
};

/** Creation under All profiles reports the actual owner after the mutation settles. */
export const CreatedInDefaultToast: Story = {
  args: {} as never,
  render: () => (
    <div className="h-screen bg-canvas">
      <Toaster duration={60_000} position="top-right" />
    </div>
  ),
  play: async ({ canvasElement }) => {
    notifyUser({ message: createdInProfileToast("default"), tone: "success" });
    await within(canvasElement.ownerDocument.body).findByText("Created in default.");
  },
};

/** Global scope is on while the work catalog remains bounded to marketing. */
export const GlobeWithScopedProfile: Story = {
  args: {} as never,
  render: () => <ProfileScopeComposition aggregate={false} />,
};

/** Global scope and All profiles compose independently; owner labels remain on mixed rows. */
export const GlobeWithAllProfiles: Story = {
  args: {} as never,
  render: () => <ProfileScopeComposition aggregate />,
};
