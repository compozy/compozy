import { composeStories, setProjectAnnotations } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider, UIProvider } from "@compozy/ui";

import { storybookSystemHandlers, type StorybookHandlerOverrides } from "@/storybook/msw";

import * as stories from "../session-thread-queue.stories";

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() },
}));

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, children, ...props }: Record<string, unknown>) => (
      <a href={typeof to === "string" ? to : "#"} {...(props as Record<string, unknown>)}>
        {children as React.ReactNode}
      </a>
    ),
  };
});

// Suite: queue story scenes (task_10 visual readiness for task_03 VC-01..06).
// Invariant: every queue story reaches the contract state it names through the production
// wiring (`useSessionPageControls` → `SessionThread` → `SessionQueueStrip` + composer) against
// the scene's MSW queue — the composer is live (never "Session is not active"), the busy
// controls and the runtime footer are present, and each `play` completes with its assertions.
// Owning layer: session thread stories (portable stories in jsdom). Canonical suite: this file.
// Boundary IN: scene handlers (msw/node). Boundary OUT: rendered strip/composer state.
setProjectAnnotations([
  {
    decorators: [
      Story => (
        <QueryClientProvider
          client={
            new QueryClient({
              defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
            })
          }
        >
          <UIProvider>
            <TooltipProvider delay={0}>
              <Story />
            </TooltipProvider>
          </UIProvider>
        </QueryClientProvider>
      ),
    ],
  },
]);

const composed = composeStories(stories);
const server = setupServer(...storybookSystemHandlers);

function storyHandlers(story: { parameters: Record<string, unknown> }) {
  const msw = story.parameters.msw as { handlers?: StorybookHandlerOverrides } | undefined;
  return Object.values(msw?.handlers ?? {}).flat();
}

async function mountStory(Story: (typeof composed)[keyof typeof composed]) {
  server.use(...storyHandlers(Story));
  // The meta loaders run as in Storybook: the session's persisted draft is discarded.
  await Story.load();
  const view = render(<Story />);
  // The composer is live: the session is public, active and mid-turn.
  await waitFor(() =>
    expect(within(view.container).getByLabelText("Session prompt")).not.toHaveAttribute("inert")
  );
  expect(within(view.container).queryByText("Session is not active")).not.toBeInTheDocument();
  await Story.play?.({ canvasElement: view.container });
  return view;
}

// jsdom ships no `ClipboardEvent`; the plays paste the draft (Lexical reads the
// event's `clipboardData`), so the harness gives it the DOM shape browsers have.
class ClipboardEventShim extends Event {
  readonly clipboardData: DataTransfer | null;
  constructor(type: string, init: ClipboardEventInit = {}) {
    super(type, init);
    this.clipboardData = init.clipboardData ?? null;
  }
}

