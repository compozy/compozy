import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { createTestQueryClient, installFetchStub, matchPath, withQuery } from "@/test/utils";

import { WorkspaceOnboarding } from "./workspace-onboarding";

describe("WorkspaceOnboarding", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should submit the resolve-by-path form and report the new workspace id", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces/resolve", "POST"),
        status: 200,
        body: {
          workspace: {
            id: "ws-new",
            name: "new",
            root_dir: "/tmp/new",
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      },
    ]);
    restore = stub.restore;
    const onResolved = vi.fn();
    render(<WorkspaceOnboarding onWorkspaceResolved={onResolved} />, {
      wrapper: withQuery(createTestQueryClient()),
    });
    await userEvent.type(screen.getByTestId("workspace-onboarding-input"), "/tmp/new");
    await userEvent.click(screen.getByTestId("workspace-onboarding-submit"));
    await waitFor(() => expect(onResolved).toHaveBeenCalledWith("ws-new"));
    expect(stub.calls[0]?.body).toBe(JSON.stringify({ path: "/tmp/new" }));
  });

  it("Should surface daemon error messages in the form", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces/resolve", "POST"),
        status: 422,
        body: {
          code: "workspace_resolve_failed",
          message: "path is not a valid workspace",
          request_id: "r",
        },
      },
    ]);
    restore = stub.restore;
    render(<WorkspaceOnboarding />, {
      wrapper: withQuery(createTestQueryClient()),
    });
    await userEvent.type(screen.getByTestId("workspace-onboarding-input"), "/tmp/bad");
    await userEvent.click(screen.getByTestId("workspace-onboarding-submit"));
    expect(await screen.findByTestId("workspace-onboarding-error")).toHaveTextContent(
      "path is not a valid workspace"
    );
  });
});
