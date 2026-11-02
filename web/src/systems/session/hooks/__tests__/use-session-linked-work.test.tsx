// Suite: session linked work data flow
// Invariant: unlinking is immediate and undone through the add path, a failed write restores the
// API envelope, the picker searches and pages on the server, naming a refused repository, an
// unknown number, a fork's pull request and an unavailable forge as their own states, and a
// multi-link confirm runs at bounded concurrency, keeping and naming only what failed. Each row
// exposes its ticked state.
// Boundary IN: useSessionLinkedWork, useSessionLinkDialogModel, SessionLinkDialogHost and TanStack
// Query.
// Boundary OUT: the adapters, mocked at the network seam.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { toast } from "@compozy/ui";

import { SessionApiError } from "../../adapters/session-api-errors";
import {
  addSessionWorkLink,
  fetchSessionLinkedWork,
  removeSessionWorkLink,
  searchSessionLinkCandidates,
} from "../../adapters/session-linked-work-api";
import { SessionLinkDialogHost } from "../../components/session-linked-work/session-link-dialog-host";
import { sessionKeys } from "../../lib/query-keys";
import { primarySessionFixture } from "../../mocks/fixtures";
import {
  SESSION_LINK_BRANCHES,
  SESSION_LINK_PULL_REQUESTS,
  SESSION_LINK_SETS,
} from "../../mocks/session-links-fixtures";
import { sessionLinkDialogStore } from "../use-session-link-dialog";
import { useSessionLinkDialogModel } from "../use-session-link-dialog-model";
import { useSessionLinkedWork } from "../use-session-linked-work";

vi.mock("../../adapters/session-linked-work-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-linked-work-api")>()),
  addSessionWorkLink: vi.fn(),
  fetchSessionLinkedWork: vi.fn(),
  removeSessionWorkLink: vi.fn(),
  searchSessionLinkCandidates: vi.fn(),
}));

vi.mock("@compozy/ui", async importOriginal => ({
  ...(await importOriginal<typeof import("@compozy/ui")>()),
  toast: { message: vi.fn(), error: vi.fn() },
}));

const WORKSPACE = "ws-one";
const session = { ...primarySessionFixture, id: "session-one" };
const items = SESSION_LINK_SETS.mixed;
const [first, second] = items;

function harness() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

const list = (client: QueryClient) =>
  client.getQueryData<{ items: unknown[] }>(sessionKeys.linkedWork(WORKSPACE, session.id));

function dialog() {
  const { wrapper } = harness();
  return renderHook(
    () =>
      useSessionLinkDialogModel({
        sessionId: session.id,
        workspaceId: WORKSPACE,
        workspaceName: "compozy",
      }),
    { wrapper }
  );
}

function openDialog() {
  const { wrapper } = harness();
  sessionLinkDialogStore.trigger.opened({ sessionId: session.id });
  return render(
    <SessionLinkDialogHost
      sessionId={session.id}
      workspaceId={WORKSPACE}
      workspaceName="compozy"
    />,
    { wrapper }
  );
}

const candidates = (
  pullRequests = SESSION_LINK_PULL_REQUESTS.slice(0, 2),
  overrides: { has_more?: boolean; supports_pr_discovery?: boolean } = {}
) =>
  vi
    .mocked(searchSessionLinkCandidates)
    .mockImplementation(async (_w, _s, query) =>
      query.kind === "branches"
        ? { items: SESSION_LINK_BRANCHES, has_more: false, supports_pr_discovery: false }
        : { items: pullRequests, has_more: false, supports_pr_discovery: true, ...overrides }
    );

beforeEach(() => {
  vi.resetAllMocks();
  sessionLinkDialogStore.trigger.closed();
  vi.mocked(fetchSessionLinkedWork).mockResolvedValue({ items });
});

