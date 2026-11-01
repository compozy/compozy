import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TopbarSlotProvider, useTopbarSlotValue } from "@compozy/ui";

import type { UseAgentDetailResult } from "../use-agent-detail";
import { useAgentDetailTopbar } from "../use-agent-detail-topbar";

const detail = {
  agent: { label: "Issue Watcher" },
  onBackToAgent: vi.fn(),
  onBackToAgents: vi.fn(),
} as unknown as UseAgentDetailResult;

function PageBody() {
  useAgentDetailTopbar(detail, "issue-watcher", "Where it works", {
    actions: <button type="button">Add to a project</button>,
  });
  return null;
}

function PageShell() {
  useAgentDetailTopbar(detail, "issue-watcher", "Where it works", { publish: !detail.agent });
  return <PageBody />;
}

function TopbarActions() {
  return <>{useTopbarSlotValue()?.actions}</>;
}

function Harness() {
  return (
    <TopbarSlotProvider>
      <TopbarActions />
      <PageShell />
    </TopbarSlotProvider>
  );
}

describe("useAgentDetailTopbar", () => {
  it("Should keep the page's action when its shell renders again", () => {
    const { rerender } = render(<Harness />);
    expect(screen.getByRole("button", { name: "Add to a project" })).toBeInTheDocument();

    rerender(<Harness />);

    expect(screen.getByRole("button", { name: "Add to a project" })).toBeInTheDocument();
  });
});
