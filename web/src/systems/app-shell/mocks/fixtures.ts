import type { Workspace } from "../types";

export const workspaceFixture: Workspace = {
  id: "ws-storybook",
  name: "storybook-workspace",
  root_dir: "/workspaces/storybook",
  created_at: "2026-04-20T00:00:00Z",
  updated_at: "2026-04-21T00:00:00Z",
};

export const secondaryWorkspaceFixture: Workspace = {
  id: "ws-storybook-2",
  name: "storybook-archive",
  root_dir: "/workspaces/storybook-archive",
  created_at: "2026-04-19T00:00:00Z",
  updated_at: "2026-04-20T00:00:00Z",
};

export function buildWorkspaceFixture(overrides: Partial<Workspace> = {}): Workspace {
  return {
    ...workspaceFixture,
    ...overrides,
  };
}

export function buildWorkspacesFixture(workspaces: Workspace[] = [workspaceFixture]): Workspace[] {
  return workspaces;
}
