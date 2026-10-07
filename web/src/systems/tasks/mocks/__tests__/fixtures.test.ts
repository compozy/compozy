// Suite: Tasks Storybook contract fixtures
// Invariant: Tasks MSW responses preserve the generated counted cursor contract after every server filter.
// Boundary IN: Tasks mock fixtures and their registered MSW handlers.
// Boundary OUT: production adapters, route loaders, and the real daemon contract suites.

import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { storyDefaultWorkspaceId, storyPeople } from "@/storybook/fintech-scenario";
import type { TaskInboxView, TaskListPage } from "../../types";
import { TASK_CATALOG_FIXTURES } from "../fixtures";
import { handlers } from "../handlers";

const server = setupServer(...handlers);

beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("tasks MSW handlers preserve counted query contracts", () => {
  it("Should apply every catalog filter before returning exact page metadata and facets", async () => {
    const query = new URLSearchParams({
      approval_state: "pending",
      include_drafts: "false",
      limit: "1",
      owner_kind: "human",
      owner_ref: storyPeople.productLead,
      parent_task_id: "task_001",
      priority: "urgent",
      query: "public timeout",
      scope: "workspace",
      sort: "priority",
      status: "blocked",
      workspace: storyDefaultWorkspaceId,
    });

    const response = await fetch(`http://localhost/api/tasks?${query}`);
    const body: TaskListPage = await response.json();

    expect(response.status).toBe(200);
    expect(body.tasks.map(task => task.id)).toEqual(["task_006"]);
    expect(body.page).toEqual({ has_more: false, limit: 1, total: 1 });
    expect(body.facets).toEqual({
      owners: [{ count: 1, owner: { kind: "human", ref: storyPeople.productLead } }],
      statuses: [{ count: 1, status: "blocked" }],
    });

    const exclusions: [string, string][] = [
      ["approval_state", "approved"],
      ["owner_kind", "agent_session"],
      ["owner_ref", "another-owner"],
      ["parent_task_id", "task_other"],
      ["priority", "low"],
      ["query", "not present"],
      ["scope", "global"],
      ["status", "ready"],
      ["workspace", "workspace_other"],
    ];
    for (const [name, value] of exclusions) {
      const excludedQuery = new URLSearchParams(query);
      excludedQuery.set(name, value);
      const excludedResponse = await fetch(`http://localhost/api/tasks?${excludedQuery}`);
      const excluded: TaskListPage = await excludedResponse.json();
      expect(excluded.page.total, `${name} must exclude the fixture`).toBe(0);
      expect(excluded.tasks, `${name} must not leak a row`).toEqual([]);
    }
  });

  it("Should sort the complete catalog before cursor paging without duplicating rows", async () => {
    const firstResponse = await fetch(
      "http://localhost/api/tasks?scope=all&include_drafts=true&sort=priority&limit=2"
    );
    const first: TaskListPage = await firstResponse.json();

    expect(first.tasks.map(task => task.id)).toEqual(["task_006", "task_004"]);
    // "Complete" means every work item — Loop execution records are excluded by
    // the daemon's default predicate before counting.
    expect(first.page.total).toBe(TASK_CATALOG_FIXTURES.filter(task => !task.loop).length);
    expect(first.page.has_more).toBe(true);
    expect(first.page.next_cursor).toBeTypeOf("string");

    const secondResponse = await fetch(
      `http://localhost/api/tasks?scope=all&include_drafts=true&sort=priority&limit=2&cursor=${encodeURIComponent(first.page.next_cursor ?? "")}`
    );
    const second: TaskListPage = await secondResponse.json();
    const firstIds = new Set(first.tasks.map(task => task.id));

    expect(second.tasks).toHaveLength(2);
    expect(second.tasks.every(task => !firstIds.has(task.id))).toBe(true);
    expect(second.page.total).toBe(first.page.total);
    expect(second.facets).toEqual(first.facets);
  });

  it("Should exclude drafts by default and include them only when requested", async () => {
    const defaultResponse = await fetch("http://localhost/api/tasks?scope=all&sort=recent");
    const defaultPage: TaskListPage = await defaultResponse.json();
    const draftResponse = await fetch(
      "http://localhost/api/tasks?scope=all&include_drafts=true&sort=recent"
    );
    const withDrafts: TaskListPage = await draftResponse.json();

    expect(defaultPage.tasks.some(task => task.status === "draft")).toBe(false);
    expect(withDrafts.tasks.some(task => task.status === "draft")).toBe(true);
    expect(withDrafts.page.total).toBe(defaultPage.page.total + 1);
  });

  it("Should exclude Loop records by default and reveal them only when asked", async () => {
    const defaultResponse = await fetch("http://localhost/api/tasks?scope=all&sort=recent");
    const defaultPage: TaskListPage = await defaultResponse.json();
    const revealedResponse = await fetch(
      "http://localhost/api/tasks?scope=all&include_loop=true&sort=recent"
    );
    const revealed: TaskListPage = await revealedResponse.json();

    expect(defaultPage.tasks.some(task => task.loop)).toBe(false);
    expect(revealed.tasks.some(task => task.loop?.role === "coordinator")).toBe(true);
    expect(revealed.tasks.some(task => task.loop?.role === "cell")).toBe(true);
    // Counts and facets are computed over the same filtered set the rows come
    // from, so exclusion can never produce an incoherent header.
    expect(revealed.page.total).toBeGreaterThan(defaultPage.page.total);
    const defaultStatusTotal = defaultPage.facets.statuses.reduce(
      (total, facet) => total + facet.count,
      0
    );
    expect(defaultStatusTotal).toBe(defaultPage.page.total);
  });

  it("Should scope to one run and imply include when a run filter is given", async () => {
    const response = await fetch(
      "http://localhost/api/tasks?scope=all&loop_run_id=looprun-8f3ab2c1d4e5f607&sort=recent"
    );
    const page: TaskListPage = await response.json();

    expect(page.tasks.length).toBeGreaterThan(0);
    expect(page.tasks.every(task => task.loop?.run_id === "looprun-8f3ab2c1d4e5f607")).toBe(true);
    expect(page.page.total).toBe(page.tasks.length);
  });

  it("Should keep exact inbox metadata stable while cursor pages distribute items by lane", async () => {
    const firstResponse = await fetch("http://localhost/api/observe/tasks/inbox?limit=2");
    const firstBody: { inbox: TaskInboxView } = await firstResponse.json();
    const first = firstBody.inbox;
    const firstIds = new Set(
      first.groups.flatMap(group => (group.items ?? []).map(item => item.task.id))
    );

    expect(first.page).toEqual({
      has_more: true,
      limit: 2,
      next_cursor: expect.any(String),
      total: 5,
    });
    expect(first.unread_total).toBe(4);
    expect(first.archived_total).toBe(1);
    expect(first.groups.map(group => [group.lane, group.count, group.unread_count])).toEqual([
      ["my_work", 2, 2],
      ["approvals", 1, 1],
      ["failed_runs", 1, 1],
      ["blocked", 0, 0],
      ["archived", 1, 0],
    ]);
    expect(firstIds).toEqual(new Set(["task_001", "task_006"]));

    const secondResponse = await fetch(
      `http://localhost/api/observe/tasks/inbox?limit=2&cursor=${encodeURIComponent(first.page.next_cursor ?? "")}`
    );
    const secondBody: { inbox: TaskInboxView } = await secondResponse.json();
    const second = secondBody.inbox;
    const secondIds = second.groups.flatMap(group => (group.items ?? []).map(item => item.task.id));

    expect(secondIds).toEqual(expect.arrayContaining(["task_002", "task_004"]));
    expect(secondIds.every(id => !firstIds.has(id))).toBe(true);
    expect(second.page.total).toBe(first.page.total);
    expect(second.facets).toEqual(first.facets);
    expect(second.groups.map(group => group.count)).toEqual(first.groups.map(group => group.count));
  });

  it("Should apply actor, lane, task, unread, scope, and search inbox filters before counting", async () => {
    const query = new URLSearchParams({
      lane: "approvals",
      limit: "1",
      owner_kind: "human",
      owner_ref: storyPeople.productLead,
      priority: "urgent",
      query: "public timeout",
      scope: "workspace",
      status: "blocked",
      unread: "true",
      workspace: storyDefaultWorkspaceId,
    });

    const response = await fetch(`http://localhost/api/observe/tasks/inbox?${query}`);
    const body: { inbox: TaskInboxView } = await response.json();
    const inbox = body.inbox;

    expect(inbox.page).toEqual({ has_more: false, limit: 1, total: 1 });
    expect(inbox.groups).toHaveLength(1);
    expect(inbox.groups[0]?.lane).toBe("approvals");
    expect(inbox.groups[0]?.items?.map(item => item.task.id)).toEqual(["task_006"]);
    expect(inbox.facets).toEqual({
      priorities: [{ count: 1, priority: "urgent" }],
      statuses: [{ count: 1, status: "blocked" }],
    });

    const exclusions: [string, string][] = [
      ["lane", "failed_runs"],
      ["owner_kind", "agent_session"],
      ["owner_ref", "another-owner"],
      ["priority", "low"],
      ["query", "not present"],
      ["scope", "global"],
      ["status", "ready"],
      ["unread", "false"],
      ["workspace", "workspace_other"],
    ];
    for (const [name, value] of exclusions) {
      const excludedQuery = new URLSearchParams(query);
      excludedQuery.set(name, value);
      const excludedResponse = await fetch(
        `http://localhost/api/observe/tasks/inbox?${excludedQuery}`
      );
      const excludedBody: { inbox: TaskInboxView } = await excludedResponse.json();
      expect(excludedBody.inbox.page.total, `${name} must exclude the fixture`).toBe(0);
      expect(
        excludedBody.inbox.groups.flatMap(group => group.items ?? []),
        `${name} must not leak an inbox row`
      ).toEqual([]);
    }
  });
});
