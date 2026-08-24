// Suite: programmable command-palette view lifecycle
// Invariant: switching profile tears down the old session before the new owner can publish.
// Boundary IN: profile lens changes, session open/close, and queued stream frames.
// Boundary OUT: adapter serialization and pure program-store frame reduction.

import { act, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { StreamEventSource } from "@/lib/ticketed-event-source";

const lifecycleMocks = vi.hoisted(() => ({
  admit: vi.fn(),
  close: vi.fn(),
  open: vi.fn(),
  readProfile: vi.fn(() => "marketing"),
}));

vi.mock("@/systems/profiles", () => ({
  useProfileReadScope: () => {
    const profile = lifecycleMocks.readProfile();
    return { destination: profile, key: profile };
  },
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ runtimeWorkspaceId: "ws-hq" }),
}));

vi.mock("../../adapters/cmd-palette-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/cmd-palette-api")>();
  return {
    ...actual,
    admitCmdPaletteViewSessionEvent: lifecycleMocks.admit,
    closeCmdPaletteViewSession: lifecycleMocks.close,
    openCmdPaletteViewSession: lifecycleMocks.open,
  };
});

vi.mock("../use-palette-registry", () => ({
  usePaletteRegistry: () => ({
    commands: [],
    byId: new Map(),
    sources: [],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  }),
}));

import type { CmdPaletteDispatch } from "../use-cmd-palette-dispatch";
import { useCmdPaletteProgramView } from "../use-cmd-palette-program-view";
import type {
  CmdPaletteViewFrame,
  CmdPaletteViewSessionOpenResponse,
} from "../../lib/cmd-palette-types";
import type { WindowManagerRegisteredClientView } from "../../lib/window-manager-types";

const dispatch: CmdPaletteDispatch = {
  run: vi.fn(async () => ({ status: "ran" }) as const),
  runById: vi.fn(async () => ({ status: "ran" }) as const),
  executeClientOp: vi.fn(async () => undefined),
  setPinned: vi.fn(async () => undefined),
};

const client: WindowManagerRegisteredClientView = {
  workspaceId: "ws-hq",
  clientId: "client:test",
  kind: "browser",
  presentationRevision: 1,
  contextRevision: 1,
  activeDesktopId: "desktop:main",
  focusedWindowId: null,
  focusOrder: [],
  stackActive: {},
  paletteContext: {
    windowFocused: false,
    windowFloating: false,
    windowStacked: false,
    desktopWindowCount: 0,
    scopeGlobal: false,
    shellDesktop: false,
    focusedSessionState: null,
    workspaceTrusted: true,
    destinationIntent: null,
  },
  connectedAt: "2026-08-23T00:00:00Z",
  attachmentToken: "attachment-token",
  globalShortcuts: [],
};

class FakeEventSource implements StreamEventSource {
  readonly listeners = new Map<string, EventListenerOrEventListenerObject>();
  readonly close = vi.fn();
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onopen: ((event: Event) => void) | null = null;

  addEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    this.listeners.set(type, listener);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    if (this.listeners.get(type) === listener) this.listeners.delete(type);
  }
}

function frame(profile: string, title: string): CmdPaletteViewFrame {
  return {
    view_session: `view:${profile}`,
    revision: `revision:${profile}:${title}`,
    generation: 1,
    handlers: [],
    payload: {
      view: "v1",
      chrome: { complete: true },
      sections: [{ rows: [{ id: `${profile}:row`, title }] }],
    },
  };
}

function openedSession(profile: string): CmdPaletteViewSessionOpenResponse {
  return {
    profile_lens: { profile_lens_id: `${profile}:id`, profile_name: profile },
    view_session: `view:${profile}`,
    stream_token: `stream:${profile}`,
    first_frame: frame(profile, `${profile} row`),
  };
}

function invoke(listener: EventListenerOrEventListenerObject, event: Event): void {
  if (typeof listener === "function") listener(event);
  else listener.handleEvent(event);
}

describe("useCmdPaletteProgramView", () => {
  it("Should close the old profile session and reject its queued frame after a profile switch", async () => {
    const sources: FakeEventSource[] = [];
    const eventSourceFactory = vi.fn(() => {
      const source = new FakeEventSource();
      sources.push(source);
      return source;
    });
    lifecycleMocks.admit.mockReset();
    lifecycleMocks.close.mockReset().mockResolvedValue(undefined);
    lifecycleMocks.readProfile.mockReset().mockReturnValue("marketing");
    lifecycleMocks.open
      .mockReset()
      .mockImplementation(async (_workspace: string, profile: string) => openedSession(profile));

    function Harness({ renderKey }: { renderKey: string }) {
      const model = useCmdPaletteProgramView({
        client,
        dispatch,
        eventSourceFactory,
        onDismiss: vi.fn(),
        onQueryChange: vi.fn(),
        query: "",
        viewId: "ext.notes.browser",
      });
      return (
        <div data-render-key={renderKey}>
          {model.content.rows.map(row => (
            <div key={row.value}>{row.node}</div>
          ))}
        </div>
      );
    }

    const rendered = render(
      <UIProvider reducedMotion="always">
        <Harness renderKey="marketing" />
      </UIProvider>
    );
    await screen.findByText("marketing row");
    const marketingListener = sources[0]?.listeners.get("cmd_palette.view.frame");
    expect(marketingListener).toBeDefined();

    lifecycleMocks.readProfile.mockReturnValue("research");
    rendered.rerender(
      <UIProvider reducedMotion="always">
        <Harness renderKey="research" />
      </UIProvider>
    );
    await screen.findByText("research row");

    expect(lifecycleMocks.open.mock.calls.map(call => call[1])).toEqual(["marketing", "research"]);
    const marketingOpen = lifecycleMocks.open.mock.calls[0];
    expect(marketingOpen).toBeDefined();
    expect((marketingOpen![5] as AbortSignal).aborted).toBe(true);
    expect(lifecycleMocks.close).toHaveBeenCalledWith("view:marketing", "attachment-token");
    expect(lifecycleMocks.close.mock.invocationCallOrder[0]).toBeLessThan(
      lifecycleMocks.open.mock.invocationCallOrder[1]!
    );

    act(() => {
      invoke(
        marketingListener!,
        new MessageEvent("cmd_palette.view.frame", {
          data: JSON.stringify(frame("marketing", "stale marketing row")),
        })
      );
    });

    await waitFor(() => expect(screen.getByText("research row")).toBeVisible());
    expect(screen.queryByText("stale marketing row")).not.toBeInTheDocument();
  });
});
