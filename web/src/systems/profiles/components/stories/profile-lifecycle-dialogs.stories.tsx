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
import { ProfileCreateDialog } from "../profile-create-dialog";
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

const catalog = {
  icons: [
    { name: "rocket", label: "rocket", keywords: "launch" },
    { name: "megaphone", label: "megaphone", keywords: "announce" },
  ],
  loading: false,
} as const;

/** Create refusal stays attached to the name field while the real identity picker remains usable. */
export const CreateInvalidName: Story = {
  args: {} as never,
  render: () => (
    <ProfileCreateDialog
      catalog={catalog}
      open
      onOpenChange={fn()}
      existingCount={4}
      lens={{ scope: "global" }}
      isPending={false}
      initialName="all"
      nameError="Profile name is reserved."
      onCreate={fn()}
    />
  ),
};

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

/** A daemon name collision is shown inline against the editable field. */
export const RenameNameTaken: Story = {
  args: {} as never,
  render: () => (
    <ProfileRenameDialog
      open
      onOpenChange={fn()}
      profile="marketing"
      newName="consulting"
      onNewNameChange={fn()}
      plan={renamePlanFixture}
      planLoading={false}
      acceptedRepos={[]}
      onToggleRepo={fn()}
      isPending={false}
      error="A profile named consulting already exists."
      onRename={fn()}
    />
  ),
};

/** The permanent default identity has no editable name control and explains why. */
export const RenameDefault: Story = {
  args: {} as never,
  render: () => (
    <ProfileRenameDialog
      open
      onOpenChange={fn()}
      profile="default"
      newName=""
      onNewNameChange={fn()}
      plan={undefined}
      planLoading={false}
      acceptedRepos={[]}
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
      catalog={catalog}
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
      planLoading={false}
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
      planLoading={false}
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
      pausedAutomations={["job:client-report-weekly", "trigger:invoice-reminder"]}
      isPending={false}
      onUnarchive={fn()}
      onSetAutomationEnabled={fn().mockResolvedValue(undefined)}
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
      planLoading={false}
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
      planLoading={false}
      workItems={4}
      isPending={false}
      onDelete={fn()}
      onArchiveInstead={fn()}
    />
  ),
};
