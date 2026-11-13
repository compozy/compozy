// Suite: Move to profile (#1185)
// Invariant: the row menu offers every other active profile for a top-level user session; the
// dialog posts exactly the chosen target, refuses a running session before submit, and shows
// the daemon's own refusal (uncommitted worktree changes) without closing.
// Owning layer: SessionProfileMoveDialog over MSW and the row-menu verb it opens from.
// Canonical suite: this file.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, type HttpHandler } from "msw";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider, type MenuTreeNode } from "@compozy/ui";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { createMswFetch } from "@/test/msw-fetch";

import {
  SessionProfileMoveContext,
  type SessionProfileMoveHandle,
} from "../../contexts/session-profile-move-context-value";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionRowMenuTree } from "../../hooks/use-session-row-menu-tree";
import { deriveSourceSessionFixture } from "../../mocks/derive-fixtures";
import type { SessionPayload } from "../../types";
import { SessionProfileMoveDialog } from "../session-profile-move-dialog";

const source: SessionPayload = {
  ...deriveSourceSessionFixture,
  state: "stopped",
  archived_at: null,
  children: null,
  profile_id: "profile-default",
  profile_name: "default",
  worktree_id: "wt-source",
};
const target = { id: "profile-work", name: "work" };
let handlers: HttpHandler[] = [];

function moveHandler(onBody: (body: unknown) => void, refusal?: { code: string; error: string }) {
  return compozyApiMock.post(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile",
    async ({ request }) => {
      onBody(await request.json());
      if (refusal) return HttpResponse.json(refusal, { status: 409 });
      return HttpResponse.json({
        session: { ...source, id: "sess-copy", profile_name: "work" },
        original: { ...source, archived_at: "2026-11-03T12:00:00Z" },
        worktree_id: "wt-new",
        branch: "feat/move",
      });
    }
  );
}

function renderDialog(session: SessionPayload = source) {
  const onOpenChange = vi.fn();
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { mutations: { retry: false } } })}
    >
      <UIProvider reducedMotion="never" skipAnimations>
        <SessionProfileMoveDialog
          onOpenChange={onOpenChange}
          open
          source={session}
          target={target}
        />
      </UIProvider>
    </QueryClientProvider>
  );
  return { onOpenChange };
}

describe("SessionProfileMoveDialog", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should post the chosen profile and close once the session moved", async () => {
    const user = userEvent.setup();
    const onBody = vi.fn();
    handlers = [moveHandler(onBody)];
    const { onOpenChange } = renderDialog();

    expect(screen.getByRole("heading", { name: "Move to profile" })).toBeInTheDocument();
    expect(screen.getByTestId("session-profile-move-usage-note")).toHaveTextContent(
      "Usage so far stays counted in default."
    );
    expect(screen.getByTestId("session-profile-move-worktree-note")).toHaveTextContent(
      "the branch is checked out again for work"
    );
    await user.click(screen.getByTestId("session-profile-move-submit"));

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(onBody).toHaveBeenCalledExactlyOnceWith({ target_profile: "work" });
  });

  it("Should show the daemon's refusal and stay open", async () => {
    const user = userEvent.setup();
    handlers = [
      moveHandler(vi.fn(), {
        code: "session_move_to_profile_worktree_dirty",
        error:
          "the session's worktree has 2 uncommitted file(s) on feat/move; commit or discard them first",
      }),
    ];
    const { onOpenChange } = renderDialog();

    await user.click(screen.getByTestId("session-profile-move-submit"));

    expect(await screen.findByTestId("session-profile-move-error")).toHaveTextContent(
      "uncommitted file(s) on feat/move"
    );
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("Should refuse a running session before anything is sent", () => {
    const onBody = vi.fn();
    handlers = [moveHandler(onBody)];
    renderDialog({ ...source, state: "active" });

    expect(screen.getByTestId("session-profile-move-refusal")).toHaveTextContent(
      "Stop this session before moving it."
    );
    expect(screen.getByTestId("session-profile-move-submit")).toBeDisabled();
    expect(onBody).not.toHaveBeenCalled();
  });
});

describe("Move to profile row verb", () => {
  const actions = {
    pendingAction: null,
    onRename: vi.fn(),
    onStop: vi.fn(),
    onArchive: vi.fn(),
    onUnarchive: vi.fn(),
    onDelete: vi.fn(),
  } as unknown as SessionLifecycleActionHandlers;

  function menuItems(session: SessionPayload, handle: SessionProfileMoveHandle) {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionProfileMoveContext value={handle}>{children}</SessionProfileMoveContext>
    );
    const nodes = renderHook(
      () => useSessionRowMenuTree(session, actions, { testIdPrefix: "session-row" }),
      { wrapper }
    ).result.current;
    return flattenMenu(nodes);
  }

  /** Every node of the tree, submenus and groups included, depth first. */
  function flattenMenu(nodes: readonly MenuTreeNode[]): MenuTreeNode[] {
    return nodes.flatMap(node =>
      node.kind === "sub" || node.kind === "group" ? [node, ...flattenMenu(node.children)] : [node]
    );
  }

  it("Should offer every other active profile under Move and open the dialog for the chosen one", () => {
    const request = vi.fn();
    const items = menuItems(source, {
      targets: [{ id: "profile-default", name: "default" }, target],
      request,
    });
    const move = items.find(item => item.id === "move");
    const verb =
      move?.kind === "sub" ? move.children.find(item => item.id === "move-to-profile") : undefined;
    const choices = verb?.kind === "sub" ? verb.children : [];

    expect(choices.map(choice => (choice.kind === "item" ? choice.label : null))).toEqual(["work"]);
    const choice = choices[0];
    if (choice?.kind === "item") choice.onSelect?.();
    expect(request).toHaveBeenCalledExactlyOnceWith(source, target);
  });

  it("Should not offer the verb for an archived or nested session, or without another profile", () => {
    const handle = { targets: [target], request: vi.fn() };
    expect(
      menuItems({ ...source, archived_at: "2026-11-03T12:00:00Z" }, handle).some(
        item => item.id === "move-to-profile"
      )
    ).toBe(false);
    const nested = {
      ...source,
      lineage: { ...source.lineage, kind: "provenance", parent_session_id: "sess-parent" },
    } as SessionPayload;
    expect(menuItems(nested, handle).some(item => item.id === "move-to-profile")).toBe(false);
    expect(
      menuItems(source, {
        targets: [{ id: "profile-default", name: "default" }],
        request: vi.fn(),
      }).some(item => item.id === "move-to-profile")
    ).toBe(false);
  });
});
