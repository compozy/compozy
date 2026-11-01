import { Profiler } from "react";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toast, UIProvider } from "@compozy/ui";

import { SessionInspectorSubagentsSection } from "../session-inspector-subagents-section";
import { SubagentCard } from "../subagent-card";
import { SubagentComposerControl } from "../subagent-composer-control";
import { SubagentOriginDivider } from "../subagent-origin-divider";
import { SubagentWaitingBanner } from "../subagent-waiting-banner";
import type { SubagentStatus, SubagentView, SubagentWorktreeView } from "../types";

beforeEach(() => {
  vi.spyOn(toast, "error").mockReturnValue("toast");
  vi.spyOn(toast, "success").mockReturnValue("toast");
});

// Suite: subagent presentational components.
// Invariant: rendered copy, roles and controls of the card, hover card, divider, banner, chip,
// composer control (issue #766) and inspector roster follow `_uiux.md` S1–S5, S9, S10 (and
// agent-collaboration S5 worktree facts); elapsed ticks from server timestamps on a shared ticker
// without re-rendering React. Owning layer: session subagent components (props in, callbacks out).
// Canonical suite: this file. Boundary OUT: sonner toast.
const NO_RUNTIME = { agent: "", provider: "", model: "", reasoning_effort: "", speed: "" };
const T0 = Date.parse("2026-10-08T21:00:00.000Z");
const at = (seconds: number) => new Date(T0 + seconds * 1_000).toISOString();

function subagent(overrides: Partial<SubagentView> = {}): SubagentView {
  return {
    id: "sub-1",
    parent_session_id: "sess-parent",
    result_preview: "",
    error: null,
    child_session_id: "sess-child",
    origin: "delegated",
    title: "Audit payment webhooks for retry safety",
    status: "running",
    progress: "",
    runtime: {
      ...NO_RUNTIME,
      provider: "claude",
      model: "opus-5.5",
      reasoning_effort: "high",
      speed: "normal",
    },
    started_at: at(0),
    settled_at: null,
    created_at: at(0),
    updated_at: at(0),
    delivery: "none",
    ...overrides,
  };
}

const many = (count: number, status: SubagentStatus) =>
  Array.from({ length: count }, (_, index) =>
    subagent({
      id: `sub-${status}-${index}`,
      title: `${status} subagent ${index}`,
      status,
      created_at: at(index),
      settled_at: ["queued", "running", "waiting"].includes(status) ? null : at(100),
    })
  );

// Popovers and hover cards animate their opacity; instant animations keep visibility assertions
// independent of how busy the test worker is.
function TestUI({ children }: { children: React.ReactNode }) {
  return (
    <UIProvider reducedMotion="never" skipAnimations>
      {children}
    </UIProvider>
  );
}

const renderUI = (ui: React.ReactNode) => render(<TestUI>{ui}</TestUI>);

