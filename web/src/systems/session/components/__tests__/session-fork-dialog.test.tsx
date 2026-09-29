// Suite: Fork dialog (S5)
// Invariant: the dialog forks the same agent (shown read-only), names the fork point, states
// what travels from the daemon's own preview (including the native-clone decision, never
// guessed), refuses an unsettled cut and a changed transcript truthfully, and posts exactly
// the fork the operator saw.
// Owning layer: SessionForkDialog with its hooks over MSW, plus the shared derive host that
// opens it. Canonical suite: this file.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { HttpHandler } from "msw";
import { use } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";
import { createMswFetch } from "@/test/msw-fetch";

import type { ForkSessionRequest } from "../../adapters/session-derive-api";
import {
  SessionForkContext,
  type SessionForkPoint,
} from "../../contexts/session-fork-context-value";
import { useSessionDeriveHost } from "../../hooks/use-session-derive-host";
import {
  deriveSourceSessionFixture,
  forkCutPreviewFixture,
  forkedSessionFixture,
  forkNativePreviewFixture,
  forkPointFixture,
  forkResultFixture,
  forkUnsettledPreviewFixture,
} from "../../mocks/derive-fixtures";
import {
  sessionDeriveHandlers,
  type SessionDeriveHandlerOptions,
} from "../../mocks/derive-handlers";
import type { SessionPayload } from "../../types";
import { SessionDeriveHost } from "../session-derive-host";
import { SessionForkDialog } from "../session-fork-dialog";

const source: SessionPayload = {
  ...deriveSourceSessionFixture,
  agent_name: "codex",
  runtime: {
    ...deriveSourceSessionFixture.runtime,
    effective: { ...deriveSourceSessionFixture.runtime.effective!, provider: "claude" },
  },
};
const workspaceId = source.workspace_id ?? "";
let handlers: HttpHandler[] = [];

function queryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function renderDialog(
  options: SessionDeriveHandlerOptions = {},
  point: SessionForkPoint | null = null
) {
  handlers = sessionDeriveHandlers(options);
  const openInNewWindow = vi.fn<(child: SessionPayload) => void>();
  const openInThisWindow = vi.fn<(child: SessionPayload) => void>();
  const onOpenChange = vi.fn();
  render(
    <QueryClientProvider client={queryClient()}>
      <UIProvider reducedMotion="never" skipAnimations>
        <SessionForkDialog
          onOpenChange={onOpenChange}
          open
          placement={{ openInNewWindow, openInThisWindow }}
          point={point}
          source={source}
          workspaceId={workspaceId}
        />
      </UIProvider>
    </QueryClientProvider>
  );
  return { openInNewWindow, openInThisWindow, onOpenChange };
}

async function measured() {
  await waitFor(() =>
    expect(screen.getByTestId("session-derive-preview")).toHaveAttribute("data-state", "ready")
  );
}