describe("SessionThread queue stories", () => {
  beforeAll(() => server.listen({ onUnhandledRequest: "bypass" }));
  // Stubs are dropped after every case (`unstubAllGlobals`), so both go in per case.
  beforeEach(() => {
    vi.stubGlobal("scrollTo", vi.fn());
    vi.stubGlobal("ClipboardEvent", ClipboardEventShim);
  });
  afterEach(() => {
    cleanup();
    server.resetHandlers();
    vi.unstubAllGlobals();
  });
  afterAll(() => server.close());

  it("Should render the populated strip with live verbs and the busy controls (VC-01)", async () => {
    await mountStory(composed.Populated);
    expect(await screen.findAllByTestId("composer-queued-prompt-row")).toHaveLength(3);
    expect(screen.getByTestId("composer-queue-count")).toHaveTextContent("3");
    expect(screen.getByTestId("composer-queue-clear")).toBeInTheDocument();
    expect(screen.getAllByTestId("composer-queued-edit")).toHaveLength(3);
    expect(screen.getByTestId("composer-queue-button")).toBeInTheDocument();
    expect(screen.getByTestId("composer-steer-button")).toBeInTheDocument();
    expect(screen.getByTestId("composer-interrupt-button")).toBeInTheDocument();
    expect(screen.getByTestId("composer-stop-button")).toBeInTheDocument();
  });

  it("Should open the in-row editor on a parked own row (VC-01 §07)", async () => {
    await mountStory(composed.EditingOwnRow);
    expect(screen.getByTestId("composer-queued-editor")).toHaveValue("Ship it with tests");
  });

  it("Should attribute another actor's rows and drop their verbs (VC-02)", async () => {
    await mountStory(composed.OtherActor);
    const rows = screen.getAllByTestId("composer-queued-prompt-row");
    expect(rows).toHaveLength(3);
    expect(within(rows[0]!).getByTestId("composer-queued-edit")).toBeInTheDocument();
    expect(within(rows[1]!).queryByTestId("composer-queued-edit")).not.toBeInTheDocument();
    expect(within(rows[1]!).getByTestId("composer-queued-owner")).toHaveTextContent("reviewer");
    expect(within(rows[2]!).queryByTestId("composer-queued-steer")).not.toBeInTheDocument();
  });

  it("Should refuse the edit of a dispatching head entry and hand the text to the composer (VC-03)", async () => {
    await mountStory(composed.DispatchingEditRefused);
    expect(screen.getByTestId("composer-feedback-note")).toHaveTextContent("entry_dispatching");
    const rows = screen.getAllByTestId("composer-queued-prompt-row");
    expect(within(rows[0]!).getByTestId("composer-queued-state")).toHaveTextContent("Sending…");
    expect(within(rows[0]!).queryByTestId("composer-queued-edit")).not.toBeInTheDocument();
  });

  it("Should keep a lost queue send as the unconfirmed row with Retry (VC-04)", async () => {
    await mountStory(composed.Unconfirmed);
    expect(screen.getByTestId("composer-queued-state")).toHaveTextContent("Not confirmed");
    expect(screen.getByTestId("composer-queued-discard")).toBeInTheDocument();
    expect(screen.getAllByTestId("composer-queued-prompt-row")).toHaveLength(2);
  });

  it("Should resolve the unconfirmed row when Retry replays the same identity (VC-04)", async () => {
    await mountStory(composed.UnconfirmedRetryReplayed);
    expect(screen.queryByTestId("composer-queued-retry")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-feedback-note")).toHaveTextContent(
      "nothing was sent twice"
    );
  });

  it("Should read full at cap and drop the queue affordance (VC-05)", async () => {
    await mountStory(composed.Full);
    expect(screen.getByTestId("composer-queue-full")).toBeInTheDocument();
    expect(screen.getByTestId("composer-queue-count")).toHaveTextContent("10");
    expect(screen.queryByTestId("composer-queue-button")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-steer-button")).toBeInTheDocument();
    expect(screen.getByTestId("composer-enter-hint")).not.toHaveTextContent("queue");
  });

  it("Should retain a refused queue draft when a concurrent enqueue fills capacity", async () => {
    await mountStory(composed.FullRefusedDraft);
    expect(screen.getByTestId("composer-feedback-note")).toHaveTextContent("10 of 10");
    expect(screen.getByLabelText("Session prompt")).toHaveTextContent(
      "Also verify the final migration before shipping"
    );
  });

  it("Should ask inline before clearing, clear on confirm, and show the trace (VC-06)", async () => {
    await mountStory(composed.ClearConfirm);
    expect(screen.getByTestId("composer-queue-clear-confirm")).toHaveTextContent("3");
    cleanup();
    server.resetHandlers();

    await mountStory(composed.Cleared);
    expect(screen.queryByTestId("composer-queued-prompts")).not.toBeInTheDocument();
    cleanup();
    server.resetHandlers();

    await mountStory(composed.ClearedTrace);
    // Three same-kind markers cluster into one line carrying the count.
    const trace = await screen.findByTestId("transcript-marker-summary");
    expect(trace).toHaveTextContent("You cleared the queue");
    expect(screen.getByTestId("transcript-marker-notice")).toHaveTextContent("3");
    expect(screen.queryByTestId("composer-queued-prompts")).not.toBeInTheDocument();
  });
});
