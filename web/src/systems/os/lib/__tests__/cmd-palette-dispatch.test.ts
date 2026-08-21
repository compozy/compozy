// Suite: cmd-palette dispatch seam
// Invariant: one seam routes every action kind to exactly one effect — client
// operations to the client-op table, tool actions to the daemon invoke,
// navigate/view/url to their shell owners, copy to the clipboard port after
// policy gates — refuses an unavailable command with the runtime's verbatim
// reason, and surfaces a stale target honestly instead of half-executing.
// Owning layer: web/src/systems/os/lib/cmd-palette-dispatch.ts
// Boundary OUT: feedback copy (cmd-palette-feedback suite) and the client-op
// lookup table (cmd-palette-client-ops suite).
import { describe, expect, it, vi } from "vitest";

import {
  COPY_CONTENT_TOO_LARGE_REASON,
  COPY_REQUIRES_TARGET_REASON,
  PALETTE_COPY_MAX_BYTES,
  copyCannotCarryReason,
} from "../cmd-palette-copy";
import {
  dispatchPaletteCommand,
  STALE_TARGET_REASON,
  UNSUPPORTED_CLIENT_OP_REASON,
} from "../cmd-palette-dispatch";
import type { ResolvedPaletteCommand } from "../cmd-palette-types";
import { paletteCommand, portsFixture } from "./cmd-palette-dispatch-fixtures";

describe("cmd-palette dispatch seam (UT-105)", () => {
  it("Should route a client_op to the client-op table and record usage", async () => {
    const { ports, context } = portsFixture();
    const outcome = await dispatchPaletteCommand({ command: paletteCommand(), ports });
    expect(outcome).toEqual({ status: "ran" });
    expect(context.manager.closeWindow).toHaveBeenCalledExactlyOnceWith("w-2");
    expect(ports.invoke).not.toHaveBeenCalled();
    expect(ports.reportUsage).toHaveBeenCalledExactlyOnceWith("window.close", "");
  });

  it("Should route a tool action to the daemon invoke and leave usage to the daemon", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: paletteCommand({
        id: "ext.notes.capture",
        action: { kind: "tool", tool: "ext.notes.capture" },
      }),
      args: { title: "Standup" },
      ports,
    });
    expect(outcome).toEqual({
      status: "invoked",
      result: { status: "ok", invocation_id: "inv-fixture" },
    });
    expect(ports.invoke).toHaveBeenCalledExactlyOnceWith("ext.notes.capture", { title: "Standup" });
    expect(ports.reportUsage).not.toHaveBeenCalled();
  });

  it("Should route navigate, view and url actions to their shell owners", async () => {
    const { ports } = portsFixture();
    await dispatchPaletteCommand({
      command: paletteCommand({
        id: "settings.appearance",
        action: {
          kind: "navigate",
          app: "settings",
          args: { pathname: "/settings/appearance" },
        },
      }),
      ports,
    });
    expect(ports.navigate).toHaveBeenCalledExactlyOnceWith("settings", "/settings/appearance", {});

    // A navigate action carries intent beyond its destination — which lifecycle
    // flow to raise, against which target. Dropping it would land the operator
    // on a page that has to guess why they arrived.
    const { ports: flowPorts } = portsFixture();
    await dispatchPaletteCommand({
      command: paletteCommand({
        id: "profile.archive",
        action: {
          kind: "navigate",
          app: "settings",
          args: { pathname: "/settings/profiles", flow: "archive" },
        },
      }),
      args: { profile: "marketing" },
      ports: flowPorts,
    });
    expect(flowPorts.navigate).toHaveBeenCalledExactlyOnceWith("settings", "/settings/profiles", {
      flow: "archive",
      profile: "marketing",
    });

    await dispatchPaletteCommand({
      command: paletteCommand({
        id: "palette.view.sessions",
        action: { kind: "view", view: "sessions" },
      }),
      ports,
    });
    expect(ports.pushView).toHaveBeenCalledExactlyOnceWith("sessions");

    await dispatchPaletteCommand({
      command: paletteCommand({
        id: "help.docs",
        action: { kind: "url", url: "https://compozy.com/docs" },
      }),
      ports,
    });
    expect(ports.openUrl).toHaveBeenCalledExactlyOnceWith("https://compozy.com/docs");
  });

  it("Should route a host-target copy action to the clipboard port without invoking [RD0039]", async () => {
    const { ports } = portsFixture();
    const command = paletteCommand({
      id: "view-action.notes.browser",
      title: "Copy",
      action: { kind: "copy", args: { content: "  clipboard text  " } },
    });
    const outcome = await dispatchPaletteCommand({ command, ports });
    expect(outcome).toEqual({ status: "ran" });
    expect(ports.copyToClipboard).toHaveBeenCalledExactlyOnceWith("clipboard text");
    expect(ports.invoke).not.toHaveBeenCalled();
    expect(ports.reportUsage).toHaveBeenCalledExactlyOnceWith("view-action.notes.browser", "");
    expect(ports.onPendingStart).not.toHaveBeenCalled();
  });

  it("Should merge declared action args under caller-supplied values", async () => {
    const { ports } = portsFixture();
    await dispatchPaletteCommand({
      command: paletteCommand({
        id: "ext.notes.capture",
        action: { kind: "tool", tool: "ext.notes.capture", args: { folder: "inbox" } },
      }),
      args: { title: "Standup" },
      ports,
    });
    expect(ports.invoke).toHaveBeenCalledExactlyOnceWith("ext.notes.capture", {
      folder: "inbox",
      title: "Standup",
    });
  });
});

