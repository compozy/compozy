import { fn } from "storybook/test";

import { projectSessionGroups } from "../../lib/session-list-groups";
import { buildProjectGroupTree } from "../../lib/workspace-folders";
import { workspaceFolderFixture } from "../../mocks/workspace-layout-handlers";

/** All-projects fixtures for the ProjectFolders sidebar story (#1044). */
const PROJECT_FACET = (all: number, working = 0, needsYou = 0) => ({
  all,
  working,
  needs_you: needsYou,
  idle: all - working - needsYou,
  finished: 0,
  terminal_approvals: 0,
  board: { needs_you: needsYou, working, in_review: 0, done: 0 },
});
export const PROJECT_GROUPS = projectSessionGroups(
  {
    by_workspace: [
      { workspace_id: "ws-api", facets: PROJECT_FACET(4, 2) },
      { workspace_id: "ws-web", facets: PROJECT_FACET(3, 1, 1) },
      { workspace_id: "ws-docs", facets: PROJECT_FACET(2) },
      { workspace_id: "ws-site", facets: PROJECT_FACET(1) },
      { workspace_id: "", facets: PROJECT_FACET(2) },
    ],
  },
  {
    workspaces: [
      { id: "ws-api", name: "compozy-api" },
      { id: "ws-web", name: "compozy-web" },
      { id: "ws-docs", name: "docs" },
      { id: "ws-site", name: "marketing-site" },
    ],
    lens: { profile: "default" },
  }
);
export const PROJECT_LAYOUT = {
  folders: [
    { ...workspaceFolderFixture("wfold-product", "Product"), position: 0 },
    { ...workspaceFolderFixture("wfold-content", "Content"), position: 1 },
  ],
  placements: [
    { workspace_id: "ws-api", folder_id: "wfold-product", position: 0 },
    { workspace_id: "ws-web", folder_id: "wfold-product", position: 1 },
    { workspace_id: "ws-docs", folder_id: "wfold-content", position: 0 },
  ],
  createFolder: fn(async () => workspaceFolderFixture("wfold-new", "New")),
  renameFolder: fn(async () => undefined),
  deleteFolder: fn(async () => undefined),
  moveFolder: fn(),
  reorderFolders: fn(),
  moveWorkspace: fn(),
  reorderWorkspaces: fn(),
};
export const PROJECT_TREE = buildProjectGroupTree(PROJECT_GROUPS, PROJECT_LAYOUT);