describe("SessionForkDialog", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // UT-067: whole session — locked agent, preview, no message_id on the wire.
  it("Should fork the whole session with the agent locked and open the child", async () => {
    const user = userEvent.setup();
    const onFork = vi.fn<(request: ForkSessionRequest) => void>();
    const child = forkedSessionFixture(source);
    const { openInNewWindow, openInThisWindow, onOpenChange } = renderDialog({
      onFork,
      result: forkResultFixture(child),
    });

    expect(screen.getByTestId("session-fork-submit")).toBeDisabled();
    await measured();
    expect(screen.getByTestId("session-fork-agent")).toHaveTextContent("codex·claude");
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-fork-point")).toHaveTextContent("Whole session");
    expect(screen.queryByTestId("session-fork-point-change")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-derive-preview")).toHaveTextContent(
      "Carries over 42 messages · 61.3 KiB"
    );
    expect(screen.queryByTestId("session-derive-preview-native")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-fork-source-note")).toHaveTextContent(
      "From Refactor flaky manager tests"
    );

    await user.click(screen.getByTestId("session-fork-submit"));

    await waitFor(() => expect(openInNewWindow).toHaveBeenCalledExactlyOnceWith(child));
    expect(openInThisWindow).not.toHaveBeenCalled();
    expect(onOpenChange).toHaveBeenCalledWith(false);
    const request = onFork.mock.calls[0]![0];
    expect(request).not.toHaveProperty("message_id");
    expect(request.idempotency_key).toMatch(/^web-derive-/);
    expect(request).toMatchObject({
      expected_epoch: 1,
      expected_generation: 3,
      expected_max_sequence: 318,
    });
  });

  // UT-067: the native line is the daemon's decision, present only on `native_fork_possible`.
  it("Should state the native clone only when the preview reports it", async () => {
    renderDialog({ preview: forkNativePreviewFixture });

    await measured();
    expect(screen.getByTestId("session-derive-preview")).toHaveAttribute("data-native", "true");
    expect(screen.getByTestId("session-derive-preview-native")).toHaveTextContent(
      "Uses the agent's own session clone."
    );
  });

  // UT-067 (fork from here): the point names the message; its id reaches preview and fork.
  it("Should fork through the clicked message and quote its first 60 characters", async () => {
    const user = userEvent.setup();
    const onFork = vi.fn<(request: ForkSessionRequest) => void>();
    const onPreview = vi.fn<(messageId: string | null) => void>();
    const { onOpenChange } = renderDialog(
      {
        preview: forkCutPreviewFixture,
        onFork,
        onPreview,
        result: forkResultFixture(forkedSessionFixture(source, forkPointFixture.messageId)),
      },
      forkPointFixture
    );

    await measured();
    expect(onPreview).toHaveBeenCalledWith(forkPointFixture.messageId);
    expect(screen.getByTestId("session-fork-point")).toHaveTextContent(
      "Through Refactor the flaky manager tests so they wait on the lifecyc…"
    );
    expect(screen.getByTestId("session-derive-preview")).toHaveTextContent(
      "Carries over 18 of 42 messages · 24.1 KiB"
    );
    expect(screen.queryByTestId("session-derive-preview-omitted")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("session-fork-submit"));
    await waitFor(() => expect(onFork).toHaveBeenCalledOnce());
    expect(onFork.mock.calls[0]![0].message_id).toBe(forkPointFixture.messageId);
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
  });

  it("Should close on Change: the fork point is chosen on the message", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderDialog({ preview: forkCutPreviewFixture }, forkPointFixture);

    await user.click(screen.getByTestId("session-fork-point-change"));
    expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
  });

  // UT-069: an unsettled cut disables Fork session and says so in the line's place.
  it("Should refuse an unsettled cut before anything is posted", async () => {
    const onFork = vi.fn();
    renderDialog({ preview: forkUnsettledPreviewFixture, onFork }, forkPointFixture);

    await waitFor(() =>
      expect(screen.getByTestId("session-derive-preview")).toHaveTextContent(
        "That turn hasn't settled yet."
      )
    );
    expect(screen.getByTestId("session-derive-preview")).toHaveAttribute("data-state", "error");
    expect(screen.getByTestId("session-fork-submit")).toBeDisabled();
    expect(onFork).not.toHaveBeenCalled();
  });

  // UT-069: the daemon's `session_turn_in_progress` on submit renders the same line.
  it("Should render the daemon's turn-in-progress refusal as the unsettled line", async () => {
    const user = userEvent.setup();
    renderDialog(
      {
        preview: forkCutPreviewFixture,
        result: {
          status: 409,
          code: "session_turn_in_progress",
          error: "the turn started by msg has not settled; fork from an earlier message or wait",
        },
      },
      forkPointFixture
    );

    await measured();
    await user.click(screen.getByTestId("session-fork-submit"));
    await waitFor(() =>
      expect(screen.getByTestId("session-derive-preview")).toHaveTextContent(
        "That turn hasn't settled yet."
      )
    );
    expect(screen.getByTestId("session-fork-submit")).toBeDisabled();
  });

  // UT-069: a fence conflict after opening is terminal for this open — reopen to refresh.
  it("Should refuse a changed transcript and offer only Close", async () => {
    const user = userEvent.setup();
    const { openInNewWindow } = renderDialog(
      {
        preview: forkCutPreviewFixture,
        result: {
          status: 409,
          code: "session_fence_conflict",
          error: "transcript changed since the fences were read",
        },
      },
      forkPointFixture
    );

    await measured();
    await user.click(screen.getByTestId("session-fork-submit"));

    expect(await screen.findByTestId("session-fork-submit-error")).toHaveTextContent(
      "Transcript changed — reopen to fork from the current state."
    );
    expect(screen.getByTestId("session-fork-submit")).toBeDisabled();
    const close = screen.getByTestId("session-fork-cancel");
    expect(close).toHaveTextContent("Close");
    expect(close).toBeEnabled();
    expect(openInNewWindow).not.toHaveBeenCalled();
  });

  it("Should show any other refusal verbatim and keep Fork session available", async () => {
    const user = userEvent.setup();
    renderDialog(
      {
        preview: forkCutPreviewFixture,
        result: {
          status: 404,
          code: "message_not_found",
          error: "message msg_01J9R3ZQ8PVX not found in session sess_source",
        },
      },
      forkPointFixture
    );

    await measured();
    await user.click(screen.getByTestId("session-fork-submit"));
    expect(await screen.findByTestId("session-fork-submit-error")).toHaveTextContent(
      "message msg_01J9R3ZQ8PVX not found in session sess_source"
    );
    expect(screen.getByTestId("session-fork-submit")).toBeEnabled();
  });

  it("Should leave the fences out for a running source", async () => {
    const user = userEvent.setup();
    const onFork = vi.fn<(request: ForkSessionRequest) => void>();
    renderDialog({
      preview: { ...forkNativePreviewFixture, source_turn_in_progress: true },
      onFork,
    });

    await measured();
    await user.click(screen.getByTestId("session-fork-submit"));
    await waitFor(() => expect(onFork).toHaveBeenCalledOnce());
    expect(onFork.mock.calls[0]![0]).not.toHaveProperty("expected_max_sequence");
  });
});

