import { AlertCircle, Plus } from "lucide-react";

import { Button, Spinner } from "@compozy/ui";

import {
  PROFILE_REMOTE_MANAGEMENT_LINE,
  PROFILE_SEPARATION_LINE,
  ProfileSelectionMap,
  ProfileSettingsList,
  useProfileFlowIntent,
  useProfilesSettingsPage,
  type ProfileFlowSearch,
} from "@/systems/profiles";
import { SettingsAdvancedFold, SettingsPageFrame, useSettingsTopbar } from "@/systems/settings";

const TEST_PREFIX = "settings-page-profiles";

/**
 * Settings → Profiles.
 *
 * The default read is the active list plus one honest sentence. Everything an
 * operator asks less often — the archived list, and where each profile is
 * currently active — sits one disclosure deeper.
 */
export interface ProfilesSettingsPageProps {
  /** Lifecycle flow a palette command navigated here to raise. */
  profileFlow?: ProfileFlowSearch;
}

export function ProfilesSettingsPage({ profileFlow }: ProfilesSettingsPageProps) {
  const page = useProfilesSettingsPage();
  useSettingsTopbar("profiles");
  useProfileFlowIntent(profileFlow);
  const lifecycleActions = {
    onEditIdentity: (name: string) => page.open({ flow: "update", profile: name }),
    onRename: (name: string) => page.open({ flow: "rename", profile: name }),
    onArchive: (name: string) => page.open({ flow: "archive", profile: name }),
    onUnarchive: (name: string) => page.open({ flow: "unarchive", profile: name }),
    onDelete: (name: string) => page.open({ flow: "delete", profile: name }),
  };

  if (page.isLoading) {
    return (
      <div
        className="flex flex-1 items-center justify-center py-12"
        role="status"
        data-testid={`${TEST_PREFIX}-loading`}
      >
        <Spinner className="size-4 text-subtle" />
      </div>
    );
  }

  if (page.errorMessage !== null) {
    return (
      <div
        className="flex flex-1 flex-col items-center justify-center gap-3 py-12"
        data-testid={`${TEST_PREFIX}-error`}
      >
        <AlertCircle aria-hidden="true" className="size-5 text-danger" />
        <p className="text-small-body text-muted">{page.errorMessage}</p>
        <Button size="sm" variant="outline" onClick={page.refetch}>
          Retry
        </Button>
      </div>
    );
  }

  return (
    <SettingsPageFrame slug="profiles">
      <div className="flex flex-col gap-4" data-testid={`${TEST_PREFIX}-content`}>
        <div className="flex items-center gap-2">
          <p className="flex-1 text-small-body text-subtle" data-testid={`${TEST_PREFIX}-line`}>
            {page.manageable ? PROFILE_SEPARATION_LINE : PROFILE_REMOTE_MANAGEMENT_LINE}
          </p>
          {page.manageable ? (
            <Button
              size="sm"
              onClick={() => page.open({ flow: "create" })}
              data-testid="profile-create-open"
            >
              <Plus aria-hidden="true" />
              Create profile
            </Button>
          ) : null}
        </div>
        <ProfileSettingsList
          profiles={page.active}
          currentName={page.currentName}
          manageable={page.manageable}
          variant="active"
          {...lifecycleActions}
        />
        {page.archived.length > 0 ? (
          <SettingsAdvancedFold
            data-testid={`${TEST_PREFIX}-archived`}
            label={`Archived (${page.archived.length})`}
            bare
          >
            <ProfileSettingsList
              profiles={page.archived}
              currentName={page.currentName}
              manageable={page.manageable}
              variant="archived"
              {...lifecycleActions}
            />
          </SettingsAdvancedFold>
        ) : null}
        <SettingsAdvancedFold
          data-testid={`${TEST_PREFIX}-selection-map`}
          label="Where each profile is active"
          padded
        >
          <ProfileSelectionMap
            selections={page.selections}
            profiles={[...page.active, ...page.archived]}
            projectName={page.projectName}
          />
        </SettingsAdvancedFold>
      </div>
    </SettingsPageFrame>
  );
}
