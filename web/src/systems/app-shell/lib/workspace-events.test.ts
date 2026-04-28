import { afterEach, describe, expect, it, vi } from "vitest";

import {
  WORKSPACE_EVENT_TYPE,
  WORKSPACE_OVERFLOW_TYPE,
  buildWorkspaceSocketUrl,
  createWorkspaceEventSocketFactory,
  type WorkspaceEventSignal,
  type WorkspaceSocketLike,
} from "./workspace-events";

class FakeWorkspaceSocket implements WorkspaceSocketLike {
  static instances: FakeWorkspaceSocket[] = [];

  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  closedWith: { code: number | undefined; reason: string | undefined } | null = null;

  constructor(readonly url: string) {
    FakeWorkspaceSocket.instances.push(this);
  }

  close(code?: number, reason?: string): void {
    this.closedWith = { code, reason };
  }

  emitMessage(data: unknown): void {
    this.onmessage?.({ data } as MessageEvent);
  }

  emitClose(): void {
    this.onclose?.({ code: 1006, reason: "lost connection" } as CloseEvent);
  }
}

describe("workspace event socket", () => {
  afterEach(() => {
    FakeWorkspaceSocket.instances = [];
    vi.useRealTimers();
  });

  it("Should build the daemon workspace socket URL", () => {
    expect(
      buildWorkspaceSocketUrl({
        workspaceId: "workspace 1",
        baseUrl: "http://localhost:1985/",
      })
    ).toBe("ws://localhost:1985/api/workspaces/workspace%201/ws");
    expect(
      buildWorkspaceSocketUrl({
        workspaceId: "workspace 1",
        baseUrl: "https://localhost:1985/",
      })
    ).toBe("wss://localhost:1985/api/workspaces/workspace%201/ws");
  });

  it("Should emit parsed WebSocket event and overflow signals", () => {
    const signals: WorkspaceEventSignal[] = [];
    const factory = createWorkspaceEventSocketFactory(FakeWorkspaceSocket);
    const controller = factory(
      { workspaceId: "workspace-1", baseUrl: "http://localhost:1985" },
      signal => signals.push(signal)
    );

    const socket = FakeWorkspaceSocket.instances[0]!;
    expect(socket.url).toBe("ws://localhost:1985/api/workspaces/workspace-1/ws");

    socket.onopen?.(new Event("open"));
    socket.emitMessage(
      JSON.stringify({
        type: WORKSPACE_EVENT_TYPE,
        id: "7",
        payload: {
          seq: 7,
          ts: "2026-04-28T12:00:00Z",
          workspace_id: "workspace-1",
          kind: "run.status_changed",
          run_id: "run-1",
        },
      })
    );
    socket.emitMessage(
      JSON.stringify({
        type: WORKSPACE_OVERFLOW_TYPE,
        payload: { workspace_id: "workspace-1", reason: "subscriber_dropped_messages" },
      })
    );

    expect(signals.map(signal => signal.type)).toEqual(["open", "event", "overflow"]);
    expect(signals[1]).toMatchObject({
      type: "event",
      eventId: "7",
      payload: { workspace_id: "workspace-1", kind: "run.status_changed" },
    });

    controller.close();
    expect(socket.closedWith).toEqual({ code: 1000, reason: "workspace event stream closed" });
  });

  it("Should reconnect after a socket close and cancel reconnect on controller close", () => {
    vi.useFakeTimers();

    const factory = createWorkspaceEventSocketFactory(FakeWorkspaceSocket);
    const controller = factory(
      { workspaceId: "workspace-1", baseUrl: "http://localhost:1985" },
      () => {}
    );

    expect(FakeWorkspaceSocket.instances).toHaveLength(1);
    FakeWorkspaceSocket.instances[0]!.emitClose();
    expect(FakeWorkspaceSocket.instances).toHaveLength(1);

    vi.advanceTimersByTime(249);
    expect(FakeWorkspaceSocket.instances).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(FakeWorkspaceSocket.instances).toHaveLength(2);

    FakeWorkspaceSocket.instances[1]!.emitClose();
    controller.close();
    vi.advanceTimersByTime(5000);
    expect(FakeWorkspaceSocket.instances).toHaveLength(2);
  });
});
