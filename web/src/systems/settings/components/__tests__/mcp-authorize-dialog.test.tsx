import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { UseMCPAuthorizeReturn } from "../../hooks/use-mcp-authorize";

import { MCPAuthorizeDialog } from "../mcp-authorize-dialog";

const AUTH_URL = "https://auth.example/authorize?code=abc";

const { clipboardWriteText, toast } = vi.hoisted(() => ({
  clipboardWriteText: vi.fn(),
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock("sonner", () => ({ toast }));

function installClipboard() {
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText: clipboardWriteText },
  });
}

function makeAuthorize(overrides: Partial<UseMCPAuthorizeReturn> = {}): UseMCPAuthorizeReturn {
  return {
    phase: "waiting",
    server: "linear",
    begin: { authorization_url: AUTH_URL },
    error: null,
    prior: null,
    mode: "automatic",
    isBeginning: false,
    isExchanging: false,
    isAwaiting: true,
    isOpen: true,
    beginAuthorize: vi.fn(),
    retryBegin: vi.fn(),
    enterManual: vi.fn(),
    submitManual: vi.fn(),
    acknowledgeStatus: vi.fn(),
    cancel: vi.fn(),
    ...overrides,
  } as UseMCPAuthorizeReturn;
}

function renderDialog(overrides: Partial<UseMCPAuthorizeReturn> = {}) {
  return render(
    <MCPAuthorizeDialog authorize={makeAuthorize(overrides)} scope="workspace" server={null} />
  );
}

describe("MCPAuthorizeDialog copy URL", () => {
  beforeEach(() => {
    clipboardWriteText.mockReset();
    clipboardWriteText.mockResolvedValue(undefined);
    toast.error.mockReset();
  });

  it("Should mark Copied only after a successful clipboard write", async () => {
    installClipboard();
    renderDialog();

    const button = screen.getByTestId("settings-page-mcp-authorize-copy-url");
    expect(button).toHaveTextContent("Copy URL");

    fireEvent.click(button);

    expect(clipboardWriteText).toHaveBeenCalledWith(AUTH_URL);
    await waitFor(() => expect(button).toHaveTextContent("Copied"));
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("Should surface an error and never claim Copied when the clipboard write fails", async () => {
    clipboardWriteText.mockRejectedValueOnce(new Error("Clipboard permission denied"));
    installClipboard();
    renderDialog();

    const button = screen.getByTestId("settings-page-mcp-authorize-copy-url");
    fireEvent.click(button);

    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1));
    expect(button).toHaveTextContent("Copy URL");
    expect(button).not.toHaveTextContent("Copied");
  });

  it("Should surface an error and never write when the clipboard API is unavailable", async () => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
    renderDialog();

    const button = screen.getByTestId("settings-page-mcp-authorize-copy-url");
    fireEvent.click(button);

    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1));
    expect(clipboardWriteText).not.toHaveBeenCalled();
    expect(button).toHaveTextContent("Copy URL");
  });
});

describe("MCPAuthorizeDialog failure recovery", () => {
  it("Should retry begin without offering an impossible exchange when start fails", () => {
    const retryBegin = vi.fn();
    const enterManual = vi.fn();
    renderDialog({
      begin: null,
      error: "provider unreachable",
      phase: "failed",
      retryBegin,
      enterManual,
    });

    expect(screen.getByText("Authorization could not be started")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-mcp-authorize-manual")).not.toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-mcp-authorize-exchange")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("settings-page-mcp-authorize-manual-fallback"));
    expect(enterManual).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByTestId("settings-page-mcp-authorize-retry"));
    expect(retryBegin).toHaveBeenCalledOnce();
  });

  it("Should retain manual exchange after a started authorization fails", () => {
    renderDialog({ error: "provider rejected the code", phase: "failed", mode: "manual" });

    expect(screen.getByText("Authorization could not be completed")).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-mcp-authorize-manual")).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-mcp-authorize-exchange")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-mcp-authorize-retry")).not.toBeInTheDocument();
  });
});
