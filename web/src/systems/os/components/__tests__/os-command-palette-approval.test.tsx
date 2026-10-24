// Suite: palette approval dialog
// Invariant: an approval whose status has not been read holds its note's place with a text bar
// under one busy region — never a "Loading approval…" sentence — and Approve waits; the status,
// once read, replaces the bar with its sentence and opens the decision. A failed read shows its
// error and neither a note nor a placeholder.
// Boundary IN: PaletteApprovalDialog over useCmdPaletteApproval.
// Boundary OUT: the approvals transport (adapters/cmd-palette-api).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CmdPaletteApprovalStatus } from "../../lib/cmd-palette-types";

vi.mock("../../adapters/cmd-palette-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/cmd-palette-api")>()),
  getPendingToolApproval: vi.fn(),
  resolvePendingToolApproval: vi.fn(),
}));

import { getPendingToolApproval } from "../../adapters/cmd-palette-api";
import { PaletteApprovalDialog } from "../os-command-palette";

function renderDialog() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <PaletteApprovalDialog
        approval={{ id: "apr_owner", profile: "editorial" }}
        description="Publishes the note to the workspace."
        destructive
        onOpenChange={vi.fn()}
        open
        title="Publish note"
      />
    </QueryClientProvider>
  );
}

describe("PaletteApprovalDialog", () => {
  let answer: (status: CmdPaletteApprovalStatus) => void;

  beforeEach(() => {
    vi.mocked(getPendingToolApproval).mockReset();
    vi.mocked(getPendingToolApproval).mockImplementation(
      () =>
        new Promise<CmdPaletteApprovalStatus>(resolve => {
          answer = resolve;
        })
    );
  });

  it("Should hold the note's place with a text bar until the approval's status is read", async () => {
    renderDialog();

    const dialog = await screen.findByRole("dialog");
    const note = within(dialog).getByRole("note");
    const loading = within(note).getByRole("status");
    expect(loading).toHaveAttribute("aria-busy", "true");
    expect(loading).toHaveTextContent("Loading approval");
    expect(loading.querySelector('[data-slot="skeleton-text"]')).not.toBeNull();
    expect(screen.queryByText("Loading approval…")).not.toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Approve" })).toBeDisabled();

    act(() => answer({ approval_status: "pending", expires_at: "2026-10-04T15:00:00Z" }));

    expect(await within(dialog).findByText("This command needs your approval.")).toBeVisible();
    expect(within(dialog).queryByRole("status")).not.toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Approve" })).toBeEnabled();
  });

  it("Should show a failed read's error with no note and no placeholder", async () => {
    vi.mocked(getPendingToolApproval).mockRejectedValue(new Error("Approval not found"));
    renderDialog();

    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByText("Approval not found")).toBeVisible();
    expect(within(dialog).queryByRole("status")).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("note")).not.toBeInTheDocument();
  });
});