function ForkEntry() {
  const requestFork = use(SessionForkContext);
  return (
    <button onClick={() => requestFork?.()} type="button">
      Fork entry
    </button>
  );
}

function HostHarness() {
  const host = useSessionDeriveHost({ workspaceId, currentSession: source });
  return (
    <SessionDeriveHost host={host} openInNewWindow={vi.fn()}>
      <ForkEntry />
    </SessionDeriveHost>
  );
}

describe("SessionDeriveHost fork", () => {
  beforeEach(() => {
    handlers = sessionDeriveHandlers();
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // UT-074: a menu's fork request opens the Fork dialog on the whole session.
  it("Should open the Fork dialog on the whole session for a source-less request", async () => {
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={queryClient()}>
        <UIProvider reducedMotion="never" skipAnimations>
          <HostHarness />
        </UIProvider>
      </QueryClientProvider>
    );

    await user.click(screen.getByRole("button", { name: "Fork entry" }));

    expect(await screen.findByTestId("session-fork-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("session-fork-point")).toHaveTextContent("Whole session");
    expect(screen.queryByTestId("session-continue-dialog")).not.toBeInTheDocument();
  });
  // Invariant: readiness belongs to this opening's own measurement. A reopened dialog
  // must not show the previous open's size or submit its fences while it remeasures.
  it("Should keep a reopened dialog measuring until its own preview answers", async () => {
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={queryClient()}>
        <UIProvider reducedMotion="never" skipAnimations>
          <HostHarness />
        </UIProvider>
      </QueryClientProvider>
    );

    await user.click(screen.getByRole("button", { name: "Fork entry" }));
    await measured();
    expect(screen.getByTestId("session-fork-submit")).toBeEnabled();
    await user.click(screen.getByTestId("session-fork-cancel"));

    handlers = sessionDeriveHandlers({ preview: "pending" });
    await user.click(screen.getByRole("button", { name: "Fork entry" }));

    const preview = await screen.findByTestId("session-derive-preview");
    expect(preview).toHaveAttribute("data-state", "measuring");
    expect(screen.getByTestId("session-fork-submit")).toBeDisabled();
  });
});
