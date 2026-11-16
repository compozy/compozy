// Invariant: the sign-in panel offers only the actions the current attempt can take — copy the
// live sign-in link, retry or finish by pasting after a failure, confirm extra access before
// anything starts — and closing it cancels the attempt.
// Owner: settings MCP sign-in panel (replaces the authorize dialog suite); the hook and store
// suites own the transitions behind these callbacks.
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { UseMCPAuthorizeReturn } from "../../hooks/use-mcp-authorize";
import { mcpPanelAnchor } from "../../lib/mcp-panel-anchor";
import type { SettingsMCPServerEntry } from "../../types";

import { MCPSignInPopover } from "../mcp-panels/mcp-sign-in-popover";

const AUTH_URL = "https://auth.example/authorize?code=abc";
const REDIRECT = "http://127.0.0.1:2123/api/mcp/oauth/callback?code=xyz&state=s";

const { clipboardWriteText } = vi.hoisted(() => ({ clipboardWriteText: vi.fn() }));

function trigger(): HTMLElement {
  const button = document.createElement("button");
  button.textContent = "Sign in linear";
  document.body.append(button);
  return button;
}

function makeAuthorize(overrides: Partial<UseMCPAuthorizeReturn> = {}): UseMCPAuthorizeReturn {
  return {
    phase: "waiting",
    server: "linear",
    filter: { scope: "workspace", workspace_id: "ws-alpha" },
    begin: { authorization_url: AUTH_URL },
    error: null,
    prior: null,
    mode: "automatic",
    approvedScopes: [],
    anchor: mcpPanelAnchor(trigger()),
    requestAuthorize: vi.fn(),
    confirmScopeEscalation: vi.fn(),
    retryBegin: vi.fn(),
    enterManual: vi.fn(),
    submitManual: vi.fn(),
    acknowledgeStatus: vi.fn(),
    cancel: vi.fn(),
    ...overrides,
  } as UseMCPAuthorizeReturn;
}

function renderPanel(
  overrides: Partial<UseMCPAuthorizeReturn> = {},
  server: SettingsMCPServerEntry | null = null
) {
  const authorize = makeAuthorize(overrides);
  render(<MCPSignInPopover authorize={authorize} server={server} />);
  return authorize;
}

describe("MCPSignInPopover waiting", () => {
  beforeEach(() => {
    clipboardWriteText.mockReset();
    clipboardWriteText.mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: clipboardWriteText },
    });
  });

  it("Should name the sign-in host and copy the live sign-in link", async () => {
    renderPanel();

    const panel = screen.getByRole("dialog", { name: "Sign in to linear" });
    expect(within(panel).getByText("auth.example")).toBeInTheDocument();
    fireEvent.click(within(panel).getByRole("button", { name: "Copy sign-in link" }));

    await waitFor(() => expect(clipboardWriteText).toHaveBeenCalledWith(AUTH_URL));
  });

  it("Should reopen the sign-in page from Open again", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    renderPanel();

    fireEvent.click(screen.getByTestId("mcp-sign-in-open-again"));

    expect(open).toHaveBeenCalledWith(AUTH_URL, "_blank", "noopener,noreferrer");
    open.mockRestore();
  });

  it("Should ask for a paste sign-in only when the fold opens, then finish with the address", () => {
    const enterManual = vi.fn();
    const submitManual = vi.fn();
    renderPanel({ enterManual, submitManual });
    expect(screen.queryByTestId("mcp-sign-in-paste-input")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("mcp-sign-in-paste-toggle"));
    expect(enterManual).toHaveBeenCalledOnce();
    cleanupAndRender({ phase: "manual", mode: "manual", enterManual, submitManual });

    const input = screen.getByTestId("mcp-sign-in-paste-input");
    expect(screen.getByTestId("mcp-sign-in-paste-submit")).toBeDisabled();
    fireEvent.change(input, { target: { value: "not a url" } });
    expect(screen.getByTestId("mcp-sign-in-paste-submit")).toBeDisabled();
    fireEvent.change(input, { target: { value: REDIRECT } });
    fireEvent.click(screen.getByTestId("mcp-sign-in-paste-submit"));
    expect(submitManual).toHaveBeenCalledWith(REDIRECT);
  });

  it("Should cancel the attempt when the panel closes", () => {
    const cancel = vi.fn();
    renderPanel({ cancel });

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(cancel).toHaveBeenCalledOnce();
  });
});

function cleanupAndRender(overrides: Partial<UseMCPAuthorizeReturn>) {
  cleanup();
  renderPanel(overrides);
}

describe("MCPSignInPopover failure recovery", () => {
  it("Should retry when start fails, and never accept a pasted address it can't exchange", () => {
    const retryBegin = vi.fn();
    const enterManual = vi.fn();
    renderPanel({
      begin: null,
      error: "provider unreachable",
      phase: "failed",
      retryBegin,
      enterManual,
    });

    const panel = screen.getByRole("dialog", { name: "Sign-in didn't finish" });
    expect(within(panel).getByRole("alert")).toHaveTextContent("provider unreachable");
    expect(panel).toHaveTextContent("Nothing changed: linear keeps its previous sign-in.");
    fireEvent.click(screen.getByTestId("mcp-sign-in-paste-toggle"));
    expect(screen.getByTestId("mcp-sign-in-paste-input")).toBeDisabled();
    expect(enterManual).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByTestId("mcp-sign-in-retry"));
    expect(retryBegin).toHaveBeenCalledOnce();
  });

  it("Should keep finishing by paste after a started sign-in fails", () => {
    const submitManual = vi.fn();
    renderPanel({
      error: "provider rejected the code",
      phase: "failed",
      mode: "manual",
      submitManual,
    });

    expect(screen.getByRole("alert")).toHaveTextContent("provider rejected the code");
    const input = screen.getByTestId("mcp-sign-in-paste-input");
    expect(input).toBeEnabled();
    fireEvent.change(input, { target: { value: REDIRECT } });
    fireEvent.click(screen.getByTestId("mcp-sign-in-paste-submit"));
    expect(submitManual).toHaveBeenCalledWith(REDIRECT);
  });
});

describe("MCPSignInPopover scope escalation", () => {
  it("Should require Continue before starting a sign-in with more access", () => {
    const confirmScopeEscalation = vi.fn();
    renderPanel({
      approvedScopes: ["issues:read", "issues:write"],
      begin: null,
      confirmScopeEscalation,
      phase: "reviewing_scopes",
    });

    const access = screen.getByRole("list", { name: "More access" });
    expect(within(access).getByText("issues:read")).toBeInTheDocument();
    expect(within(access).getByText("issues:write")).toBeInTheDocument();
    expect(confirmScopeEscalation).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("mcp-sign-in-continue"));

    expect(confirmScopeEscalation).toHaveBeenCalledOnce();
  });
});

describe("MCPSignInPopover connected", () => {
  it("Should report the server's real tool count once connected", () => {
    renderPanel({ phase: "confirmed" }, {
      name: "linear",
      runtime_status: { configured: true, initialized: true, state: "ready", tool_count: 23 },
    } as SettingsMCPServerEntry);

    expect(screen.getByRole("dialog", { name: "Connected to linear" })).toHaveTextContent(
      "23 tools from linear are ready to use."
    );
  });

  it("Should not invent a count before the server reports its tools", () => {
    renderPanel({ phase: "confirmed" });

    expect(screen.getByRole("status")).toHaveTextContent("Its tools from linear are ready to use.");
  });
});
