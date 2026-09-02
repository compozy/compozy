// Suite: window-manager wire schemas
// Invariant: strict event parsing recognizes window.navigate and client presentation frames only
// when the frame revision matches the client revision.
// Owning layer: the Web window-manager wire decoder.
import { describe, expect, it } from "vitest";

import { parseWindowManagerRegisteredClientView } from "../window-manager-schemas";
import { parseWindowManagerStreamFrame } from "../window-manager-stream-schema";

function eventFrame(commandId: string) {
  return {
    type: "event",
    workspace_id: "workspace:test",
    revision: 4,
    event: {
      workspace_id: "workspace:test",
      revision: 4,
      command_id: commandId,
      changes: {},
      actor: { kind: "web", id: "client:web" },
      origin: "web",
      occurred_at: "2026-07-22T00:00:00Z",
    },
  };
}

function clientFrame(frameRevision: number, presentationRevision: number) {
  return {
    type: "client",
    workspace_id: "workspace:test",
    revision: frameRevision,
    client: {
      workspace_id: "workspace:test",
      client_id: "client:web",
      kind: "browser",
      presentation_revision: presentationRevision,
      context_revision: 1,
      active_desktop_id: "desktop:main",
      focus_order: [],
      stack_active: {},
      palette_context: {
        window_focused: false,
        window_floating: false,
        window_stacked: false,
        desktop_window_count: 0,
        scope_global: false,
        shell_desktop: false,
        workspace_trusted: true,
      },
      connected_at: "2026-07-22T00:00:00Z",
      global_shortcuts: [],
    },
  };
}

function snapshotFrame() {
  return {
    type: "snapshot",
    workspace_id: "workspace:test",
    revision: 4,
    snapshot: {
      version: 4,
      workspace_id: "workspace:test",
      revision: 4,
      desktops: [
        {
          id: "desktop:main",
          name: "Main",
          order: 0,
          groups: [],
          floating: [],
          floating_stacks: [],
        },
      ],
      windows: {},
      closed_entry_count: 0,
      overrides: {},
      updated_at: "2026-07-22T00:00:00Z",
    },
  };
}

describe("parseWindowManagerStreamFrame", () => {
  it("Should accept window.navigate as a strict topology event command", () => {
    const parsed = parseWindowManagerStreamFrame(eventFrame("window.navigate"));

    expect(parsed).toMatchObject({
      type: "event",
      workspaceId: "workspace:test",
      revision: 4,
      event: { commandId: "window.navigate" },
    });
  });

  it("Should reject command IDs outside the semantic registry", () => {
    expect(() => parseWindowManagerStreamFrame(eventFrame("window.teleport"))).toThrow();
  });

  it("Should reject snapshot and event identities that disagree with their envelopes", () => {
    const snapshotWorkspaceMismatch = snapshotFrame();
    snapshotWorkspaceMismatch.snapshot.workspace_id = "workspace:other";
    expect(() => parseWindowManagerStreamFrame(snapshotWorkspaceMismatch)).toThrow(
      "Snapshot frame workspace must match snapshot workspace_id."
    );

    const snapshotRevisionMismatch = snapshotFrame();
    snapshotRevisionMismatch.snapshot.revision = 3;
    expect(() => parseWindowManagerStreamFrame(snapshotRevisionMismatch)).toThrow(
      "Snapshot frame revision must match snapshot revision."
    );

    const eventWorkspaceMismatch = eventFrame("window.navigate");
    eventWorkspaceMismatch.event.workspace_id = "workspace:other";
    expect(() => parseWindowManagerStreamFrame(eventWorkspaceMismatch)).toThrow(
      "Event frame workspace must match event workspace_id."
    );

    const eventRevisionMismatch = eventFrame("window.navigate");
    eventRevisionMismatch.event.revision = 3;
    expect(() => parseWindowManagerStreamFrame(eventRevisionMismatch)).toThrow(
      "Event frame revision must match event revision."
    );
  });

  it("Should accept a client frame only when both presentation revisions match", () => {
    expect(parseWindowManagerStreamFrame(clientFrame(3, 3))).toMatchObject({
      type: "client",
      revision: 3,
      client: {
        workspaceId: "workspace:test",
        clientId: "client:web",
        presentationRevision: 3,
      },
    });
    expect(() => parseWindowManagerStreamFrame(clientFrame(4, 3))).toThrow(
      "Client frame revision must match presentation_revision."
    );
  });

  it("Should omit attachment tokens from stream client projections", () => {
    const frame = clientFrame(3, 3);
    const parsed = parseWindowManagerStreamFrame({
      ...frame,
      client: { ...frame.client, attachment_token: "tok-1" },
    });
    expect(parsed.type).toBe("client");
    expect(parsed.type === "client" ? parsed.client : undefined).not.toHaveProperty(
      "attachmentToken"
    );
  });

  it("Should accept only strict daemon-to-client command envelopes", () => {
    expect(
      parseWindowManagerStreamFrame({
        type: "client_command",
        workspace_id: "workspace:test",
        command_id: "invocation-a",
        op: "palette.open",
        payload: { args: {} },
      })
    ).toEqual({
      type: "client_command",
      workspaceId: "workspace:test",
      command: {
        commandId: "invocation-a",
        op: "palette.open",
        payload: { args: {} },
      },
    });
    expect(() =>
      parseWindowManagerStreamFrame({
        type: "client_command",
        workspace_id: "workspace:test",
        command_id: "invocation-a",
        op: "palette.open",
        unexpected: true,
      })
    ).toThrow();
  });
});

describe("parseWindowManagerRegisteredClientView", () => {
  it("Should require a non-empty attachment token on registration", () => {
    const client = clientFrame(1, 1).client;
    expect(() => parseWindowManagerRegisteredClientView(client)).toThrow(
      "Window-manager registration omitted attachment_token."
    );
    expect(
      parseWindowManagerRegisteredClientView({ ...client, attachment_token: "tok-1" })
        .attachmentToken
    ).toBe("tok-1");
  });
});
