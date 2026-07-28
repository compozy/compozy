import { render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import {
  multiHunkEditToolMessageFixture,
  sessionFixtures,
  uiMessageFixtures,
} from "@/systems/session/mocks";
import {
  resetSettingsRestartStore,
  useSettingsRestartStore,
} from "@/systems/settings/stores/use-settings-restart-store";
import { workspaceDetailFixture, workspaceFixtures } from "@/systems/workspace/mocks";
import { StorybookRestartPhaseSetup } from "@/storybook/settings-state-helpers";

afterEach(() => {
  resetSettingsRestartStore();
});

describe("storybook story and fixture regressions", () => {
  it("keeps UI message fixture ids unique and workspace paths neutral", () => {
    const ids = uiMessageFixtures.map(message => message.id);
    const sessionPaths = sessionFixtures
      .map(session => session.workspace_path)
      .filter((value): value is string => typeof value === "string");
    const skillDirs =
      workspaceDetailFixture.skills?.flatMap(skill =>
        typeof skill === "object" &&
        skill !== null &&
        "dir" in skill &&
        typeof skill.dir === "string"
          ? [skill.dir]
          : []
      ) ?? [];
    const workspacePaths = [
      ...workspaceFixtures.flatMap(workspace => [workspace.root_dir, ...workspace.add_dirs]),
      workspaceDetailFixture.workspace.root_dir,
      ...workspaceDetailFixture.workspace.add_dirs,
      ...skillDirs,
    ];

    expect(new Set(ids).size).toBe(ids.length);

    for (const path of [...workspacePaths, ...sessionPaths]) {
      expect(path).not.toMatch(/^\/Users\//);
      expect(path).not.toContain("/pedro/");
    }
  });

  it("keeps the multi-hunk edit fixture truthful", () => {
    const oldString = String(multiHunkEditToolMessageFixture.toolInput?.old_string ?? "");
    const newString = String(multiHunkEditToolMessageFixture.toolInput?.new_string ?? "");

    expect(oldString.trim()).not.toBe("");
    expect(newString.trim()).not.toBe("");
    expect(oldString).not.toEqual(newString);
  });

  it("keeps restart phase seeding stable across equivalent rerenders", () => {
    const restartPhase = () => (
      <StorybookRestartPhaseSetup
        section="general"
        overrides={{
          mutationRestartRequired: true,
          operationId: "op_success",
          status: "ready",
        }}
      />
    );
    const view = render(restartPhase());

    expect(useSettingsRestartStore.getState().mutationGeneration).toBe(1);

    view.rerender(restartPhase());

    expect(useSettingsRestartStore.getState().mutationGeneration).toBe(1);
  });
});