describe("useSessionLinkedWork", () => {
  it("Should hand the catalog summary's lead to the header before the list answers", () => {
    vi.mocked(fetchSessionLinkedWork).mockReturnValue(new Promise(() => undefined));
    const { wrapper } = harness();
    const { result } = renderHook(
      () =>
        useSessionLinkedWork({
          session: {
            ...session,
            linked_work: { total: 4, needs_you: 1, pull_requests: 4, lead: first },
          },
          workspaceId: WORKSPACE,
          enabled: true,
        }),
      { wrapper }
    );
    expect(result.current.lead).toBe(first);
    expect(result.current.more).toBe("+3 PRs");
  });

  it("Should unlink at once and offer Undo through the add path", async () => {
    let finish!: (value: { items: typeof items }) => void;
    vi.mocked(removeSessionWorkLink).mockReturnValue(
      new Promise(resolve => {
        finish = resolve;
      })
    );
    vi.mocked(addSessionWorkLink).mockResolvedValue({ items });
    const { client, wrapper } = harness();
    const { result } = renderHook(
      () => useSessionLinkedWork({ session, workspaceId: WORKSPACE, enabled: true }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.items).toHaveLength(4));

    act(() => result.current.unlink(first!));
    await waitFor(() => expect(list(client)?.items).toHaveLength(3));
    expect(removeSessionWorkLink).toHaveBeenCalledWith(WORKSPACE, session.id, {
      branch: first!.branch,
      repository: first!.repository,
    });
    await act(async () => finish({ items: items.slice(1) }));

    const [title, options] = vi.mocked(toast.message).mock.calls[0] ?? [];
    expect(title).toBe("Unlinked #951");
    expect(options?.description).toBe(
      "It's still on GitHub. Only the link to this session is gone."
    );
    const undo = options?.action as { label: string; onClick: () => void };
    expect(undo.label).toBe("Undo");
    act(() => undo.onClick());
    await waitFor(() =>
      expect(addSessionWorkLink).toHaveBeenCalledWith(WORKSPACE, session.id, {
        branch: first!.branch,
        repository: first!.repository,
        pr_number: 951,
      })
    );
  });

  it("Should keep Undo on the session it came from after a switch", async () => {
    vi.mocked(removeSessionWorkLink).mockResolvedValue({ items });
    vi.mocked(addSessionWorkLink).mockResolvedValue({ items });
    const { wrapper } = harness();
    const { result, rerender } = renderHook(
      ({ current }) =>
        useSessionLinkedWork({ session: current, workspaceId: WORKSPACE, enabled: true }),
      { initialProps: { current: session }, wrapper }
    );
    await waitFor(() => expect(result.current.items).toHaveLength(4));

    act(() => result.current.unlink(first!));
    await waitFor(() => expect(toast.message).toHaveBeenCalled());
    rerender({ current: { ...session, id: "sess-other" } });
    const undo = vi.mocked(toast.message).mock.calls[0]?.[1]?.action as { onClick: () => void };
    act(() => undo.onClick());

    await waitFor(() => expect(addSessionWorkLink).toHaveBeenCalledTimes(1));
    expect(addSessionWorkLink).toHaveBeenCalledWith(
      WORKSPACE,
      session.id,
      expect.objectContaining({ branch: first!.branch })
    );
  });

  it("Should toast and undo every unlink of a burst", async () => {
    const answers = new Map<string, (value: { items: typeof items }) => void>();
    vi.mocked(removeSessionWorkLink).mockImplementation(
      (_workspace, _session, request) =>
        new Promise(resolve => answers.set(request.branch, resolve))
    );
    vi.mocked(addSessionWorkLink).mockResolvedValue({ items });
    const { wrapper } = harness();
    const { result } = renderHook(
      () => useSessionLinkedWork({ session, workspaceId: WORKSPACE, enabled: true }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.items).toHaveLength(4));

    act(() => result.current.unlink(first!));
    await waitFor(() => expect(answers.has(first!.branch)).toBe(true));
    act(() => result.current.unlink(second!));
    await waitFor(() => expect(answers.has(second!.branch)).toBe(true));
    await act(async () => answers.get(first!.branch)?.({ items: items.slice(2) }));
    await act(async () => answers.get(second!.branch)?.({ items: items.slice(2) }));

    await waitFor(() => expect(toast.message).toHaveBeenCalledTimes(2));
    const calls = vi.mocked(toast.message).mock.calls;
    expect(calls.map(([title]) => title)).toEqual(["Unlinked #951", "Unlinked #948"]);
    const undo = calls[0]?.[1]?.action as { onClick: () => void };
    act(() => undo.onClick());
    await waitFor(() =>
      expect(addSessionWorkLink).toHaveBeenCalledWith(
        WORKSPACE,
        session.id,
        expect.objectContaining({ branch: first!.branch })
      )
    );
  });

  it("Should settle on the daemon's list when one unlink of a burst fails", async () => {
    const answers = new Map<
      string,
      { resolve: (value: { items: typeof items }) => void; reject: (cause: Error) => void }
    >();
    vi.mocked(removeSessionWorkLink).mockImplementation(
      (_workspace, _session, request) =>
        new Promise((resolve, reject) => answers.set(request.branch, { resolve, reject }))
    );
    const { client, wrapper } = harness();
    const { result } = renderHook(
      () => useSessionLinkedWork({ session, workspaceId: WORKSPACE, enabled: true }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.items).toHaveLength(4));

    act(() => result.current.unlink(first!));
    await waitFor(() => expect(answers.has(first!.branch)).toBe(true));
    act(() => result.current.unlink(second!));
    await waitFor(() => expect(answers.has(second!.branch)).toBe(true));
    const remaining = items.filter(item => item !== second);
    await act(async () => answers.get(second!.branch)?.resolve({ items: remaining }));
    vi.mocked(fetchSessionLinkedWork).mockResolvedValue({ items: remaining });
    await act(async () => answers.get(first!.branch)?.reject(new SessionApiError("no", 500)));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Couldn't unlink #951."));
    await waitFor(() => expect(list(client)?.items).toEqual(remaining));
    expect(toast.message).toHaveBeenCalledTimes(1);
  });

  it("Should restore the whole envelope when the unlink fails", async () => {
    vi.mocked(removeSessionWorkLink).mockRejectedValue(new SessionApiError("no", 500));
    const { client, wrapper } = harness();
    const { result } = renderHook(
      () => useSessionLinkedWork({ session, workspaceId: WORKSPACE, enabled: true }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.items).toHaveLength(4));
    act(() => result.current.unlink(second!));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(list(client)).toEqual({ items });
    expect(toast.message).not.toHaveBeenCalled();
  });
});

