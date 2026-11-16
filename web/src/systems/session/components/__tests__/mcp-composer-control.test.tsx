import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { mcpPanelAnchor } from "@/systems/settings";

import {
  sessionMCPAllReadyFixture,
  sessionMCPFailedFixture,
  sessionMCPFixture,
  sessionMCPLimitedFixture,
  sessionMCPNeedsSignInFixture,
  sessionMCPPendingFixture,
  sessionMCPReloadFailedFixture,
  sessionMCPRuntimeOffFixture,
  sessionMCPSummaryFixture,
  sessionMCPWaitingFixture,
} from "../../mocks/session-mcp-fixtures";
import type { SessionMCPActions } from "../../hooks/use-session-mcp-actions";
import { MCPComposerControl } from "../mcp-composer-control";
import { SessionComposerMCPPanels } from "../session-composer-mcp-panels";

// Suite: composer MCP control (spec mcp-extension-hot-activation, `_uiux.md` S3).
// Invariant: the control appears only for a session whose payload carries an MCP summary; its
// popover lists what the session can use in one vocabulary (N tools · Sign in · Review · Not
// working · Off), offers the one action a row needs with the button its panel opens beside, keeps
// the coding agent's own servers folded, and never calls a server ready without a status read.
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
    // Icon-only: the counts live in the name, and the attention dot stands in for the old warning tint.
    expect(trigger.textContent).toBe("");
    expect(screen.getByTestId("composer-mcp-attention")).toBeInTheDocument();
    expect(screen.queryByTestId("composer-mcp-pending")).not.toBeInTheDocument();
    await user.click(trigger);
    expect(onOpenChange).toHaveBeenLastCalledWith(true);

    const rows = screen.getAllByTestId(/^composer-mcp-(notion|linear|github-local|grafana)$/);
    expect(rows[0]).toHaveAttribute("data-testid", "composer-mcp-notion");
    const notion = screen.getByTestId("composer-mcp-notion");
    const signIn = within(notion).getByRole("button", { name: "Sign in notion" });
    expect(screen.getByTestId("composer-mcp-linear")).toHaveTextContent("23 tools");
    await user.click(signIn);
    // The pressed button travels with the action: the sign-in panel opens beside it.
    expect(onAction).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ name: "notion", state: "needs_sign_in" }),
      "sign_in",
      signIn
    );
  });

  it("Should offer Review for a waiting project server and Not working for a failed one", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    const data = sessionMCPFixture({
      compozy: [
        ...sessionMCPWaitingFixture.compozy_servers.slice(0, 1),
        ...sessionMCPFailedFixture.compozy_servers,
      ],
    });
    renderControl({ summary: sessionMCPSummaryFixture(data), data, onAction, defaultOpen: true });

    const review = within(screen.getByTestId("composer-mcp-sentry")).getByRole("button", {
      name: "Review sentry",
    });
    await user.click(review);
    expect(onAction).toHaveBeenLastCalledWith(
      expect.objectContaining({ name: "sentry" }),
      "allow",
      review
    );
    const failed = within(screen.getByTestId("composer-mcp-pagerduty")).getByRole("button", {
      name: "pagerduty is not working",
    });
    expect(failed).toHaveTextContent("Not working");
    await user.click(failed);
    expect(onAction).toHaveBeenLastCalledWith(
      expect.objectContaining({ name: "pagerduty" }),
      "check",
      failed
    );
  });

  it("Should fold the coding agent's own servers and the parent's limits behind closed rows", async () => {
    const user = userEvent.setup();
    const onManage = vi.fn();
    renderControl({
      summary: sessionMCPSummaryFixture(sessionMCPLimitedFixture),
      data: sessionMCPLimitedFixture,
      defaultOpen: true,
      onManage,
    });

    expect(screen.queryByTestId("composer-mcp-context7")).not.toBeInTheDocument();
    const runtime = screen.getByTestId("composer-mcp-runtime-toggle");
    expect(runtime).toHaveTextContent("Claude Code's own servers");
    expect(runtime).toHaveAttribute("aria-expanded", "false");
    await user.click(runtime);
    expect(await screen.findByTestId("composer-mcp-context7")).toBeInTheDocument();
    expect(
      screen.getByText("Claude Code loads these by itself. Change them in Claude Code.")
    ).toBeInTheDocument();

    expect(screen.queryByTestId("composer-mcp-grafana")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Off in this session (2)" }));
    expect(await screen.findByTestId("composer-mcp-grafana")).toHaveTextContent("Off");

    await user.click(screen.getByRole("button", { name: "Manage MCP servers in Settings" }));
    expect(onManage).toHaveBeenCalledOnce();
  });

  it("Should say the coding agent's own servers are turned off for CompozyOS sessions", async () => {
    const user = userEvent.setup();
    renderControl({ data: sessionMCPRuntimeOffFixture, defaultOpen: true });

    const runtime = screen.getByTestId("composer-mcp-runtime-toggle");
    expect(runtime).toHaveTextContent("Off");
    await user.click(runtime);
    expect(
      await screen.findByText("Turned off for CompozyOS sessions. Turn them on in Settings.")
    ).toBeInTheDocument();
  });

  it("Should keep a pending change visible and named while a server also needs the operator", () => {
    renderControl({
      summary: { ...sessionMCPSummaryFixture(sessionMCPNeedsSignInFixture), pending_reload: true },
    });

    expect(
      screen.getByRole("button", {
        name: "MCP servers: 3 ready, 1 need you, changes apply on your next message",
      })
    ).toBeInTheDocument();
    expect(screen.getByTestId("composer-mcp-attention")).toBeInTheDocument();
    expect(screen.getByTestId("composer-mcp-pending")).toBeInTheDocument();
  });

  it("Should name a failed reload and the change still waiting for the next message", () => {
    renderControl({
      summary: sessionMCPSummaryFixture(sessionMCPReloadFailedFixture),
      data: sessionMCPReloadFailedFixture,
    });

    expect(
      screen.getByRole("button", {
        name: /, couldn't start with the new servers, changes apply on your next message$/,
      })
    ).toBeInTheDocument();
    expect(screen.getByTestId("composer-mcp-attention")).toBeInTheDocument();
    expect(screen.getByTestId("composer-mcp-pending")).toBeInTheDocument();
  });

  it("Should promise the next message only while a pending change has not failed", () => {
    const { unmount } = renderControl({
      summary: sessionMCPSummaryFixture(sessionMCPPendingFixture),
      data: sessionMCPPendingFixture,
      defaultOpen: true,
    });
    expect(screen.getByText("Updates on your next message")).toBeInTheDocument();
    expect(screen.getByTestId("composer-mcp-sentry")).toHaveTextContent("Next message");
    unmount();

    renderControl({
      summary: sessionMCPSummaryFixture(sessionMCPReloadFailedFixture),
      data: sessionMCPReloadFailedFixture,
      defaultOpen: true,
    });
    expect(screen.queryByText("Updates on your next message")).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Claude Code couldn't start with the new servers. Your message wasn't sent"
    );
  });

  it("Should clear a failed reload from the closed trigger once the change has applied", () => {
    // A later reload applied: the summary has nothing pending, while the read cached before it
    // still records the failure.
    renderControl({
      summary: {
        ...sessionMCPSummaryFixture(sessionMCPReloadFailedFixture),
        pending_reload: false,
      },
      data: sessionMCPReloadFailedFixture,
    });

    expect(screen.getByRole("button", { name: "MCP servers: 3 ready" })).toBeInTheDocument();
    expect(screen.queryByTestId("composer-mcp-attention")).not.toBeInTheDocument();
    expect(screen.queryByTestId("composer-mcp-pending")).not.toBeInTheDocument();
  });

  it("UT-W09: Should keep the last rows as out of date when a reread fails", async () => {
    const user = userEvent.setup();
    renderControl({ data: sessionMCPAllReadyFixture, failed: true });

    await user.click(screen.getByTestId("composer-mcp-button"));

    expect(screen.getByText("May be out of date")).toBeInTheDocument();
    expect(screen.getByTestId("composer-mcp-linear")).toHaveTextContent("23 tools");
  });

  it("UT-W09: Should call nothing ready when the first status read fails", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderControl({ data: null, failed: true, onRetry });

    await user.click(screen.getByTestId("composer-mcp-button"));

    expect(screen.getByText("Couldn't load MCP servers.")).toBeInTheDocument();
    expect(screen.queryByText(/tools?$/)).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });
});