beforeEach(() => {
  vi.mocked(toast.error).mockClear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("SubagentCard", () => {
  // Invariant: pending admission is navigable without inventing a child session.
  // Owner: subagent presentational components; canonical suite: this file.
  it("Should open a pending subagent and show its recorded runtime", () => {
    const pending = subagent({ child_session_id: null, status: "queued" });
    const onOpen = vi.fn();
    renderUI(<SubagentCard subagent={pending} onOpen={onOpen} />);
    const control = screen.getByRole("button", { name: `Open ${pending.title}` });
    fireEvent.click(control);
    expect(onOpen).toHaveBeenCalledWith(pending, { newWindow: false });
    expect(within(control).getByText("opus-5.5 · high")).toBeVisible();
  });
  it("Should be a labeled button that opens the child, in a new window with ⌘ (UT-W05)", () => {
    const onOpen = vi.fn();
    renderUI(
      <SubagentCard subagent={subagent({ progress: "Reading webhook.go" })} onOpen={onOpen} />
    );
    const card = screen.getByRole("button", {
      name: "Open Audit payment webhooks for retry safety",
    });
    expect(card).toHaveAttribute("aria-description", "Working");
    expect(within(card).getByText("Reading webhook.go")).toBeInTheDocument();
    fireEvent.click(card, { metaKey: true });
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: "sub-1" }), {
      newWindow: true,
    });
  });

  // Invariant: running and completed rows render distinct glyph states and words (the
  // "everything looks done" regression); settled titles dim in place.
  it("Should mark a running and a completed card with distinct glyphs and words", () => {
    renderUI(
      <>
        <SubagentCard subagent={subagent({ id: "run", title: "Running one" })} />
        <SubagentCard
          subagent={subagent({
            id: "done",
            title: "Finished one",
            status: "completed",
            settled_at: at(65),
            result_preview: "Shipped the fix",
          })}
        />
      </>
    );
    const [running, done] = [...document.querySelectorAll('[data-slot="subagent-card"]')];
    const status = (card: Element | undefined) =>
      card?.querySelector('[data-slot="subagent-status"]') as HTMLElement;
    expect(status(running)).toHaveAttribute("data-state", "running");
    expect(status(running).querySelector('[data-slot="state-glyph"]')).toHaveAttribute(
      "data-state",
      "running"
    );
    expect(status(running)).toHaveTextContent(/^Working/);
    expect(status(done).querySelector('[data-slot="state-glyph"]')).toHaveAttribute(
      "data-state",
      "done"
    );
    expect(status(done)).toHaveTextContent("Done1m 05s");
    expect(within(done as HTMLElement).getByText("Finished one")).toHaveClass(
      "text-muted-foreground"
    );
    expect(within(running as HTMLElement).getByText("Running one")).not.toHaveClass(
      "text-muted-foreground"
    );
    // The provider avatar no longer carries a status dot on subagent rows.
    expect(
      document.querySelector('[data-slot="subagent-avatar"] [data-slot="status-dot"]')
    ).toBeNull();
  });

  it("Should keep a provider-native card static when its host provides no navigation (UT-W05)", () => {
    const { container } = renderUI(
      <SubagentCard subagent={subagent({ origin: "provider_native", child_session_id: null })} />
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(container.querySelector('[data-slot="subagent-card-chevron"]')).toBeNull();
  });

  it("Should render the bot glyph for an unknown provider (UT-W05)", () => {
    const { container } = renderUI(
      <SubagentCard subagent={subagent({ runtime: { ...NO_RUNTIME, provider: "mystery" } })} />
    );
    expect(
      container.querySelector('[data-kind="mystery"] [data-slot="kind-icon-fallback"]')
    ).not.toBeNull();
  });

  it("Should tick elapsed from started_at on the shared ticker without re-rendering React (UT-W06)", () => {
    vi.useFakeTimers();
    vi.setSystemTime(T0 + 724_000);
    const onRender = vi.fn();
    renderUI(
      <Profiler id="card" onRender={onRender}>
        <SubagentCard subagent={subagent()} />
      </Profiler>
    );
    const live = document.querySelector('[data-slot="subagent-elapsed"][data-live="true"]');
    expect(live).toHaveTextContent("12m 04s");
    const renders = onRender.mock.calls.length;
    act(() => {
      vi.advanceTimersByTime(1_000);
    });
    expect(live).toHaveTextContent("12m 05s");
    expect(onRender.mock.calls.length).toBe(renders);
  });

  it("Should freeze a settled card at settled_at and never tick a stale one (UT-W06)", () => {
    vi.useFakeTimers();
    vi.setSystemTime(T0 + 999_000);
    renderUI(
      <>
        <SubagentCard
          subagent={subagent({ id: "a", status: "completed", settled_at: at(3_780) })}
        />
        <SubagentCard subagent={subagent({ id: "b", updated_at: at(45) })} stale />
      </>
    );
    act(() => {
      vi.advanceTimersByTime(5_000);
    });
    const labels = [...document.querySelectorAll('[data-slot="subagent-elapsed"]')];
    expect(labels.map(label => label.textContent)).toEqual(["1h 03m", "45s"]);
    expect(labels.every(label => !label.hasAttribute("data-live"))).toBe(true);
  });

  it("Should still open on hover after an unconfirmed static card becomes a drill-in button", async () => {
    const user = userEvent.setup();
    const unconfirmed = subagent({ child_session_id: null });
    const { rerender } = renderUI(<SubagentCard subagent={unconfirmed} onOpen={vi.fn()} />);
    // The roster confirms the child: the static row becomes the drill-in button.
    rerender(
      <TestUI>
        <SubagentCard subagent={subagent()} onOpen={vi.fn()} />
      </TestUI>
    );
    await user.hover(
      screen.getByRole("button", { name: "Open Audit payment webhooks for retry safety" })
    );
    await waitFor(() =>
      expect(document.querySelector('[data-slot="subagent-hover"]')).not.toBeNull()
    );
  });

  it("Should open the hover card on focus with Not reported for a missing model (UT-W08)", async () => {
    const user = userEvent.setup();
    renderUI(
      <SubagentCard
        subagent={subagent({
          runtime: { ...NO_RUNTIME, provider: "claude" },
          progress: "Reading tickets",
        })}
        onOpen={vi.fn()}
      />
    );
    await user.tab();
    const hover = await waitFor(() => {
      const node = document.querySelector('[data-slot="subagent-hover"]');
      expect(node).not.toBeNull();
      return node as HTMLElement;
    });
    expect(within(hover).getByText("Not reported")).toBeInTheDocument();
    expect(within(hover).getByText("Reading tickets")).toBeInTheDocument();
  });
});

