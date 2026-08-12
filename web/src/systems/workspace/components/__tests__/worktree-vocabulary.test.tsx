// Suite: worktree row vocabulary truthfulness
// Invariant (UT-129): the chip renders only when state is not ready; unknown facts render as absent
// rather than zero or "clean"; a detached HEAD states its pin instead of inventing a branch; and
// remote-derived values are stamped stale. Chip, dot, and signals share this one invariant family,
// so they share one suite instead of duplicating it three times.
// Owning layer: workspace domain components. Canonical suite: this component test.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { WorktreeDetachedPin } from "../worktree-signal-parts";
import { WorktreeSignals } from "../worktree-signals";
import { WorktreeStateChip } from "../worktree-state-chip";
import { WorktreeStateDot } from "../worktree-state-dot";
import { toWorktreeDisplayState } from "../../lib/worktree-display";

function renderSignals(props: Partial<React.ComponentProps<typeof WorktreeSignals>> = {}) {
  return render(
    <WorktreeSignals
      dirty={null}
      ahead={null}
      behind={null}
      agentActivity="idle"
      origin="manual"
      setupState="none"
      {...props}
    />
  );
}

describe("WorktreeStateChip", () => {
  it("Should treat an unknown wire state as an error instead of ready", () => {
    expect(toWorktreeDisplayState({ state: "future-state" })).toBe("error");
  });

  it("Should render nothing for a ready worktree", () => {
    const { container } = render(<WorktreeStateChip state="ready" />);

    // Ready is calm: the quiet success dot is the whole signal.
    expect(container).toBeEmptyDOMElement();
  });

  it.each(["pending", "discovered", "missing", "error", "failed"] as const)(
    "Should render the %s chip with its own label",
    state => {
      render(<WorktreeStateChip state={state} />);

      expect(screen.getByText(state)).toBeVisible();
    }
  );

  it("Should keep error and failed as distinct states", () => {
    const { rerender } = render(<WorktreeStateChip state="error" />);
    expect(screen.getByText("error")).toBeVisible();

    rerender(<WorktreeStateChip state="failed" />);
    // A rolled-back creation is not an unreadable status (TechSpec N-004).
    expect(screen.queryByText("error")).toBeNull();
    expect(screen.getByText("failed")).toBeVisible();
  });
});

describe("WorktreeStateDot", () => {
  it("Should give pending a different shape from discovered and missing", () => {
    const { container: pending } = render(<WorktreeStateDot state="pending" />);
    const { container: discovered } = render(<WorktreeStateDot state="discovered" />);

    const pendingClass = pending.firstElementChild?.className ?? "";
    const discoveredClass = discovered.firstElementChild?.className ?? "";
    // Shape carries state alongside colour, so the two never collapse into one
    // glyph for a colour-blind read.
    expect(pendingClass).not.toBe(discoveredClass);
    expect(pendingClass).toContain("rotate-45");
  });
});

describe("WorktreeSignals", () => {
  it("Should render nothing for a clean worktree with unknown counts", () => {
    renderSignals({ dirty: null });

    expect(screen.queryByText(/\+0/)).toBeNull();
    expect(screen.queryByText(/−0/)).toBeNull();
    expect(screen.queryByText(/clean/i)).toBeNull();
  });

  it("Should never render +0 −0 when a dirty worktree has no known counts", () => {
    renderSignals({ dirty: true });

    expect(screen.queryByText("+0")).toBeNull();
    expect(screen.queryByText("−0")).toBeNull();
    // Honest about the magnitude being unknown rather than inventing zeroes.
    expect(screen.getByText("uncommitted")).toBeVisible();
  });

  it("Should render real insertion and deletion counts when they are known", () => {
    renderSignals({ dirty: true, insertions: 142, deletions: 18, changedFiles: 6 });

    expect(screen.getByText("+142")).toBeVisible();
    expect(screen.getByText("−18")).toBeVisible();
    expect(screen.getByText("· 6 files")).toBeVisible();
  });

  it("Should hide ahead and behind when the counts are unknown", () => {
    renderSignals({ ahead: null, behind: null });

    expect(screen.queryByText(/ahead by/)).toBeNull();
    expect(screen.queryByText(/behind by/)).toBeNull();
  });

  it("Should hide ahead and behind when both are zero rather than showing 0", () => {
    renderSignals({ ahead: 0, behind: 0 });

    expect(screen.queryByText("0")).toBeNull();
  });

  it("Should stamp remote-derived values as of their read time", () => {
    renderSignals({ ahead: 1, staleLabel: "18m ago" });

    // Freshness is never implied for a remote value.
    expect(screen.getByText("as of 18m ago")).toBeVisible();
  });

  it("Should render nothing for an idle agent", () => {
    renderSignals({ agentActivity: "idle" });

    expect(screen.queryByText(/agent running/)).toBeNull();
    expect(screen.queryByText(/awaiting input/)).toBeNull();
  });

  it("Should distinguish a running agent from one awaiting input", () => {
    const { rerender } = renderSignals({ agentActivity: "running" });
    expect(screen.getByText("agent running")).toBeInTheDocument();

    rerender(
      <WorktreeSignals
        dirty={null}
        ahead={null}
        behind={null}
        agentActivity="awaiting_input"
        origin="manual"
        setupState="none"
      />
    );
    expect(screen.getByText("awaiting input")).toBeInTheDocument();
  });

  it("Should flag a failed bootstrap while stating the worktree is still usable", () => {
    renderSignals({ setupState: "failed", setupError: "bun install exited 1" });

    expect(screen.getByText("setup failed")).toBeVisible();
    expect(screen.getByTitle(/worktree is usable/)).toBeInTheDocument();
  });

  it("Should hold nested rows to a single trailing signal", () => {
    const { container } = renderSignals({
      density: "nest",
      dirty: true,
      ahead: 3,
      agentActivity: "running",
    });

    // The nest density budget is one signal; the name must survive a busy row.
    const rendered = container.querySelectorAll("[data-slot^='worktree-']");
    const budgeted = [...rendered].filter(node =>
      ["worktree-agent", "worktree-dirty", "worktree-ahead-behind", "worktree-merged"].includes(
        node.getAttribute("data-slot") ?? ""
      )
    );
    expect(budgeted).toHaveLength(1);
  });
});

describe("WorktreeDetachedPin", () => {
  it("Should state the pinned sha instead of inventing a branch name", () => {
    render(<WorktreeDetachedPin sha="9ec3d15aa1b2c3" />);

    // US-012 EC-1: a detached HEAD has no branch, so none is displayed.
    expect(screen.getByText("pinned at 9ec3d15")).toBeVisible();
  });

  it("Should render nothing without a sha", () => {
    const { container } = render(<WorktreeDetachedPin sha="   " />);

    expect(container).toBeEmptyDOMElement();
  });
});