describe("cmd-palette dispatch refusals (UT-106)", () => {
  it("Should refuse an unavailable command with the runtime reason instead of running it", async () => {
    const { ports, context } = portsFixture();
    const unavailable = paletteCommand({
      available: false,
      reason: "needs two windows on this desktop",
    });
    const outcome = await dispatchPaletteCommand({ command: unavailable, ports });
    expect(outcome).toEqual({
      status: "refused",
      reason: "needs two windows on this desktop",
    });
    expect(context.manager.closeWindow).not.toHaveBeenCalled();
    expect(ports.onFailure).toHaveBeenCalledExactlyOnceWith(
      unavailable,
      "needs two windows on this desktop"
    );
  });

  it("Should surface a stale target honestly when the invoke rejects", async () => {
    const { ports } = portsFixture({
      invoke: vi.fn(async () => {
        throw new Error("Session no longer exists");
      }),
    });
    const stale = paletteCommand({
      id: "session.open",
      action: { kind: "tool", tool: "session.open" },
    });
    const outcome = await dispatchPaletteCommand({ command: stale, ports });
    expect(outcome).toEqual({ status: "refused", reason: "Session no longer exists" });
    expect(ports.onFailure).toHaveBeenCalledExactlyOnceWith(
      stale,
      "Session no longer exists",
      undefined
    );
    expect(ports.refresh).toHaveBeenCalledOnce();
  });

  it("Should refuse a client operation this build cannot perform rather than failing silently", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: paletteCommand({
        id: "window.swap",
        action: { kind: "client_op", op: "window.swap" },
      }),
      ports,
    });
    expect(outcome).toEqual({ status: "refused", reason: UNSUPPORTED_CLIENT_OP_REASON });
  });

  it("Should fall back to an honest reason when an unavailable command carries none", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: paletteCommand({ available: false, reason: "" }),
      ports,
    });
    expect(outcome).toEqual({ status: "refused", reason: STALE_TARGET_REASON });
  });
});

