import { UIProvider } from "@agh/ui";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { KnowledgeMemoryItem, MemoryDecision, MemoryHeader } from "../../types";

import { KnowledgeDetailPanel } from "../knowledge-detail-panel";

const MEMORY: MemoryHeader = {
  filename: "user-role.md",
  mod_time: "2026-04-09T10:00:00Z",
  name: "User Role",
  scope: "global",
  type: "user",
  recall_count: 4,
  injection: true,
  system_managed: false,
  description: "Guidance for the assistant.",
  last_recalled_at: "2026-04-09T11:00:00Z",
};

const AGENT_MEMORY: MemoryHeader = {
  filename: "cto-tone.md",
  mod_time: "2026-04-09T10:00:00Z",
  name: "CTO Tone",
  scope: "agent",
  agent_name: "cto",
  agent_tier: "workspace",
  workspace_id: "ws_launch",
  type: "user",
  recall_count: 6,
  injection: true,
  system_managed: false,
  staleness_banner: "Updated >7 days after last recall",
  superseded_by: "cto-tone-v2.md",
};

const SAMPLE_DECISION: MemoryDecision = {
  id: "dec_001",
  candidate_hash: "h",
  op: "update",
  scope: "global",
  source: "rule",
  confidence: 0.93,
  decided_at: "2026-04-09T11:00:00Z",
  applied_at: "2026-04-09T11:00:01Z",
  target_filename: "user-role.md",
  reason: "rule:exact-slug-collision",
  frontmatter: {
    filename: "user-role.md",
    mod_time: "2026-04-09T10:00:00Z",
    name: "User Role",
    type: "user",
  },
};

function renderDetail(props: Partial<React.ComponentProps<typeof KnowledgeDetailPanel>> = {}) {
  const merged: React.ComponentProps<typeof KnowledgeDetailPanel> = {
    memory: MEMORY,
    content: "# User Role\n\nBody content.",
    scope: "global",
    status: {
      isDecisionsLoading: false,
      isDeletePending: false,
      isLoading: false,
    },
    error: null,
    onDelete: vi.fn(),
    decisions: [],
    decisionsError: null,
    ...props,
  };
  return render(
    <UIProvider reducedMotion="always">
      <KnowledgeDetailPanel {...merged} />
    </UIProvider>
  );
}

