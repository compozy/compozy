import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import {
  sessionMCPAllReadyFixture,
  sessionMCPNeedsSignInFixture,
  sessionMCPSummaryFixture,
} from "../../mocks/session-mcp-fixtures";
import { MCPComposerControl } from "../mcp-composer-control";

// Suite: composer MCP control (spec mcp-extension-hot-activation, `_uiux.md` S3).
// Invariant: the control appears only for a session whose payload carries an MCP summary; its
// popover lists what the session can use, offers the one repair action a row needs, and never
// calls a server ready without a status read to say so.
// Owning layer: session presentational components (props in, callbacks out). Canonical suite: this file.
function renderControl(props: Partial<Parameters<typeof MCPComposerControl>[0]> = {}) {
  return render(
    <UIProvider>
      <MCPComposerControl
        summary={sessionMCPSummaryFixture(sessionMCPAllReadyFixture)}
        data={null}
        {...props}
      />
    </UIProvider>
  );
}

describe("MCPComposerControl", () => {
  it("UT-W08: Should render nothing for a session without an MCP summary", () => {
    renderControl({ summary: null, data: sessionMCPAllReadyFixture });
    expect(screen.queryByTestId("composer-mcp-button")).not.toBeInTheDocument();
  });

  it("Should put rows that need the operator first and hand their action to the owner", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    const onOpenChange = vi.fn();
    renderControl({
      summary: sessionMCPSummaryFixture(sessionMCPNeedsSignInFixture),
      data: sessionMCPNeedsSignInFixture,
      onAction,
      onOpenChange,
    });

    const trigger = screen.getByRole("button", { name: "MCP servers: 3 ready, 1 need you" });
    expect(trigger).toHaveTextContent("MCP3");
    await user.click(trigger);
    expect(onOpenChange).toHaveBeenLastCalledWith(true);

    const rows = screen.getAllByTestId(/^composer-mcp-(notion|linear|github-local|grafana)$/);
    expect(rows[0]).toHaveAttribute("data-testid", "composer-mcp-notion");
    const notion = screen.getByTestId("composer-mcp-notion");
    expect(notion).toHaveTextContent("Needs sign-in");
    await user.click(within(notion).getByRole("button", { name: "Sign in" }));
    expect(onAction).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ name: "notion", state: "needs_sign_in" }),
      "sign_in"
    );
    expect(screen.getByTestId("composer-mcp-context7")).toHaveTextContent("Loaded by Claude Code");
  });

  it("UT-W09: Should keep the last rows as out of date when a reread fails", async () => {
    const user = userEvent.setup();
    renderControl({ data: sessionMCPAllReadyFixture, failed: true });

    await user.click(screen.getByTestId("composer-mcp-button"));

    expect(screen.getByText("Status may be out of date")).toBeInTheDocument();
    expect(screen.getByTestId("composer-mcp-linear")).toHaveTextContent("Ready");
  });

  it("UT-W09: Should call nothing ready when the first status read fails", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderControl({ data: null, failed: true, onRetry });

    await user.click(screen.getByTestId("composer-mcp-button"));

    expect(screen.getByText("Couldn't load MCP servers.")).toBeInTheDocument();
    expect(screen.queryByText("Ready")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });
});