describe("cmd-palette pre-execution gates (UT-120, UT-123)", () => {
  const withArguments = paletteCommand({
    id: "ext.notes.capture",
    title: "Capture note",
    action: { kind: "tool", tool: "ext__notes__capture" },
    arguments: [{ name: "title", type: "text", required: true, placeholder: "Note title" }],
  });
  const withConfirmation = paletteCommand({
    id: "ext.notes.purge",
    title: "Purge archived notes",
    action: { kind: "tool", tool: "ext__notes__purge" },
    destructive: true,
    confirmation: {
      title: "Purge archived notes?",
      body: "Permanently deletes every archived note in this workspace.",
      confirm: "Purge",
    },
  });

  it("Should raise the argument step instead of running a command that declares arguments", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({ command: withArguments, ports });
    expect(outcome).toEqual({ status: "needs_args" });
    expect(ports.requestArgs).toHaveBeenCalledExactlyOnceWith(withArguments);
    expect(ports.invoke).not.toHaveBeenCalled();
  });

  it("Should run once the arguments arrive", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: withArguments,
      args: { title: "Standup follow-ups" },
      ports,
    });
    expect(outcome).toEqual({
      status: "invoked",
      result: { status: "ok", invocation_id: "inv-fixture" },
    });
    expect(ports.requestArgs).not.toHaveBeenCalled();
  });

  it("Should raise the confirmation step carrying the arguments already collected", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: withConfirmation,
      args: { scope: "workspace" },
      ports,
    });
    expect(outcome).toEqual({ status: "needs_confirmation" });
    expect(ports.requestConfirmation).toHaveBeenCalledExactlyOnceWith(withConfirmation, {
      scope: "workspace",
    });
    expect(ports.invoke).not.toHaveBeenCalled();
  });

  it("Should run a confirmed destructive command exactly once", async () => {
    const { ports } = portsFixture();
    const outcome = await dispatchPaletteCommand({
      command: withConfirmation,
      args: { scope: "workspace" },
      confirmed: true,
      ports,
    });
    expect(outcome).toEqual({
      status: "invoked",
      result: { status: "ok", invocation_id: "inv-fixture" },
    });
    expect(ports.invoke).toHaveBeenCalledOnce();
    expect(ports.requestConfirmation).not.toHaveBeenCalled();
  });

  it("Should refuse an unavailable command before asking for anything", async () => {
    const { ports } = portsFixture();
    await dispatchPaletteCommand({
      command: paletteCommand({
        ...withArguments,
        available: false,
        reason: "extension notes is unhealthy (crash loop)",
      }),
      ports,
    });
    expect(ports.requestArgs).not.toHaveBeenCalled();
  });
});

describe("cmd-palette feedback lifecycle (UT-159, UT-160)", () => {
  const asyncCommand = paletteCommand({
    id: "ext.notes.capture",
    title: "Capture note",
    action: { kind: "tool", tool: "ext__notes__capture" },
  });

  it("Should hold the command pending across the invoke and report its completion", async () => {
    const order: string[] = [];
    const { ports } = portsFixture({
      invoke: vi.fn(async () => {
        order.push("invoke");
        return { status: "ok", invocation_id: "inv-async" };
      }),
      onPendingStart: vi.fn(() => order.push("start")),
      onPendingSettle: vi.fn(() => order.push("settle")),
      onCompleted: vi.fn(() => order.push("completed")),
    });
    await dispatchPaletteCommand({ command: asyncCommand, ports });
    expect(order).toEqual(["start", "invoke", "completed", "settle"]);
  });

  it("Should release the pending state when the invoke fails mid-flight", async () => {
    const { ports } = portsFixture({
      invoke: vi.fn(async () => {
        throw new Error("runtime unavailable");
      }),
    });
    const outcome = await dispatchPaletteCommand({ command: asyncCommand, ports });
    expect(outcome).toEqual({ status: "refused", reason: "runtime unavailable" });
    expect(ports.onPendingSettle).toHaveBeenCalledExactlyOnceWith(asyncCommand);
    expect(ports.onCompleted).not.toHaveBeenCalled();
  });

  it("Should carry the daemon's error code so retry can be gated on it", async () => {
    const rejection = Object.assign(new Error("ext.notes.purge is already in flight"), {
      code: "already_running",
    });
    const { ports } = portsFixture({
      invoke: vi.fn(async () => {
        throw rejection;
      }),
    });
    const outcome = await dispatchPaletteCommand({ command: asyncCommand, ports });
    expect(outcome).toEqual({
      status: "refused",
      reason: "ext.notes.purge is already in flight",
      code: "already_running",
    });
    expect(ports.onFailure).toHaveBeenCalledExactlyOnceWith(
      asyncCommand,
      "ext.notes.purge is already in flight",
      "already_running"
    );
  });

  it("Should never report a synchronous client operation as pending", async () => {
    const { ports } = portsFixture();
    await dispatchPaletteCommand({ command: paletteCommand(), ports });
    expect(ports.onPendingStart).not.toHaveBeenCalled();
    expect(ports.onCompleted).not.toHaveBeenCalled();
  });
});

