import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import {
  archiveBlockedPlanFixture,
  archivePlanFixture,
  marketingProfileFixture,
  deletePlanFixture,
  renamePlanFixture,
} from "../../mocks/fixtures";
import { ProfileArchiveDialog } from "../profile-archive-dialog";
import { ProfileDeleteDialog } from "../profile-delete-dialog";
import { ProfileIdentityDialog } from "../profile-identity-dialog";
import { ProfileRenameDialog } from "../profile-rename-dialog";
import { ProfileUnarchiveDialog } from "../profile-unarchive-dialog";

const meta: Meta<typeof ProfileArchiveDialog> = {
  title: "systems/profiles/components/ProfileLifecycleDialogs",
  component: ProfileArchiveDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Every dialog renders exactly what its plan endpoint returned. Archive is calm because nothing is deleted; delete inks danger on the confirm button alone, and routes to archive when the profile still holds work.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Rename: machine folders move automatically, repo folders are offers. */
export const Rename: Story = {
  args: {} as never,
  render: () => (
    <ProfileRenameDialog
      open
      onOpenChange={fn()}
      profile="marketing"
      newName="brand-studio"
      onNewNameChange={fn()}
      plan={renamePlanFixture}
      planLoading={false}
      acceptedRepos={["ws-acme"]}
      onToggleRepo={fn()}
      isPending={false}
      onRename={fn()}
    />
  ),
};

export const Identity: Story = {
  args: {} as never,
  render: () => (
    <ProfileIdentityDialog
      open
      onOpenChange={fn()}
      profile={marketingProfileFixture}
      isPending={false}
      onSave={fn()}
    />
  ),
};

/** Archive lists what pauses. Nothing is destroyed, so nothing reads as danger. */
export const Archive: Story = {
  args: {} as never,
  render: () => (
    <ProfileArchiveDialog
      open
      onOpenChange={fn()}
      profile="marketing"
      plan={archivePlanFixture}
      isPending={false}
      onArchive={fn()}
    />
  ),
};

/** Blocked by running work: a warning that names the sessions, not an error. */
export const ArchiveBlocked: Story = {
  args: {} as never,
  render: () => (
    <ProfileArchiveDialog
      open
      onOpenChange={fn()}
      profile="marketing"
      plan={archiveBlockedPlanFixture}
      isPending={false}
      onArchive={fn()}
    />
  ),
};

/** Unarchive settled: each paused automation stays off until turned on. */
export const UnarchiveReactivation: Story = {
  args: {} as never,
  render: () => (
    <ProfileUnarchiveDialog
      open
      onOpenChange={fn()}
      profile="old-agency"
      pausedAutomations={["client-report-weekly", "invoice-reminder"]}
      isPending={false}
      onUnarchive={fn()}
      onDone={fn()}
    />
  ),
};

/** Delete enumerates exactly what goes; danger inks the confirm and nothing else. */
export const Delete: Story = {
  args: {} as never,
  render: () => (
    <ProfileDeleteDialog
      open
      onOpenChange={fn()}
      profile="scratch"
      plan={deletePlanFixture}
      workItems={0}
      isPending={false}
      onDelete={fn()}
      onArchiveInstead={fn()}
    />
  ),
};

/** Work exists, so the road bends to archive rather than to a scarier confirm. */
export const DeleteRoutesToArchive: Story = {
  args: {} as never,
  render: () => (
    <ProfileDeleteDialog
      open
      onOpenChange={fn()}
      profile="old-agency"
      plan={undefined}
      workItems={4}
      isPending={false}
      onDelete={fn()}
      onArchiveInstead={fn()}
    />
  ),
};
