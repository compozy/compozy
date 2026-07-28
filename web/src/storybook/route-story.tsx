import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { settingsRestartStore, type SettingsSectionName } from "@/systems/settings";
import { statusKeys } from "@/systems/status";
import { statusFixture } from "@/systems/status/mocks";
import { setActiveWorkspaceId } from "@/systems/workspace";
import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";

export function StorybookRouteCanvas() {
  return null;
}

export function StorybookWorkspaceSetup({
  workspaceId = storyDefaultWorkspaceId,
}: {
  workspaceId?: string;
}) {
  useEffect(() => {
    setActiveWorkspaceId(workspaceId);
  }, [workspaceId]);

  return null;
}

export function StorybookUserHomeDirSetup({ userHomeDir }: { userHomeDir: string | null }) {
  const queryClient = useQueryClient();

  useEffect(() => {
    queryClient.setQueryData(statusKeys.current(), {
      ...statusFixture,
      daemon: {
        ...statusFixture.daemon,
        user_home_dir: userHomeDir ?? "",
      },
    });
  }, [queryClient, userHomeDir]);

  return null;
}

export function StorybookRestartNoticeSetup({ section }: { section: SettingsSectionName }) {
  useEffect(() => {
    settingsRestartStore.trigger.settingsMutationRecorded({
      mutation: {
        section,
        restartRequired: true,
        restartScope: "global",
        warnings: [],
        completedAt: "2026-04-18T01:00:00Z",
      },
    });
  }, [section]);

  return null;
}