describe("cmd-palette copy policy (RD0039)", () => {
  const copyCommand = (action: ResolvedPaletteCommand["action"]) =>
    paletteCommand({
      id: "view-action.notes.browser",
      title: "Copy",
      action,
    });

  it("Should refuse invalid copy payloads before writing the clipboard", async () => {
    const cases: ReadonlyArray<{
      readonly action: ResolvedPaletteCommand["action"];
      readonly reason: string;
    }> = [
      { action: { kind: "copy" }, reason: COPY_REQUIRES_TARGET_REASON },
      { action: { kind: "copy", args: { content: "   " } }, reason: COPY_REQUIRES_TARGET_REASON },
      { action: { kind: "copy", args: { content: 1 } }, reason: COPY_REQUIRES_TARGET_REASON },
      {
        action: { kind: "copy", url: "https://example.com", args: { content: "clipboard text" } },
        reason: copyCannotCarryReason("url"),
      },
      {
        action: {
          kind: "copy",
          args: { content: "clipboard text", mime: "text/plain" },
        },
        reason: copyCannotCarryReason("mime"),
      },
      {
        action: {
          kind: "copy",
          args: { content: "x".repeat(PALETTE_COPY_MAX_BYTES + 1) },
        },
        reason: COPY_CONTENT_TOO_LARGE_REASON,
      },
    ];
    for (const { action, reason } of cases) {
      const { ports } = portsFixture();
      const command = copyCommand(action);
      const outcome = await dispatchPaletteCommand({ command, ports });
      expect(outcome, reason).toEqual({ status: "refused", reason });
      expect(ports.copyToClipboard).not.toHaveBeenCalled();
      expect(ports.invoke).not.toHaveBeenCalled();
    }
  });

  it("Should raise confirmation before writing clipboard content", async () => {
    const { ports } = portsFixture();
    const command = paletteCommand({
      id: "view-action.notes.browser",
      title: "Copy secret",
      destructive: true,
      confirmation: {
        title: "Copy secret?",
        body: "Places the secret on the clipboard.",
        confirm: "Copy",
      },
      action: { kind: "copy", args: { content: "secret" } },
    });
    const outcome = await dispatchPaletteCommand({ command, ports });
    expect(outcome).toEqual({ status: "needs_confirmation" });
    expect(ports.requestConfirmation).toHaveBeenCalledExactlyOnceWith(command, {
      content: "secret",
    });
    expect(ports.copyToClipboard).not.toHaveBeenCalled();
  });

  it("Should refuse when the clipboard port rejects after policy passes", async () => {
    const { ports } = portsFixture({
      copyToClipboard: vi.fn(async () => {
        throw new Error("Clipboard access is unavailable.");
      }),
    });
    const command = copyCommand({ kind: "copy", args: { content: "clipboard text" } });
    const outcome = await dispatchPaletteCommand({ command, ports });
    expect(outcome).toEqual({
      status: "refused",
      reason: "Clipboard access is unavailable.",
    });
    expect(ports.onFailure).toHaveBeenCalledExactlyOnceWith(
      command,
      "Clipboard access is unavailable."
    );
    expect(ports.invoke).not.toHaveBeenCalled();
  });
});