// Suite: the composer's row panels (`SessionComposerMCPPanels`), fed a stubbed actions hook.
// Invariant: each row state opens its own panel beside its row — sign-in for a server that needs
// it, the allow review for a waiting project server, and why it is not working with Check again
// — and Check again acts on that server's full identity.
function panelActions(overrides: Partial<SessionMCPActions> = {}): SessionMCPActions {
  return {
    panel: null,
    panelAllowing: false,
    panelChecking: false,
    panelStillFailing: false,
    allow: vi.fn(),
    check: vi.fn(),
    closePanel: vi.fn(),
    authorization: {
      authorize: { phase: "idle", anchor: null, cancel: vi.fn() },
      server: null,
    },
    ...overrides,
  } as unknown as SessionMCPActions;
}

function rowButton(): HTMLElement {
  const row = document.createElement("li");
  const button = document.createElement("button");
  row.append(button);
  document.body.append(row);
  return button;
}

function renderPanels(actions: SessionMCPActions) {
  return render(
    <UIProvider>
      <SessionComposerMCPPanels actions={actions} onOpenSettings={vi.fn()} />
    </UIProvider>
  );
}

describe("SessionComposerMCPPanels", () => {
  it("Should open the sign-in panel beside a row that needs sign-in", () => {
    const row = sessionMCPNeedsSignInFixture.compozy_servers[0]!;
    renderPanels(
      panelActions({
        authorization: {
          authorize: {
            phase: "waiting",
            server: row.name,
            begin: { authorization_url: "https://auth.notion.so/authorize" },
            anchor: mcpPanelAnchor(rowButton()),
            mode: "automatic",
            approvedScopes: [],
            error: null,
            cancel: vi.fn(),
          },
          server: null,
        },
      } as unknown as Partial<SessionMCPActions>)
    );

    const panel = screen.getByRole("dialog", { name: `Sign in to ${row.name}` });
    expect(within(panel).getByTestId("mcp-sign-in-panel")).toHaveAttribute("data-phase", "waiting");
  });

  it("Should review a waiting project server with what it runs before Allow", () => {
    const row = sessionMCPWaitingFixture.compozy_servers[0]!;
    renderPanels(
      panelActions({
        panel: {
          kind: "allow",
          target: { name: row.name, owner: row.owner },
          row,
          anchor: mcpPanelAnchor(rowButton()),
        },
      })
    );

    const panel = screen.getByRole("dialog", { name: `Allow ${row.name}?` });
    expect(within(panel).getByTestId("mcp-allow-panel-launch")).toHaveTextContent(
      "npx -y @sentry/mcp-server"
    );
  });

  it("Should say a failing server still fails and check it again by name and owner", async () => {
    const user = userEvent.setup();
    const row = { ...sessionMCPFailedFixture.compozy_servers[0]!, owner: "extension:ops" };
    const check = vi.fn();
    renderPanels(
      panelActions({
        panel: {
          kind: "check",
          target: { name: row.name, owner: row.owner },
          row,
          anchor: mcpPanelAnchor(rowButton()),
        },
        panelStillFailing: true,
        check,
      })
    );

    const panel = screen.getByRole("dialog", { name: `${row.name} isn't working` });
    expect(within(panel).getByTestId("mcp-issue-panel-still")).toBeInTheDocument();
    await user.click(within(panel).getByTestId("mcp-issue-panel-check"));
    expect(check).toHaveBeenCalledExactlyOnceWith({ name: row.name, owner: "extension:ops" });
  });
});
