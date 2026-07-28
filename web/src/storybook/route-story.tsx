import { useEffect } from "react";

import type { SettingsSectionName } from "@/systems/settings";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import { useActiveWorkspaceStore } from "@/systems/workspace/hooks/use-active-workspace-store";
import { useUserHomeDirStore } from "@/systems/workspace/hooks/use-user-home-dir-store";
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
    useActiveWorkspaceStore.getState().setSelectedWorkspaceId(workspaceId);
  }, [workspaceId]);

  return null;
}

export function StorybookUserHomeDirSetup({ userHomeDir }: { userHomeDir: string | null }) {
  useEffect(() => {
    useUserHomeDirStore.getState().setUserHomeDir(userHomeDir);
  }, [userHomeDir]);

  return null;
}

export function StorybookRestartNoticeSetup({ section }: { section: SettingsSectionName }) {
  useEffect(() => {
    useSettingsRestartStore.getState().recordMutation({
      section,
      restartRequired: true,
      restartScope: "global",
      warnings: [],
      completedAt: "2026-04-18T01:00:00Z",
    });
  }, [section]);

  return null;
}