describe("useSessionLinkDialogModel", () => {
  it("Should hand the query to the server and never filter a loaded page", async () => {
    candidates();
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.onQueryChange("nothing in these titles"));
    await waitFor(() =>
      expect(searchSessionLinkCandidates).toHaveBeenCalledWith(
        WORKSPACE,
        session.id,
        expect.objectContaining({ q: "nothing in these titles", kind: "pull_requests", page: 1 }),
        expect.anything()
      )
    );
    await waitFor(() => {
      const { search } = result.current;
      expect(search.kind === "results" ? search.pullRequests : []).toHaveLength(2);
    });
  });

  it("Should page on the server by its own answer, not by a page size", async () => {
    // Page one may carry one extra exact match, and the pages after it don't repeat that match.
    const pages = [SESSION_LINK_PULL_REQUESTS.slice(0, 31), SESSION_LINK_PULL_REQUESTS.slice(31)];
    vi.mocked(searchSessionLinkCandidates).mockImplementation(async (_w, _s, query) =>
      query.kind === "branches"
        ? { items: [], has_more: false, supports_pr_discovery: false }
        : {
            items: pages[query.page - 1] ?? [],
            has_more: query.page < pages.length,
            supports_pr_discovery: true,
          }
    );
    const { result } = dialog();
    await waitFor(() => expect(result.current.hasMore).toBe(true));
    act(() => result.current.onLoadMore());
    await waitFor(() => expect(result.current.hasMore).toBe(false));
    expect(searchSessionLinkCandidates).toHaveBeenCalledWith(
      WORKSPACE,
      session.id,
      expect.objectContaining({ kind: "pull_requests", page: 2 }),
      expect.anything()
    );
    const { search } = result.current;
    expect(search.kind === "results" ? search.pullRequests : []).toEqual(
      SESSION_LINK_PULL_REQUESTS
    );
  });

  it("Should keep branches linkable with no pull request discovery", async () => {
    candidates([], { supports_pr_discovery: false });
    const { result } = dialog();
    await waitFor(() => expect(result.current.offline).toBe(true));
    const { search } = result.current;
    expect(search.kind === "results" ? search.branches : []).toEqual(SESSION_LINK_BRANCHES);
  });

  it("Should never show the daemon's identity for a repository with no hosted forge", async () => {
    candidates([], { supports_pr_discovery: false });
    vi.mocked(fetchSessionLinkedWork).mockResolvedValue({
      items: [{ ...SESSION_LINK_SETS.worktreeNoPr[0]!, repository: `local:${WORKSPACE}` }],
    });
    const { result } = dialog();
    await waitFor(() => expect(result.current.linked.size).toBeGreaterThan(0));
    expect(result.current.forge).toBe("The forge");
    expect(result.current.forgeless).toBe(true);
    expect(result.current.project).toBe("compozy");
  });

  it("Should treat a forge failure as offline, not as a dead dialog", async () => {
    vi.mocked(searchSessionLinkCandidates).mockImplementation(async (_w, _s, query) => {
      if (query.kind === "pull_requests") throw new SessionApiError("forge down", 502);
      return { items: SESSION_LINK_BRANCHES, has_more: false, supports_pr_discovery: false };
    });
    const { result } = dialog();
    await waitFor(() => expect(result.current.offline).toBe(true));
    const { search } = result.current;
    expect(search.kind === "results" ? search.branches : []).toEqual(SESSION_LINK_BRANCHES);
  });

  it("Should tick a pasted link's pull request and let it be unticked", async () => {
    candidates();
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.onQueryChange("https://github.com/compozy/compozy/pull/963"));
    await waitFor(() => expect(result.current.selected).toHaveLength(1));
    act(() => result.current.onToggle(result.current.selected[0]!));
    expect(result.current.selected).toHaveLength(0);
  });

  it("Should leave nothing selected by another project's link to the same number", async () => {
    candidates();
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.onQueryChange("https://github.com/acme/billing/pull/963"));
    await waitFor(() => expect(result.current.search.kind).toBe("other-project"));
    expect(result.current.selected).toEqual([]);
  });

  it("Should say another project owns the link when the daemon refuses it", async () => {
    candidates([]);
    vi.mocked(fetchSessionLinkedWork).mockResolvedValue({ items: [] });
    vi.mocked(addSessionWorkLink).mockRejectedValue(
      new SessionApiError("refused", 400, session.id, { code: "session_link_repository_mismatch" })
    );
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.onQueryChange("https://github.com/acme/billing/pull/212"));
    act(() => result.current.onToggle(SESSION_LINK_PULL_REQUESTS[0]!));
    await waitFor(() => expect(result.current.selected.length).toBeGreaterThan(0));
    await act(async () => result.current.onConfirm());
    await waitFor(() => expect(result.current.search.kind).toBe("other-project"));
    expect(result.current.error).toBeNull();
  });

  it("Should send a pasted link whole and show the daemon's refusal with nothing linked", async () => {
    const pasted = "https://github.com/acme/billing/pull/212";
    vi.mocked(fetchSessionLinkedWork).mockResolvedValue({ items: [] });
    vi.mocked(searchSessionLinkCandidates).mockImplementation(async (_w, _s, query) => {
      if (query.q === pasted) {
        throw new SessionApiError("refused", 400, session.id, {
          code: "session_link_repository_mismatch",
        });
      }
      return { items: [], has_more: false, supports_pr_discovery: true };
    });
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.onQueryChange(pasted));
    await waitFor(() =>
      expect(result.current.search).toEqual({
        kind: "other-project",
        repository: "github.com/acme/billing",
        number: 212,
      })
    );
    expect(result.current.offline).toBe(false);
  });

  it("Should tell assistive technology which rows are ticked", async () => {
    candidates(SESSION_LINK_PULL_REQUESTS.slice(0, 3));
    openDialog();
    const row = await screen.findByRole("option", { name: /monthly usage report/ });
    expect(row).toHaveAttribute("aria-checked", "false");
    fireEvent.click(row);
    expect(row).toHaveAttribute("aria-checked", "true");
    const linked = await screen.findByRole("option", { name: /Fix the login redirect loop/ });
    expect(linked).toHaveAttribute("aria-disabled", "true");
    expect(linked).not.toHaveAttribute("aria-checked");
  });

  it("Should refuse a fork's pull request by name and let the rest of the selection through", async () => {
    candidates();
    vi.mocked(addSessionWorkLink).mockImplementation(async (_workspace, _session, request) => {
      if (request.pr_number === 963) {
        throw new SessionApiError("fork", 400, session.id, {
          code: "session_link_fork_unsupported",
        });
      }
      return { items };
    });
    openDialog();
    fireEvent.click(await screen.findByRole("option", { name: /monthly usage report/ }));
    fireEvent.click(screen.getByRole("option", { name: /login redirect on Safari/ }));
    fireEvent.click(screen.getByRole("button", { name: "Link 2" }));
    expect(await screen.findByText("#963 comes from a fork")).toBeInTheDocument();
    expect(screen.getByText("Pull requests from forks can't be linked yet.")).toBeInTheDocument();
    expect(screen.queryByText(/Couldn't link/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Link" })).toBeDisabled();
    expect(addSessionWorkLink).toHaveBeenCalledTimes(2);
    expect(sessionLinkDialogStore.getSnapshot().context.sessionId).toBe(session.id);
  });

  it("Should link a selection at bounded concurrency and name what failed", async () => {
    candidates(SESSION_LINK_PULL_REQUESTS.slice(0, 6));
    sessionLinkDialogStore.trigger.opened({ sessionId: session.id });
    const finish = new Map<number, () => void>();
    vi.mocked(addSessionWorkLink).mockImplementation(
      (_workspace, _session, request) =>
        new Promise((resolve, reject) => {
          finish.set(request.pr_number!, () =>
            request.pr_number === 962 || request.pr_number === 951
              ? reject(new SessionApiError("not found", 400))
              : resolve({ items })
          );
        })
    );
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    const picks = SESSION_LINK_PULL_REQUESTS.slice(0, 5);
    for (const pick of picks) act(() => result.current.onToggle(pick));
    act(() => result.current.onConfirm());
    await waitFor(() => expect(finish.size).toBe(3));
    await act(async () => {
      await Promise.resolve();
    });
    expect(addSessionWorkLink).toHaveBeenCalledTimes(3);
    await waitFor(async () => {
      const running = [...finish.values()];
      finish.clear();
      await act(async () => running.forEach(settle => settle()));
      expect(result.current.pending).toBe(false);
    });
    expect(addSessionWorkLink).toHaveBeenCalledTimes(picks.length);
    expect(result.current.selected.map(candidate => candidate.pr_number).sort()).toEqual([
      951, 962,
    ]);
    expect(result.current.error).toBe("Couldn't link #962, #951: not found");
    expect(sessionLinkDialogStore.getSnapshot().context.sessionId).toBe(session.id);
  });

  it("Should not leave a pasted pull request selected once it is linked", async () => {
    candidates();
    sessionLinkDialogStore.trigger.opened({ sessionId: session.id });
    vi.mocked(addSessionWorkLink).mockImplementation(async (_workspace, _session, request) => {
      if (request.pr_number === 962) throw new SessionApiError("not found", 400);
      return { items };
    });
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.onToggle(SESSION_LINK_PULL_REQUESTS[1]!));
    act(() => result.current.onQueryChange("#963"));
    await waitFor(() => expect(result.current.selected).toHaveLength(2));
    await act(async () => result.current.onConfirm());
    await waitFor(() => expect(result.current.error).not.toBeNull());
    expect(result.current.selected.map(candidate => candidate.pr_number)).toEqual([962]);
  });

  it("Should settle on the daemon's list when a batch's answers land out of order", async () => {
    candidates();
    sessionLinkDialogStore.trigger.opened({ sessionId: session.id });
    // Every write answers with a list older than the batch; only the reread is complete.
    vi.mocked(addSessionWorkLink).mockResolvedValue({ items: [] });
    const { result } = dialog();
    await waitFor(() => expect(result.current.linked.size).toBeGreaterThan(0));
    const complete = result.current.linked;
    act(() => result.current.onToggle(SESSION_LINK_PULL_REQUESTS[0]!));
    act(() => result.current.onToggle(SESSION_LINK_PULL_REQUESTS[1]!));
    await act(async () => result.current.onConfirm());
    await waitFor(() => expect(addSessionWorkLink).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.current.linked).toEqual(complete));
  });

  it("Should report a refused pick when no pasted link can say so", async () => {
    candidates();
    vi.mocked(addSessionWorkLink).mockRejectedValue(
      new SessionApiError("refused", 400, session.id, { code: "session_link_repository_mismatch" })
    );
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.onToggle(SESSION_LINK_PULL_REQUESTS[0]!));
    await act(async () => result.current.onConfirm());
    await waitFor(() => expect(result.current.error).toBe("Couldn't link #963: refused"));
    expect(result.current.search.kind).toBe("results");
  });

  it("Should keep what failed selected and close once everything is linked", async () => {
    candidates();
    sessionLinkDialogStore.trigger.opened({ sessionId: session.id });
    vi.mocked(addSessionWorkLink)
      .mockRejectedValueOnce(new SessionApiError("not found", 400))
      .mockResolvedValue({ items });
    const { result } = dialog();
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.onToggle(SESSION_LINK_PULL_REQUESTS[0]!));
    await act(async () => result.current.onConfirm());
    await waitFor(() => expect(result.current.error).toBe("Couldn't link #963: not found"));
    expect(result.current.selected).toHaveLength(1);
    expect(sessionLinkDialogStore.getSnapshot().context.sessionId).toBe(session.id);
    await act(async () => result.current.onConfirm());
    await waitFor(() => expect(sessionLinkDialogStore.getSnapshot().context.sessionId).toBeNull());
  });
});
