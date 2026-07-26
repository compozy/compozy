import { describe, expectTypeOf, it } from "vitest";

import type {
  WorkspaceDetailPayload,
  WorkspacePayload,
  WorkspaceResponse,
  WorkspacesResponse,
} from "../types";

describe("workspace contract types", () => {
  it("derives workspace payloads from the generated contract", () => {
    expectTypeOf<WorkspacePayload>().toMatchTypeOf<{
      id: string;
      root_dir: string;
      add_dirs: string[];
      name: string;
      created_at: string;
      updated_at: string;
      default_agent?: string;
    }>();

    expectTypeOf<WorkspacesResponse>().toMatchTypeOf<{ workspaces: WorkspacePayload[] }>();
    expectTypeOf<WorkspaceResponse>().toMatchTypeOf<{ workspace: WorkspacePayload }>();
    expectTypeOf<WorkspaceDetailPayload>().toMatchTypeOf<{
      workspace: WorkspacePayload;
      sessions?: unknown[];
      agents?: unknown[];
      skills?: unknown[];
    }>();
  });
});
