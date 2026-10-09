import { Profiler } from "react";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import { SessionInspectorSubagentsSection } from "../session-inspector-subagents-section";
import { SubagentCard } from "../subagent-card";
import { SubagentChip } from "../subagent-chip";
import { SubagentOriginDivider } from "../subagent-origin-divider";
import { SubagentWaitingBanner } from "../subagent-waiting-banner";
import type { SubagentStatus, SubagentView } from "../types";

// Suite: subagent presentational components.
// Invariant: rendered copy, roles and controls of the card, hover card, divider, banner, chip and
// inspector roster follow `_uiux.md` S1–S5, S9, S10; elapsed ticks from server timestamps on a
// shared ticker without re-rendering React. Owning layer: session subagent components (props in,
// callbacks out). Canonical suite: this file. Boundary OUT: sonner toast.
const T0 = Date.parse("2026-10-08T21:00:00.000Z");
const at = (seconds: number) => new Date(T0 + seconds * 1_000).toISOString();

function subagent(overrides: Partial<SubagentView> = {}): SubagentView {
  return {
    id: "sub-1",
    parent_session_id: "sess-parent",
    child_session_id: "sess-child",
    origin: "delegated",
    title: "Audit payment webhooks for retry safety",
    status: "running",
    progress: "",
    runtime: { provider: "claude", model: "opus-5.5", reasoning_effort: "high", speed: "normal" },
    started_at: at(0),
    settled_at: null,
    created_at: at(0),
    updated_at: at(0),
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

const renderUI = (ui: React.ReactNode) => render(<UIProvider>{ui}</UIProvider>);

beforeEach(() => {
  vi.mocked(toast.error).mockClear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("SubagentCard", () => {
  it("Should be a labeled button that opens the child, in a new window with ⌘ (UT-W05)", () => {
    const onOpen = vi.fn();
    renderUI(
      <SubagentCard subagent={subagent({ progress: "Reading webhook.go" })} onOpen={onOpen} />
    );
    const card = screen.getByRole("button", {
      name: "Open Audit payment webhooks for retry safety",
    });
    expect(card).toHaveAttribute("aria-description", "Running");
    expect(within(card).getByText("Reading webhook.go")).toBeInTheDocument();
    fireEvent.click(card, { metaKey: true });
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: "sub-1" }), {
      newWindow: true,
    });
  });

  it("Should render a provider-native card without a child as static, with no chevron (UT-W05)", () => {
    const { container } = renderUI(
      <SubagentCard
        subagent={subagent({ origin: "provider_native", child_session_id: null })}
        onOpen={vi.fn()}
      />
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(container.querySelector(".lucide-chevron-right")).toBeNull();
  });

  it("Should render the bot glyph for an unknown provider (UT-W05)", () => {
    const { container } = renderUI(
      <SubagentCard subagent={subagent({ runtime: { provider: "mystery" } })} />
    );
    expect(container.querySelector('[data-kind="mystery"] .lucide-bot')).not.toBeNull();
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

  it("Should open the hover card on focus with Not reported for a missing model (UT-W08)", async () => {
    const user = userEvent.setup();
    renderUI(
      <SubagentCard
        subagent={subagent({ runtime: { provider: "claude" }, progress: "Reading tickets" })}
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
    expect(screen.getByRole("status")).toHaveTextContent("Waiting on subagent Draft release notes");
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

  it("Should render nothing with no live delegated subagents", () => {
    const { container } = renderUI(<SubagentWaitingBanner subagents={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe("SubagentChip (UT-W17)", () => {
  it("Should label live/total and preview five rows then +N more with no other footer", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const rows = [...many(3, "running"), ...many(7, "completed")];
    renderUI(
      <SubagentChip
        counts={{ live: 3, total: 10, failed: 0, attention: 0 }}
        parentTurnRunning
        preview={rows}
        onOpen={onOpen}
      />
    );
    const chip = screen.getByRole("button", { name: "3 of 10 subagents running" });
    expect(chip).toHaveTextContent("3/10");
    await user.tab();
    await waitFor(() => expect(screen.getByText("+5 more")).toBeInTheDocument());
    expect(document.querySelectorAll('[data-slot="hover-card-content"] li')).toHaveLength(5);
    expect(screen.queryByText(/click/i)).not.toBeInTheDocument();
    fireEvent.click(chip);
    expect(onOpen).toHaveBeenCalledOnce();
  });

  it("Should render nothing once every subagent settled cleanly", () => {
    const { container } = renderUI(
      <SubagentChip
        counts={{ live: 0, total: 6, failed: 0, attention: 0 }}
        parentTurnRunning={false}
      />
    );
    expect(container).toBeEmptyDOMElement();
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