describe("SubagentOriginDivider (UT-W12)", () => {
  it("Should read Subagent of <parent> with an Open parent action", () => {
    const onOpenParent = vi.fn();
    renderUI(
      <SubagentOriginDivider
        parent={{ id: "sess-parent", title: "Ship checkout v2" }}
        onOpenParent={onOpenParent}
      />
    );
    expect(
      screen.getByRole("separator", { name: "Subagent of Ship checkout v2" })
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open parent" }));
    expect(onOpenParent).toHaveBeenCalledWith("sess-parent");
  });

  it("Should read Subagent of a deleted session with no action once the parent is gone", () => {
    renderUI(<SubagentOriginDivider parent={null} onOpenParent={vi.fn()} />);
    expect(
      screen.getByRole("separator", { name: "Subagent of a deleted session" })
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});

describe("SubagentWaitingBanner (UT-W13)", () => {
  it("Should name the one subagent in the title", () => {
    const onOpen = vi.fn();
    renderUI(
      <SubagentWaitingBanner
        subagents={[subagent({ title: "Draft release notes" })]}
        onOpen={onOpen}
      />
    );
    expect(
      screen.getByRole("region", { name: /Waiting on subagent Draft release notes/ })
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open subagent Draft release notes" }));
    expect(onOpen).toHaveBeenCalledOnce();
  });

  it("Should count many, name three, and send the rest to the inspector", () => {
    const onShowAll = vi.fn();
    renderUI(<SubagentWaitingBanner subagents={many(5, "running")} onShowAll={onShowAll} />);
    expect(screen.getByText("Waiting on 5 subagents")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /^Open subagent / })).toHaveLength(3);
    fireEvent.click(screen.getByRole("button", { name: "2 more" }));
    expect(onShowAll).toHaveBeenCalledOnce();
  });

  it("Should show Stopping… while pending and toast when the stop fails", async () => {
    let reject: (error: Error) => void = () => undefined;
    const onStop = vi.fn(() => new Promise((_, fail) => (reject = fail)));
    renderUI(<SubagentWaitingBanner subagents={many(3, "running")} onStop={onStop} />);
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
    expect(screen.getByRole("button", { name: "Stopping…" })).toBeDisabled();
    await act(async () => reject(new Error("boom")));
    expect(toast.error).toHaveBeenCalledWith("Could not stop subagents.");
    expect(screen.getByRole("button", { name: "Stop" })).toBeEnabled();
  });

  it("Should stay Stopping… after the 202 until the stopped rows actually change (m19)", async () => {
    const onStop = vi.fn(() => Promise.resolve({ status: "cancel_requested" }));
    const live = many(2, "running");
    const { rerender } = renderUI(<SubagentWaitingBanner subagents={live} onStop={onStop} />);
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
    await act(async () => undefined);
    // The daemon accepted the stop but has not settled the rows: no second Stop yet.
    expect(screen.getByRole("button", { name: "Stopping…" })).toBeDisabled();
    // A subagent that starts mid-Stop does not end the pending state of the ones stopped.
    const late = subagent({ id: "sub-late", title: "late subagent", status: "running" });
    rerender(
      <TestUI>
        <SubagentWaitingBanner subagents={[...live, late]} onStop={onStop} />
      </TestUI>
    );
    expect(screen.getByRole("button", { name: "Stopping…" })).toBeDisabled();
    rerender(
      <TestUI>
        <SubagentWaitingBanner subagents={[late]} onStop={onStop} />
      </TestUI>
    );
    expect(screen.getByRole("button", { name: "Stop" })).toBeEnabled();
  });

  it("Should render nothing with no live delegated subagents", () => {
    const { container } = renderUI(<SubagentWaitingBanner subagents={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe("SubagentComposerControl (issue #766)", () => {
  const counts = (live: number, total: number, attention = 0) => ({
    live,
    total,
    failed: 0,
    attention,
  });

  it("Should stay a quiet icon with an empty popover while the session has none", async () => {
    const user = userEvent.setup();
    renderUI(<SubagentComposerControl subagents={[]} counts={counts(0, 0)} />);
    const trigger = screen.getByRole("button", { name: "Subagents, none yet" });
    expect(trigger).toHaveAttribute("data-state", "idle");
    expect(trigger).not.toHaveTextContent(/\d/);
    // Icon-only, it is the composer's square icon tool (#826).
    expect(trigger).toHaveAttribute("data-size", "tool-icon");
    await user.click(trigger);
    expect(await screen.findByText("Nothing delegated yet.")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Subagents" })).toBeInTheDocument();
  });

  it("Should read the live count with the avatar's live dot while subagents run", () => {
    const rows = [...many(2, "running"), ...many(1, "waiting"), ...many(3, "completed")];
    const { rerender } = renderUI(
      <SubagentComposerControl subagents={rows} counts={counts(3, 6, 1)} />
    );
    const trigger = screen.getByRole("button", { name: "Subagents, 3 running, 1 needs you" });
    expect(trigger).toHaveAttribute("data-state", "attention");
    expect(trigger).toHaveTextContent("3");
    // The live count widens it into the padded tool pill at the same height (#826).
    expect(trigger).toHaveAttribute("data-size", "tool");
    expect(trigger.querySelector('[data-slot="status-dot"]')).toHaveAttribute("data-tone", "brand");
    rerender(
      <TestUI>
        <SubagentComposerControl subagents={rows} counts={counts(2, 6)} />
      </TestUI>
    );
    const running = screen.getByRole("button", { name: "Subagents, 2 running" });
    expect(running).toHaveAttribute("data-state", "running");
    expect(running.querySelector('[data-slot="status-dot"]')).toHaveAttribute(
      "data-tone",
      "success"
    );
  });

  it("Should list current rows first, fold previous ones, and return focus on Escape", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    renderUI(
      <SubagentComposerControl
        subagents={[...many(2, "completed"), ...many(1, "running")]}
        counts={counts(1, 3)}
        onOpen={onOpen}
      />
    );
    const trigger = screen.getByRole("button", { name: "Subagents, 1 running" });
    trigger.focus();
    await user.keyboard("{Enter}");
    const popover = await screen.findByRole("dialog");
    expect(within(popover).getByText("Subagents · 1 running")).toBeInTheDocument();
    expect(
      within(popover)
        .getAllByRole("button", { name: /^Open / })
        .map(row => row.getAttribute("aria-label"))
    ).toEqual(["Open running subagent 0"]);
    expect(within(popover).getByRole("button", { name: "Previous subagents (2)" })).toHaveAttribute(
      "aria-expanded",
      "false"
    );
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await waitFor(() => expect(trigger).toHaveFocus());
    await user.click(trigger);
    await user.click(await screen.findByRole("button", { name: "Open running subagent 0" }));
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: "sub-running-0" }), {
      newWindow: false,
    });
  });

  it("Should open previous rows when nothing runs and hold their place while they load", async () => {
    const user = userEvent.setup();
    const { rerender } = renderUI(
      <SubagentComposerControl subagents={[]} counts={counts(0, 2)} loading onOpen={vi.fn()} />
    );
    const trigger = screen.getByRole("button", { name: "Subagents, none running" });
    await user.click(trigger);
    const popover = await screen.findByRole("dialog");
    // The summary's two rows hold their place under one busy status.
    const loadingRows = within(popover).getByRole("status");
    expect(loadingRows).toHaveTextContent("Loading subagents");
    expect(loadingRows.querySelectorAll("li")).toHaveLength(2);
    rerender(
      <TestUI>
        <SubagentComposerControl
          subagents={many(2, "completed")}
          counts={counts(0, 2)}
          onOpen={vi.fn()}
        />
      </TestUI>
    );
    expect(await screen.findByRole("button", { name: "Previous subagents (2)" })).toHaveAttribute(
      "aria-expanded",
      "true"
    );
    expect(screen.getAllByRole("button", { name: /^Open completed/ })).toHaveLength(2);
  });

  it("Should say the rows could not be read and offer Try again", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderUI(
      <SubagentComposerControl
        subagents={[]}
        counts={{ live: 1, total: 2, failed: 0, attention: 0 }}
        loading
        failed
        onRetry={onRetry}
      />
    );
    await user.click(screen.getByRole("button", { name: "Subagents, 1 running" }));
    const popover = await screen.findByRole("dialog");
    expect(within(popover).getByText("Couldn't load subagents.")).toBeInTheDocument();
    expect(popover.querySelector('[data-slot="skeleton-rows"]')).toBeNull();
    await user.click(within(popover).getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("Should not call an unread roster empty when its read failed", () => {
    renderUI(<SubagentComposerControl subagents={[]} counts={counts(0, 0)} failed />);
    expect(screen.getByRole("button", { name: "Subagents, couldn't load" })).toHaveAttribute(
      "data-state",
      "idle"
    );
  });

  it("Should keep the rows that loaded and still offer Try again when a later page fails", async () => {
    const user = userEvent.setup();
    renderUI(
      <SubagentComposerControl
        subagents={many(1, "running")}
        counts={counts(3, 250)}
        failed
        onRetry={vi.fn()}
        onOpen={vi.fn()}
      />
    );
    await user.click(screen.getByRole("button", { name: "Subagents, 3 running" }));
    const popover = await screen.findByRole("dialog");
    await waitFor(() =>
      expect(within(popover).getByRole("button", { name: "Open running subagent 0" })).toBeVisible()
    );
    expect(within(popover).getByText("Couldn't load subagents.")).toBeInTheDocument();
    expect(within(popover).getByRole("button", { name: "Try again" })).toBeInTheDocument();
  });
});

describe("SessionInspectorSubagentsSection (UT-W19)", () => {
  it("Should be absent when the session has no subagents", () => {
    const { container } = renderUI(<SessionInspectorSubagentsSection subagents={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("Should pin failed rows above live rows and fold previous rows", () => {
    renderUI(
      <SessionInspectorSubagentsSection
        subagents={[
          ...many(1, "completed"),
          ...many(2, "running"),
          ...many(1, "failed"),
          ...many(1, "waiting"),
        ]}
        onOpen={vi.fn()}
      />
    );
    expect(screen.getByText("Subagents · 3 running")).toBeInTheDocument();
    const names = screen
      .getAllByRole("button", { name: /^Open / })
      .map(row => row.getAttribute("aria-label"));
    expect(names).toEqual([
      "Open failed subagent 0",
      "Open waiting subagent 0",
      "Open running subagent 1",
      "Open running subagent 0",
    ]);
    const states = [...document.querySelectorAll('[data-slot="subagent-roster-row"]')].map(node =>
      node.querySelector('[data-slot="state-glyph"]')?.getAttribute("data-state")
    );
    expect(states).toEqual(["failed", "attention", "running", "running"]);
    const previous = screen.getByRole("button", { name: "Previous subagents (1)" });
    expect(previous).toHaveAttribute("aria-expanded", "false");
    expect(previous).toHaveTextContent(/^Previous subagents \(1\)$/);
  });

  it("Should show six rows first, then Show 12 more", () => {
    renderUI(<SessionInspectorSubagentsSection subagents={many(20, "running")} onOpen={vi.fn()} />);
    expect(screen.getAllByRole("button", { name: /^Open / })).toHaveLength(6);
    fireEvent.click(screen.getByRole("button", { name: "Show 12 more" }));
    expect(screen.getAllByRole("button", { name: /^Open / })).toHaveLength(18);
    fireEvent.click(screen.getByRole("button", { name: "Show 2 more" }));
    expect(screen.getAllByRole("button", { name: /^Open / })).toHaveLength(20);
    expect(screen.queryByRole("button", { name: /^Show / })).not.toBeInTheDocument();
  });

  it("Should keep native work keyboard-accessible and open it from the inspector (issue #947)", async () => {
    const user = userEvent.setup();
    const native = subagent({ id: "native", origin: "provider_native", child_session_id: null });
    const onOpen = vi.fn();
    renderUI(<SessionInspectorSubagentsSection subagents={[native]} onOpen={onOpen} />);
    const row = screen.getByRole("button", { name: `Open ${native.title}` });
    await user.tab();
    expect(row).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onOpen).toHaveBeenCalledWith(native, { newWindow: false });
    await waitFor(() =>
      expect(document.querySelector('[data-slot="subagent-hover"]')).not.toBeNull()
    );
  });

  it("Should offer Stop on live delegated rows only and toast when it fails", async () => {
    let reject: (error: Error) => void = () => undefined;
    const onStop = vi.fn(() => new Promise((_, fail) => (reject = fail)));
    renderUI(
      <SessionInspectorSubagentsSection
        subagents={[
          ...many(1, "running"),
          subagent({ id: "native", origin: "provider_native", child_session_id: null }),
        ]}
        onStop={onStop}
      />
    );
    const stops = screen.getAllByRole("button", { name: "Stop subagent" });
    expect(stops).toHaveLength(1);
    fireEvent.click(stops[0]!);
    expect(onStop).toHaveBeenCalledWith(expect.objectContaining({ id: "sub-running-0" }));
    expect(screen.getByRole("button", { name: "Stopping subagent" })).toBeDisabled();
    await act(async () => reject(new Error("boom")));
    expect(toast.error).toHaveBeenCalledWith("Could not stop subagent");
    expect(screen.getByRole("button", { name: "Stop subagent" })).toBeEnabled();
  });
});

describe("Isolated subagent worktree facts (UT-070, UT-071, UT-072)", () => {
  const PR_URL = "https://github.com/compozy/compozy/pull/731";
  function worktree(overrides: Partial<SubagentWorktreeView> = {}): SubagentWorktreeView {
    return {
      id: "wt-5d1a7c0e",
      name: "extract-billing-client-3f9a0c12",
      branch: "run/extract-billing-client-3f9a0c12",
      base_ref: "origin/main",
      base_sha: "4be1c9d2a7f0e6b13c5d8e9f0a1b2c3d4e5f6a7b",
      path: "/w/extract-billing-client-3f9a0c12",
      head_sha: "9c41e07",
      commits_ahead: 3,
      dirty_files: 0,
      observed_at: new Date(Date.now() - 240_000).toISOString(),
      pull_request_status: "open",
      pull_request: { url: PR_URL, number: 731, state: "open" },
      ...overrides,
    };
  }
  const isolated = (
    overrides: Partial<SubagentWorktreeView> = {},
    status: SubagentStatus = "completed"
  ) =>
    subagent({
      title: "Extract billing client",
      status,
      settled_at: status === "running" ? null : at(567),
      result_preview: "Moved the client into internal/billing/client",
      isolation: "worktree",
      worktree: worktree(overrides),
    });

  async function focusHover() {
    const user = userEvent.setup();
    await user.tab();
    return waitFor(() => {
      const node = document.querySelector('[data-slot="subagent-hover-facts"]');
      expect(node).not.toBeNull();
      return node as HTMLElement;
    });
  }
  const factLabels = (facts: HTMLElement) =>
    [...facts.querySelectorAll('[data-slot="property-row-label"]')].map(label => label.textContent);
  const factText = (facts: HTMLElement) =>
    [...facts.querySelectorAll('[data-slot="property-row-label"]')].map(
      label => `${label.textContent}: ${label.nextElementSibling?.textContent}`
    );

  it("Should begin line two with the branch and link the PR beside, not inside, the open control (UT-070)", () => {
    const onOpen = vi.fn();
    renderUI(<SubagentCard subagent={isolated()} onOpen={onOpen} />);
    const open = screen.getByRole("button", { name: "Open Extract billing client" });
    const lineTwo = open.querySelector('[data-slot="subagent-card-line-two"]')!;
    const branch = within(lineTwo as HTMLElement).getByTitle("run/extract-billing-client-3f9a0c12");
    expect(lineTwo.firstElementChild).toBe(branch);
    expect(branch).toHaveTextContent("run/extract-billi…3f9a0c12");
    expect(lineTwo).toHaveTextContent("Moved the client into internal/billing/client");

    const link = screen.getByRole("link", { name: "Pull request #731, open" });
    expect(link).toHaveTextContent("#731");
    expect(link).toHaveAttribute("href", PR_URL);
    expect(link).toHaveAttribute("target", "_blank");
    expect(open).not.toContainElement(link);
    fireEvent.click(link);
    expect(onOpen).not.toHaveBeenCalled();
    fireEvent.click(open);
    expect(onOpen).toHaveBeenCalledOnce();
  });

  it("Should show branch, base, commits, PR and observed time in the hover (UT-071)", async () => {
    renderUI(<SubagentCard subagent={isolated()} onOpen={vi.fn()} />);
    expect(factText(await focusHover())).toEqual([
      "Worktree: extract-billing-client-3f9a0c12",
      "Branch: run/extract-billing-client-3f9a0c12",
      "Base: origin/main· 4be1c9d",
      "Commits: 3 ahead·clean",
      "PR: #731open",
      "Observed: 4m ago",
    ]);
  });

  it("Should show only static facts and no PR link while running (US-016.AC-1)", async () => {
    renderUI(
      <SubagentCard
        subagent={isolated(
          {
            head_sha: null,
            commits_ahead: null,
            dirty_files: null,
            observed_at: null,
            pull_request_status: null,
            pull_request: null,
          },
          "running"
        )}
        onOpen={vi.fn()}
      />
    );
    expect(screen.getByTitle("run/extract-billing-client-3f9a0c12")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    const facts = await focusHover();
    expect(factLabels(facts)).toEqual(["Worktree", "Branch", "Base"]);
  });

  it("Should show no link for an unknown PR and say PR status unknown (UT-072)", async () => {
    renderUI(
      <SubagentCard
        subagent={isolated({ pull_request_status: "unknown", pull_request: null, dirty_files: 2 })}
        onOpen={vi.fn()}
      />
    );
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    const facts = factText(await focusHover());
    expect(facts).toContain("PR: PR status unknown");
    expect(facts).toContain("Commits: 3 ahead·2 changed");
  });

  it("Should say No pull request when the forge answered none (UT-072)", async () => {
    renderUI(
      <SubagentCard
        subagent={isolated({ pull_request_status: "none", pull_request: null })}
        onOpen={vi.fn()}
      />
    );
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(factText(await focusHover())).toContain("PR: No pull request");
  });

  it("Should leave a shared subagent unchanged: no branch, no PR, no facts", async () => {
    renderUI(
      <SubagentCard
        subagent={subagent({ isolation: "shared", progress: "Reading webhook.go" })}
        onOpen={vi.fn()}
      />
    );
    expect(document.querySelector('[data-slot="subagent-branch"]')).toBeNull();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    const user = userEvent.setup();
    await user.tab();
    await waitFor(() =>
      expect(document.querySelector('[data-slot="subagent-hover"]')).not.toBeNull()
    );
    expect(document.querySelector('[data-slot="subagent-hover-facts"]')).toBeNull();
  });

  it("Should show the branch and PR link on isolated inspector rows only (UT-071)", () => {
    renderUI(
      <SessionInspectorSubagentsSection
        subagents={[
          { ...isolated(), id: "iso" },
          subagent({
            id: "shared",
            title: "Review retry config",
            status: "completed",
            settled_at: at(9),
          }),
        ]}
        onOpen={vi.fn()}
        defaultPreviousOpen
      />
    );
    const open = screen.getByRole("button", { name: "Open Extract billing client" });
    expect(within(open).getByTitle("run/extract-billing-client-3f9a0c12")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: "Pull request #731, open" });
    expect(open).not.toContainElement(link);
    const shared = screen.getByRole("button", { name: "Open Review retry config" });
    expect(shared.querySelector('[data-slot="subagent-branch"]')).toBeNull();
    expect(screen.getAllByRole("link")).toHaveLength(1);
  });
});