describe("KnowledgeDetailPanel", () => {
  it("Should render the empty state when no memory is selected", () => {
    renderDetail({ memory: undefined, content: undefined });
    const empty = screen.getByTestId("knowledge-detail-empty");
    expect(empty).toBeInTheDocument();
    expect(
      within(empty).getByText("Select a memory to view details", { selector: "h3" })
    ).toBeInTheDocument();
  });

  it("Should render the loading spinner when isLoading is true", () => {
    renderDetail({
      status: { isDecisionsLoading: false, isDeletePending: false, isLoading: true },
      content: undefined,
    });
    expect(screen.getByTestId("knowledge-detail-loading")).toBeInTheDocument();
  });

  it("Should render the error fallback when error is set", () => {
    renderDetail({ error: new Error("Boom"), content: undefined });
    expect(screen.getByTestId("knowledge-detail-error")).toBeInTheDocument();
    expect(screen.getByText("Boom")).toBeInTheDocument();
  });

  it("Should render the markdown preview inside the CodeBlock primitive", () => {
    renderDetail();
    const preview = screen.getByTestId("content-preview");
    expect(preview).toHaveAttribute("data-slot", "code-block");
  });

  it("Should render only scope + age pills in the detail header (no type/tier/staleness)", () => {
    renderDetail();
    const header = screen.getByTestId("knowledge-detail-header");
    expect(header.querySelector("[data-slot='page-head']")).toBeNull();
    expect(within(header).getByTestId("detail-scope-badge")).toHaveTextContent("Global");
    expect(within(header).getByTestId("detail-age-badge")).toBeInTheDocument();
    // The header must NOT carry type/tier/staleness pills.
    expect(within(header).queryByTestId("detail-type-badge")).toBeNull();
    expect(within(header).queryByTestId("detail-agent-tier-badge")).toBeNull();
    expect(within(header).queryByTestId("detail-staleness-badge")).toBeNull();
  });

  it("Should surface filename under the unified window head", () => {
    renderDetail();
    expect(screen.getByTestId("knowledge-detail-filename")).toHaveTextContent("user-role.md");
  });

  it("Should embed an Overview ContextBox carrying type/tier/staleness", () => {
    renderDetail({
      memory: AGENT_MEMORY,
      content: "agent body",
      scope: "agent",
    });
    const context = screen.getByTestId("knowledge-detail-context");
    expect(context).toBeInTheDocument();
    expect(screen.getByTestId("context-type-value")).toHaveTextContent("user");
    expect(screen.getByTestId("context-tier-value")).toHaveTextContent("notes");
    expect(screen.getByTestId("context-staleness-value")).toHaveTextContent(
      "Updated >7 days after last recall"
    );
    expect(screen.getByTestId("context-agent-tier-value")).toHaveTextContent("Agent · workspace");
    expect(screen.getByTestId("context-agent-value")).toHaveTextContent("cto");
    expect(screen.getByTestId("context-workspace-value")).toHaveTextContent("ws_launch");
    expect(screen.getByTestId("context-superseded-value")).toHaveTextContent("cto-tone-v2.md");
    expect(screen.getByTestId("context-recalls-value")).toHaveTextContent("6");
  });

  it("Should resolve modified + last-recalled via the <Time> primitive", () => {
    renderDetail();
    const modified = screen.getByTestId("context-modified-value");
    expect(modified.tagName.toLowerCase()).toBe("time");
    expect(modified.getAttribute("datetime")).toBe(MEMORY.mod_time);
    const lastRecalled = screen.getByTestId("context-last-recalled-value");
    expect(lastRecalled.tagName.toLowerCase()).toBe("time");
    expect(lastRecalled.getAttribute("datetime")).toBe(MEMORY.last_recalled_at);
  });

  it("Should default staleness to Active when the banner is absent", () => {
    renderDetail();
    expect(screen.getByTestId("context-staleness-value")).toHaveTextContent("Active");
  });

  it("Should open the delete dialog and emit onDelete when confirmed", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn().mockResolvedValue(undefined);
    renderDetail({ onDelete });

    await user.click(screen.getByTestId("delete-memory-btn"));
    expect(screen.getByTestId("knowledge-delete-dialog")).toBeInTheDocument();

    await user.type(screen.getByTestId("knowledge-delete-confirm-typing"), MEMORY.filename);
    await user.click(screen.getByTestId("confirm-delete-memory-btn"));
    expect(onDelete).toHaveBeenCalledWith(MEMORY);
  });

  it("Should reset dialog state when switching between memories that share scope and filename", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn().mockResolvedValue(undefined);
    const firstMemory: KnowledgeMemoryItem = {
      ...MEMORY,
      filename: "shared.md",
      key: undefined,
      name: "Shared Alpha",
      scope: "workspace",
      workspace_id: "ws_alpha",
    };
    const secondMemory: KnowledgeMemoryItem = {
      ...MEMORY,
      filename: "shared.md",
      key: undefined,
      name: "Shared Beta",
      scope: "workspace",
      workspace_id: "ws_beta",
    };
    const status = {
      isDecisionsLoading: false,
      isDeletePending: false,
      isLoading: false,
    };
    const { rerender } = render(
      <UIProvider reducedMotion="always">
        <KnowledgeDetailPanel
          content="# Shared Alpha"
          decisions={[]}
          decisionsError={null}
          error={null}
          memory={firstMemory}
          onDelete={onDelete}
          scope="workspace"
          status={status}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("delete-memory-btn"));
    expect(screen.getByTestId("knowledge-delete-dialog")).toBeInTheDocument();

    rerender(
      <UIProvider reducedMotion="always">
        <KnowledgeDetailPanel
          content="# Shared Beta"
          decisions={[]}
          decisionsError={null}
          error={null}
          memory={secondMemory}
          onDelete={onDelete}
          scope="workspace"
          status={status}
        />
      </UIProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId("knowledge-delete-dialog")).toHaveAttribute("data-closed");
    });

    await user.click(screen.getByTestId("delete-memory-btn"));
    await user.type(screen.getByTestId("knowledge-delete-confirm-typing"), secondMemory.filename);
    await user.click(screen.getByTestId("confirm-delete-memory-btn"));

    expect(onDelete).toHaveBeenCalledWith(secondMemory);
  });

  it("Should disable the delete button while a delete is pending", () => {
    renderDetail({
      status: { isDecisionsLoading: false, isDeletePending: true, isLoading: false },
    });
    expect(screen.getByTestId("delete-memory-btn")).toBeDisabled();
  });

  it("Should surface delete failures inline and inside the delete dialog", async () => {
    const user = userEvent.setup();
    renderDetail({ deleteError: "Delete failed" });

    expect(screen.getByTestId("knowledge-delete-error")).toHaveTextContent("Delete failed");
    await user.click(screen.getByTestId("delete-memory-btn"));
    expect(screen.getByTestId("knowledge-delete-dialog-error")).toHaveTextContent("Delete failed");
  });

  it("Should hide the edit button when no edit handler is provided", () => {
    renderDetail();
    expect(screen.queryByTestId("edit-memory-btn")).not.toBeInTheDocument();
  });

  it("Should open the edit dialog and submit the new content via onEdit", async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn().mockResolvedValue(undefined);
    renderDetail({ onEdit });

    await user.click(screen.getByTestId("edit-memory-btn"));
    const contentArea = screen.getByTestId("knowledge-edit-content");
    await user.type(contentArea, "\nMore body");

    await user.click(screen.getByTestId("confirm-edit-memory-btn"));
    expect(onEdit).toHaveBeenCalledWith(MEMORY, {
      content: "# User Role\n\nBody content.\nMore body",
      description: "Guidance for the assistant.",
    });
  });

  it("Should disable the edit button while content is unavailable", () => {
    renderDetail({ onEdit: vi.fn(), content: undefined });
    expect(screen.getByTestId("edit-memory-btn")).toBeDisabled();
  });

  it("Should surface edit failures inline and inside the edit dialog", async () => {
    const user = userEvent.setup();
    renderDetail({ onEdit: vi.fn(), editError: "Edit failed" });

    expect(screen.getByTestId("knowledge-edit-error")).toHaveTextContent("Edit failed");
    await user.click(screen.getByTestId("edit-memory-btn"));
    expect(screen.getByTestId("knowledge-edit-dialog-error")).toHaveTextContent("Edit failed");
  });

  it("Should render the controller decisions section when decisions are present", () => {
    renderDetail({ decisions: [SAMPLE_DECISION] });
    expect(screen.getByTestId("knowledge-decisions-list")).toBeInTheDocument();
    expect(screen.getByTestId(`knowledge-decision-${SAMPLE_DECISION.id}`)).toBeInTheDocument();
  });

  it("Should render the empty decisions fallback when there are no decisions", () => {
    renderDetail();
    expect(screen.getByTestId("knowledge-decisions-empty")).toBeInTheDocument();
  });

  it("Should render the decisions error fallback when the decisions query fails", () => {
    renderDetail({ decisionsError: new Error("Decisions failed") });
    expect(screen.getByTestId("knowledge-decisions-error")).toBeInTheDocument();
    expect(screen.getByText("Decisions failed")).toBeInTheDocument();
  });

  it("Should render the decisions loading state while decisions are loading", () => {
    renderDetail({
      status: { isDecisionsLoading: true, isDeletePending: false, isLoading: false },
    });
    expect(screen.getByTestId("knowledge-decisions-loading")).toBeInTheDocument();
  });
});
