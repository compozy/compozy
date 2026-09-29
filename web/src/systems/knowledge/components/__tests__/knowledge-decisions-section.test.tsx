import { UIProvider } from "@compozy/ui";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { MemoryDecision } from "../../types";

import { KnowledgeDecisionsSection } from "../knowledge-decisions-section";

const SAMPLE: MemoryDecision = {
  id: "dec_alpha",
  candidate_hash: "h",
  op: "update",
  scope: "profile",
  source: "rule",
  confidence: 0.91,
  decided_at: "2026-04-09T10:00:00Z",
  applied_at: "2026-04-09T10:00:01Z",
  target_filename: "user.md",
  reason: "rule:exact-slug-collision",
  frontmatter: {
    filename: "user.md",
    mod_time: "2026-04-09T10:00:00Z",
    name: "User",
    type: "user",
  },
};

function renderSection(
  props: Partial<React.ComponentProps<typeof KnowledgeDecisionsSection>> = {}
) {
  const merged: React.ComponentProps<typeof KnowledgeDecisionsSection> = {
    decisions: [],
    isLoading: false,
    error: null,
    ...props,
  };
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      <KnowledgeDecisionsSection {...merged} />
    </UIProvider>
  );
}

describe("KnowledgeDecisionsSection", () => {
  it("Should render the loading fallback when isLoading is true", () => {
    renderSection({ isLoading: true });
    expect(screen.getByTestId("knowledge-decisions-loading")).toBeInTheDocument();
  });

  it("Should render the error fallback when error is set", () => {
    renderSection({ error: new Error("Decisions failed") });
    expect(screen.getByTestId("knowledge-decisions-error")).toBeInTheDocument();
    expect(screen.getByText("Decisions failed")).toBeInTheDocument();
  });

  it("Should render nothing when there are no decisions", () => {
    renderSection();
    expect(screen.queryByTestId("knowledge-decisions-section")).not.toBeInTheDocument();
  });

  it("Should keep the history fold closed by default and open it on toggle", async () => {
    const user = userEvent.setup();
    renderSection({ decisions: [SAMPLE] });
    const toggle = screen.getByTestId("knowledge-decisions-toggle");
    expect(toggle).toHaveTextContent("History");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
  });

  it("Should render decisions as <TimelineEvent> rows with plain op/source labels", () => {
    renderSection({ decisions: [SAMPLE] });
    const list = screen.getByTestId("knowledge-decisions-list");
    expect(list).toBeInTheDocument();
    // Dense table rows are represented as TimelineEvent rows; no <table>.
    expect(list.tagName.toLowerCase()).toBe("ul");
    expect(list.querySelector("table")).toBeNull();
    const row = screen.getByTestId(`knowledge-decision-${SAMPLE.id}`);
    expect(row).toHaveAttribute("data-slot", "timeline-event");
    expect(screen.getByTestId(`knowledge-decision-op-${SAMPLE.id}`)).toHaveTextContent("Updated");
    expect(screen.getByTestId(`knowledge-decision-source-${SAMPLE.id}`)).toHaveTextContent("Rule");
    expect(screen.getByTestId(`knowledge-decision-confidence-${SAMPLE.id}`)).toHaveTextContent(
      "91% confident"
    );
    expect(screen.getByTestId(`knowledge-decision-applied-${SAMPLE.id}`)).toBeInTheDocument();
    expect(screen.getByTestId(`knowledge-decision-target-${SAMPLE.id}`)).toHaveTextContent(
      /user\.md/
    );
  });

  it("Should render decided_at via <Time>", () => {
    renderSection({ decisions: [SAMPLE] });
    const time = screen.getByTestId(`knowledge-decision-time-${SAMPLE.id}`);
    expect(time.tagName.toLowerCase()).toBe("time");
    expect(time.getAttribute("datetime")).toBe(SAMPLE.decided_at);
  });

  it("Should render a not-applied chip when applied_at is missing", () => {
    renderSection({
      decisions: [
        {
          ...SAMPLE,
          id: "dec_pending",
          applied_at: null,
        },
      ],
    });
    expect(screen.getByTestId("knowledge-decision-pending-dec_pending")).toHaveTextContent(
      /Not applied/
    );
  });
});
