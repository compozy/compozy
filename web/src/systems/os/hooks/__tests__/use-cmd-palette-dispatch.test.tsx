// Suite: command palette dispatch targeting
// Invariant: daemon invokes carry the bound client, navigation preserves route
// intent, a stale runById target is announced, and host-target copy writes the
// clipboard after seam policy gates. Pending approvals keep the decision surface
// open, preserve its recorded profile, and require a server result before success.
// Owning layer: OS dispatch and approval hooks; this is their canonical suite.
// Boundary IN: resolveInvokeClientId and useCmdPaletteDispatch.run.
// Boundary OUT: OpenAPI invoke transport, app navigation, catalog cache, and
// navigator.clipboard.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { notifyUser } from "@/lib/user-feedback";

import { paletteCommand, shellFixture } from "../../lib/__tests__/cmd-palette-dispatch-fixtures";
import { CLIPBOARD_UNAVAILABLE_REASON } from "../../lib/cmd-palette-copy";
import { STALE_TARGET_REASON } from "../../lib/cmd-palette-dispatch";
import type { CmdPaletteApprovalStatus, PaletteRegistry } from "../../lib/cmd-palette-types";
import {
  getPendingToolApproval,
  invokeCmdPaletteCommand,
  resolvePendingToolApproval,
} from "../../adapters/cmd-palette-api";
import {
  cmdPaletteExecutionStore,
  resetPaletteExecutionEntry,
} from "../../stores/cmd-palette-execution-store";
import { useCmdPaletteApproval } from "../use-cmd-palette-approval";
import {
  resolveInvokeAttachmentToken,
  resolveInvokeClientId,
  useCmdPaletteDispatch,
} from "../use-cmd-palette-dispatch";

vi.mock("@/lib/user-feedback", () => ({ notifyUser: vi.fn() }));

vi.mock("../../adapters/cmd-palette-api", () => ({
  invokeCmdPaletteCommand: vi.fn(),
  getPendingToolApproval: vi.fn(),
  resolvePendingToolApproval: vi.fn(),
  recordCmdPaletteUsage: vi.fn(async () => undefined),
  setCmdPalettePin: vi.fn(),
}));

vi.mock("../use-os-shell", () => ({
  useOsShell: () => ({ manager: {}, coordinator: {}, projection: {} }),
}));

const emptyRegistry: PaletteRegistry = {
  commands: [],
  byId: new Map(),
  sources: [],
  catalogRevision: "sha256:empty",
  stale: false,
  daemonReachable: true,
};

function wrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("command palette dispatch targeting", () => {
  it("Should present the pending approval under its recorded profile without approving it", async () => {
    resetPaletteExecutionEntry();
    vi.mocked(invokeCmdPaletteCommand).mockResolvedValue({
      status: "approval_pending",
      approval_id: "apr_owner",
      invocation_id: "inv_owner",
      profile_lens: { profile_lens_id: "profile-owner", profile_name: "editorial" },
    });
    const shell = shellFixture();
    const command = paletteCommand({
      id: "ext.notes.publish",
      action: { kind: "tool", tool: "ext.notes.publish" },
    });
    const { result } = renderHook(
      () =>
        useCmdPaletteDispatch({
          registry: emptyRegistry,
          workspaceId: "ws_home",
          shell,
          openApp: vi.fn(),
        }),
      { wrapper: wrapper() }
    );

    await expect(result.current.run(command)).resolves.toMatchObject({ status: "needs_approval" });

    expect(cmdPaletteExecutionStore.getSnapshot().context.entry).toMatchObject({
      kind: "approval",
      commandId: "ext.notes.publish",
      approval: { id: "apr_owner", profile: "editorial" },
    });
    expect(shell.openPaletteExecution).toHaveBeenCalled();
    resetPaletteExecutionEntry();
  });

  it("Should wait for the recorded owner's execution result after approval", async () => {
    const pending: CmdPaletteApprovalStatus = {
      approval_status: "pending",
      expires_at: "2026-10-04T15:00:00Z",
    };
    const approved: CmdPaletteApprovalStatus = {
      ...pending,
      approval_status: "approved",
      execution_status: "dispatching",
    };
    let complete!: (status: CmdPaletteApprovalStatus) => void;
    const execution = new Promise<CmdPaletteApprovalStatus>(resolve => {
      complete = resolve;
    });
    vi.mocked(getPendingToolApproval)
      .mockReset()
      .mockResolvedValueOnce(pending)
      .mockImplementation(() => execution);
    vi.mocked(resolvePendingToolApproval).mockReset().mockResolvedValue(approved);
    const { result } = renderHook(
      () => useCmdPaletteApproval({ id: "apr_owner", profile: "editorial" }),
      {
        wrapper: wrapper(),
      }
    );
    await waitFor(() => expect(result.current.canDecide).toBe(true));

    act(() => result.current.decide("approved"));

    await waitFor(() =>
      expect(resolvePendingToolApproval).toHaveBeenCalledExactlyOnceWith(
        "editorial",
        "apr_owner",
        "approved"
      )
    );
    await waitFor(() => expect(result.current.message).toContain("Waiting"));
    expect(result.current.canDecide).toBe(false);
    expect(result.current.isPending).toBe(true);
    expect(getPendingToolApproval).toHaveBeenLastCalledWith(
      "editorial",
      "apr_owner",
      expect.any(AbortSignal)
    );

    act(() => complete({ ...approved, execution_status: "completed" }));

    await waitFor(() => expect(result.current.message).toBe("Command finished."));
    expect(result.current.isPending).toBe(false);
    expect(result.current.canDecide).toBe(false);
  });

  it("Should keep a denied approval terminal without reporting command success", async () => {
    const pending: CmdPaletteApprovalStatus = {
      approval_status: "pending",
      expires_at: "2026-10-04T15:00:00Z",
    };
    const denied: CmdPaletteApprovalStatus = { ...pending, approval_status: "denied" };
    vi.mocked(getPendingToolApproval)
      .mockReset()
      .mockResolvedValueOnce(pending)
      .mockResolvedValue(denied);
    vi.mocked(resolvePendingToolApproval).mockReset().mockResolvedValue(denied);
    const { result } = renderHook(
      () => useCmdPaletteApproval({ id: "apr_owner", profile: "editorial" }),
      {
        wrapper: wrapper(),
      }
    );
    await waitFor(() => expect(result.current.canDecide).toBe(true));

    act(() => result.current.decide("denied"));

    await waitFor(() => expect(result.current.message).toContain("did not run"));
    expect(result.current.canDecide).toBe(false);
    expect(resolvePendingToolApproval).toHaveBeenCalledExactlyOnceWith(
      "editorial",
      "apr_owner",
      "denied"
    );
  });

  it("Should show the persisted execution failure from the approval response", async () => {
    vi.mocked(getPendingToolApproval)
      .mockReset()
      .mockResolvedValue({
        approval_status: "approved",
        execution_status: "failed",
        error: { message: "Tool backend failed" },
      });
    const { result } = renderHook(
      () => useCmdPaletteApproval({ id: "apr_owner", profile: "editorial" }),
      {
        wrapper: wrapper(),
      }
    );
    await waitFor(() => expect(result.current.error).toBe("Tool backend failed"));
    expect(result.current.canDecide).toBe(false);
    expect(result.current.isPending).toBe(false);
  });

  it("Should prefer an explicit client id over the bound window-manager client [RD0082]", () => {
    expect(resolveInvokeClientId(" client-a ", "client-b")).toBe("client-a");
    expect(resolveInvokeClientId("  ", "client-b")).toBe("client-b");
    expect(resolveInvokeClientId(undefined, undefined)).toBeUndefined();
    expect(resolveInvokeAttachmentToken(" attachment-token ")).toBe("attachment-token");
    expect(resolveInvokeAttachmentToken("  ")).toBeUndefined();
  });

  it("Should send the attached client id and token together on invoke [RD0082]", async () => {
    vi.mocked(invokeCmdPaletteCommand).mockResolvedValue({
      status: "ok",
      invocation_id: "inv-rd0082",
      profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
    });
    const command = paletteCommand({
      id: "ext.notes.capture",
      action: { kind: "tool", tool: "ext.notes.capture" },
    });
    const { result } = renderHook(
      () =>
        useCmdPaletteDispatch({
          registry: emptyRegistry,
          workspaceId: "ws_home",
          shell: shellFixture(),
          openApp: vi.fn(),
          clientId: "client-web",
          attachmentToken: "attachment-token",
        }),
      { wrapper: wrapper() }
    );

    await expect(result.current.run(command)).resolves.toMatchObject({
      status: "invoked",
      result: { status: "ok", invocation_id: "inv-rd0082" },
    });
    expect(invokeCmdPaletteCommand).toHaveBeenCalledWith({
      workspaceId: "ws_home",
      // The command runs as a profile, so the acting destination rides the call.
      // Omitting it would run every palette command as `default`.
      profile: "default",
      commandId: "ext.notes.capture",
      args: {},
      clientId: "client-web",
      attachmentToken: "attachment-token",
    });
  });

  it("Should omit the invoke token when the surface has no attachment [RD0082]", async () => {
    vi.mocked(invokeCmdPaletteCommand).mockResolvedValue({
      status: "ok",
      invocation_id: "inv-control-plane",
      profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
    });
    const command = paletteCommand({
      id: "ext.notes.capture",
      action: { kind: "tool", tool: "ext.notes.capture" },
    });
    const { result } = renderHook(
      () =>
        useCmdPaletteDispatch({
          registry: emptyRegistry,
          workspaceId: "ws_home",
          shell: shellFixture(),
          openApp: vi.fn(),
        }),
      { wrapper: wrapper() }
    );

    await result.current.run(command);
    const input = vi.mocked(invokeCmdPaletteCommand).mock.calls.at(-1)?.[0];
    expect(input).toMatchObject({
      workspaceId: "ws_home",
      profile: "default",
      commandId: "ext.notes.capture",
      args: {},
    });
    expect(input).not.toHaveProperty("attachmentToken");
  });

  it("Should preserve route intent when a command navigates to an app", async () => {
    const openApp = vi.fn();
    const command = paletteCommand({
      id: "profile.archive",
      arguments: [{ name: "profile", type: "text", required: true }],
      action: {
        kind: "navigate",
        app: "settings",
        args: { pathname: "/settings/profiles", flow: "archive" },
      },
    });
    const { result } = renderHook(
      () =>
        useCmdPaletteDispatch({
          registry: emptyRegistry,
          workspaceId: "ws_home",
          shell: shellFixture(),
          openApp,
        }),
      { wrapper: wrapper() }
    );

    await expect(result.current.run(command, { args: { profile: "marketing" } })).resolves.toEqual({
      status: "ran",
    });
    expect(openApp).toHaveBeenCalledExactlyOnceWith("settings", {
      pathname: "/settings/profiles",
      search: { flow: "archive", profile: "marketing" },
    });
  });

  it("Should announce when runById names a command the registry no longer carries [RD0094]", async () => {
    const { result } = renderHook(
      () =>
        useCmdPaletteDispatch({
          registry: emptyRegistry,
          workspaceId: "ws_home",
          shell: shellFixture(),
          openApp: vi.fn(),
        }),
      { wrapper: wrapper() }
    );

    await expect(result.current.runById("gone.command")).resolves.toEqual({
      status: "refused",
      reason: STALE_TARGET_REASON,
    });
    expect(notifyUser).toHaveBeenCalledWith({
      message: STALE_TARGET_REASON,
      tone: "error",
    });
  });

  it("Should write host-target copy content through the clipboard after policy gates [RD0039]", async () => {
    const writeText = vi.fn<(value: string) => Promise<void>>().mockResolvedValue(undefined);
    const clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    const command = paletteCommand({
      id: "view-action.notes.browser",
      title: "Copy",
      action: { kind: "copy", args: { content: "clipboard text" } },
    });
    const { result } = renderHook(
      () =>
        useCmdPaletteDispatch({
          registry: emptyRegistry,
          workspaceId: "ws_home",
          shell: shellFixture(),
          openApp: vi.fn(),
        }),
      { wrapper: wrapper() }
    );

    try {
      await expect(result.current.run(command)).resolves.toEqual({ status: "ran" });
      expect(writeText).toHaveBeenCalledExactlyOnceWith("clipboard text");
    } finally {
      if (clipboardDescriptor) {
        Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
      } else {
        Reflect.deleteProperty(navigator, "clipboard");
      }
    }
  });

  it("Should refuse a host-target copy when clipboard access is unavailable [RD0039]", async () => {
    const clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
    const command = paletteCommand({
      id: "view-action.notes.browser",
      title: "Copy",
      action: { kind: "copy", args: { content: "clipboard text" } },
    });
    const { result } = renderHook(
      () =>
        useCmdPaletteDispatch({
          registry: emptyRegistry,
          workspaceId: "ws_home",
          shell: shellFixture(),
          openApp: vi.fn(),
        }),
      { wrapper: wrapper() }
    );

    try {
      await expect(result.current.run(command)).resolves.toEqual({
        status: "refused",
        reason: CLIPBOARD_UNAVAILABLE_REASON,
      });
    } finally {
      if (clipboardDescriptor) {
        Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
      } else {
        Reflect.deleteProperty(navigator, "clipboard");
      }
    }
  });
});
