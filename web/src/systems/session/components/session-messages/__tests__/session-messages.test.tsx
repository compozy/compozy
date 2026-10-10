import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, renderHook, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { SubagentNavigationContext } from "@/systems/session/contexts/session-subagents-context-value";
import { primarySessionFixture } from "@/systems/session/mocks/fixtures";
import {
  queuedInputOrigin,
  sessionMessageOrigin,
  sessionReplyMeta,
  sessionReplyOutcomes,
  sessionReplyText,
  sessionSentMessagePart,
} from "@/systems/session/lib/session-message-payload";
import type { QueuedPrompt } from "@/systems/session/lib/queued-prompt";

import { SessionQueueEntryRow } from "../../session-queue-strip-row";
import { SessionMessageCard } from "../session-message-card";
import type { SessionMessageParty } from "../session-message-party";
import { SessionReplyCard } from "../session-reply-card";
import { SessionSentCard, type SessionSentCardProps } from "../session-sent-card";
import { useSessionLabel } from "../use-session-label";
import { useSessionMessageOpen } from "../use-session-message-open";

// Suite: session message surfaces (`_uiux.md` S1–S4; UT-061..064, UT-066, UT-067).
// Invariant: the daemon's typed origin / synthetic / part facts map to one view model, and the
// cards render the sender or target, its link, the mode and reply chips, the outcome and the
// reply state from it; a deleted session reads "a deleted session" with no link; a session
// message queued for this session keeps Remove only. Owning layer: session-messages components
// and their adapter (props in, callbacks out). Canonical suite: this file (`_tests.md`).
// Boundary IN: session detail / workspace detail reads (fetch). Boundary OUT: openSession.

const WS = "ws_alpha";
const SENDER_ID = "sess-7f3a2c11d09e4b58";

const sender: SessionMessageParty = {
  sessionId: SENDER_ID,
  workspaceId: WS,
  title: "Refactor billing",
  agentName: "claude",
};

const target: SessionMessageParty = {
  sessionId: "sess-c03f9d61b2e84a07",
  workspaceId: WS,
  title: "Billing reviewer",
  agentName: "codex",
};

const AT = Date.parse("2026-10-09T22:30:00.000Z");

function sessionDetail(id: string, name: string, agent = "claude") {
  return { session: { ...primarySessionFixture, id, name, agent_name: agent, workspace_id: WS } };
}

/** Routes the detail reads the labels make; any other session answers 404 (deleted). */
function stubDetailReads(sessions: Record<string, ReturnType<typeof sessionDetail>>) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(input instanceof Request ? input.url : String(input), "http://x");
      const workspace = /^\/api\/workspaces\/([^/]+)$/.exec(url.pathname);
      if (workspace) {
        return Response.json({ workspace: { id: workspace[1], name: "compozy-site" } });
      }
      const id = /\/sessions\/([^/]+)$/.exec(url.pathname)?.[1] ?? "";
      const session = sessions[id];
      return session
        ? Response.json(session)
        : Response.json({ error: `Session not found: ${id}` }, { status: 404 });
    })
  );
}

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <QueryClientProvider client={client}>
      <UIProvider>{children}</UIProvider>
    </QueryClientProvider>
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("session message payload adapter", () => {
  it("Should read a session origin and ignore every other origin kind", () => {
    const origin = {
      kind: "session",
      session_id: SENDER_ID,
      workspace_id: WS,
      agent_name: "claude",
      title_at_send: "Refactor billing",
      hop: 1,
      notify_on_complete: true,
      reply_watch_id: "rw-6e2d81a0",
    };
    expect(sessionMessageOrigin({ custom: { origin } })).toEqual({
      sessionId: SENDER_ID,
      workspaceId: WS,
      agentName: "claude",
      titleAtSend: "Refactor billing",
      hop: 1,
      notifyOnComplete: true,
      replyWatchId: "rw-6e2d81a0",
    });
    expect(sessionMessageOrigin({ custom: { origin: { ...origin, kind: "goal" } } })).toBeNull();
    expect(sessionMessageOrigin({ custom: { turn_id: "turn-1" } })).toBeNull();
    expect(queuedInputOrigin({ id: "inp-1", origin })?.sessionId).toBe(SENDER_ID);
    expect(queuedInputOrigin({ id: "inp-2" })).toBeNull();
  });

  it("Should read only session_reply wakes and fold an unknown reason to unknown (UT-066)", () => {
    const synthetic = {
      kind: "session_reply",
      wake_event_id: "rw-6e2d81a0",
      child_session_id: target.sessionId,
      child_workspace_id: WS,
      child_agent_name: "codex",
      reason: "completed",
      reply_truncated: true,
      hop: 1,
    };
    expect(sessionReplyMeta({ custom: { synthetic } })).toEqual({
      watchId: "rw-6e2d81a0",
      targetSessionId: target.sessionId,
      targetWorkspaceId: WS,
      targetAgentName: "codex",
      outcome: "completed",
      truncated: true,
    });
    expect(
      sessionReplyMeta({ custom: { synthetic: { ...synthetic, reason: "exploded" } } })?.outcome
    ).toBe("unknown");
    expect(
      sessionReplyMeta({ custom: { synthetic: { ...synthetic, kind: "subagent_wake" } } })
    ).toBeNull();
    expect(sessionReplyMeta({ custom: { synthetic: { reason: "completed" } } })).toBeNull();
  });

  it("Should lift only the answer out of the reply wake text", () => {
    const header =
      'Session "Billing reviewer" (sess-c03f) replied to your message msg-1: completed.';
    expect(sessionReplyText(`${header}\n---\nPer job.\n\nShared counter.`)).toBe(
      "Per job.\n\nShared counter."
    );
    expect(
      sessionReplyText(
        `${header}\n---\nPer job.\n[Reply truncated at 12000 characters. Read the full turn with compozy__session_history.]`
      )
    ).toBe("Per job.");
    expect(sessionReplyText(`${header}\n---\n(no reply text)`)).toBe("");
  });

  it("Should map the sent part's mode and call state and match replies by watch id (UT-067)", () => {
    const part = sessionSentMessagePart("data-compozy-session-message", {
      tool_call_id: "call-1",
      target_session_id: target.sessionId,
      target_workspace_id: WS,
      message_id: "msg-retry-q",
      mode: "steer",
      reply_watch_id: "rw-6e2d81a0",
      state: "output-error",
    });
    expect(part).toMatchObject({ toolCallId: "call-1", mode: "steer", state: "failed" });
    expect(sessionSentMessagePart("data-compozy-subagent", { target_session_id: "x" })).toBeNull();

    const reply = (watch: string, reason: string) => ({
      role: "system",
      metadata: {
        custom: {
          synthetic: {
            kind: "session_reply",
            wake_event_id: watch,
            child_session_id: target.sessionId,
            reason,
          },
        },
      },
    });
    const outcomes = sessionReplyOutcomes([
      { role: "user", metadata: {} },
      reply("rw-a", "completed"),
      reply("rw-b", "dropped"),
    ]);
    expect(outcomes.get("rw-a")).toBe("completed");
    expect(outcomes.get("rw-b")).toBe("dropped");
    expect(outcomes.get("rw-c")).toBeUndefined();
  });
});

describe("SessionMessageCard", () => {
  it("Should name the article by its sender and show the mark, link, chips, time and body (UT-061)", () => {
    render(
      <SessionMessageCard
        sender={sender}
        delivery="steered"
        replyRequested
        timestampMs={AT}
        onOpenSender={vi.fn()}
      >
        Is the retry budget per request or per job?
      </SessionMessageCard>,
      { wrapper }
    );
    const card = screen.getByRole("article", { name: "Message from Refactor billing" });
    expect(card.querySelector('[data-kind="claude"]')).not.toBeNull();
    expect(within(card).getByText("From")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Refactor billing" })).toBeInTheDocument();
    expect(within(card).getByText("Steered")).toBeInTheDocument();
    expect(within(card).getByText("Reply requested")).toBeInTheDocument();
    expect(within(card).getByTestId("session-message-time")).toHaveAttribute(
      "datetime",
      "2026-10-09T22:30:00.000Z"
    );
    expect(within(card).getByText("Is the retry budget per request or per job?")).toBeVisible();
  });

  it("Should clamp a long body behind Show more and lift it on demand (UT-061)", async () => {
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(900);
    render(
      <SessionMessageCard sender={sender} delivery={null} replyRequested={false} timestampMs={AT}>
        {Array.from({ length: 30 }, (_, line) => `line ${line}`).join("\n")}
      </SessionMessageCard>,
      { wrapper }
    );
    const body = screen.getByTestId("session-message-body");
    expect(body).toHaveAttribute("data-clamped", "true");
    await userEvent.click(screen.getByRole("button", { name: "Show more" }));
    expect(body).not.toHaveAttribute("data-clamped");
    expect(screen.getByRole("button", { name: "Show less" })).toHaveAttribute(
      "aria-expanded",
      "true"
    );
  });

  it("Should open the sender in this window, or a new one with ⌘ (UT-062)", () => {
    const onOpen = vi.fn();
    render(
      <SessionMessageCard
        sender={sender}
        delivery={null}
        replyRequested={false}
        timestampMs={AT}
        onOpenSender={onOpen}
      >
        Hi
      </SessionMessageCard>,
      { wrapper }
    );
    const link = screen.getByRole("button", { name: "Refactor billing" });
    fireEvent.click(link);
    fireEvent.click(link, { metaKey: true });
    expect(onOpen).toHaveBeenNthCalledWith(1, sender, { newWindow: false });
    expect(onOpen).toHaveBeenNthCalledWith(2, sender, { newWindow: true });
  });

  it("Should route the open through the window's openSession with the party's workspace (UT-062)", () => {
    const openSession = vi.fn();
    const { result } = renderHook(() => useSessionMessageOpen(), {
      wrapper: ({ children }) => (
        <SubagentNavigationContext value={{ openSession }}>{children}</SubagentNavigationContext>
      ),
    });
    result.current?.({ ...sender, workspaceId: "ws_site" }, { newWindow: true });
    expect(openSession).toHaveBeenCalledWith(
      { sessionId: SENDER_ID, agentName: "claude", workspaceId: "ws_site" },
      { newWindow: true }
    );
    expect(renderHook(() => useSessionMessageOpen()).result.current).toBeUndefined();
  });

  it("Should read a deleted sender plainly with no link (UT-063)", () => {
    render(
      <SessionMessageCard
        sender={{ ...sender, title: null }}
        delivery={null}
        replyRequested={false}
        timestampMs={AT}
        onOpenSender={vi.fn()}
      >
        Hi
      </SessionMessageCard>,
      { wrapper }
    );
    const card = screen.getByRole("article", { name: "Message from a deleted session" });
    expect(within(card).getByText("a deleted session")).toBeInTheDocument();
    expect(within(card).queryByRole("button")).not.toBeInTheDocument();
  });
});

describe("useSessionLabel", () => {
  beforeEach(() => {
    stubDetailReads({ [SENDER_ID]: sessionDetail(SENDER_ID, "Billing refactor · PR split") });
  });

  it("Should show the current title once the detail read lands, not the recorded one (UT-063)", async () => {
    const { result } = renderHook(
      () =>
        useSessionLabel({
          sessionId: SENDER_ID,
          workspaceId: WS,
          currentWorkspaceId: WS,
          agentName: "claude",
          titleAtSend: "Refactor billing",
        }),
      { wrapper }
    );
    expect(result.current.title).toBe("Refactor billing");
    await waitFor(() => expect(result.current.title).toBe("Billing refactor · PR split"));
    expect(result.current.workspaceName).toBeNull();
  });

  it("Should read a 404 as a deleted session (UT-063)", async () => {
    const { result } = renderHook(
      () =>
        useSessionLabel({
          sessionId: "sess-gone",
          workspaceId: WS,
          currentWorkspaceId: WS,
          agentName: "claude",
          titleAtSend: "Docs sweep",
        }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.title).toBeNull());
    expect(result.current.agentName).toBe("claude");
  });

  it("Should name a sender in another workspace and keep that workspace (UT-063)", async () => {
    const { result } = renderHook(
      () =>
        useSessionLabel({
          sessionId: SENDER_ID,
          workspaceId: "ws_site",
          currentWorkspaceId: WS,
          agentName: "claude",
        }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.workspaceName).toBe("compozy-site"));
    expect(result.current.workspaceId).toBe("ws_site");
  });
});

describe("SessionReplyCard", () => {
  it("Should show the outcome word and the answer, with a truncation note (UT-066)", () => {
    render(
      <SessionReplyCard
        target={target}
        outcome="completed"
        text="Per job."
        truncated
        timestampMs={AT}
        onOpenTarget={vi.fn()}
      />,
      { wrapper }
    );
    const card = screen.getByRole("article", {
      name: "Reply from Billing reviewer, completed, truncated",
    });
    expect(within(card).getByTestId("session-reply-outcome")).toHaveTextContent("Completed");
    expect(within(card).getByText("Per job.")).toBeInTheDocument();
    expect(within(card).getByTestId("session-reply-truncated")).toHaveTextContent(
      "Reply truncated"
    );
    expect(within(card).getByRole("button", { name: "Read the full turn" })).toBeInTheDocument();
  });

  it.each([
    ["completed", "", "No reply text."],
    ["dropped", "ignored", "The message was removed from the queue before it ran."],
    ["unknown", "", "The message may not have been delivered; check the target session."],
    [
      "failed",
      "Provider error: rate limit exceeded (429).",
      "Provider error: rate limit exceeded (429).",
    ],
  ] as const)(
    "Should render the %s body as its fixed sentence or text (UT-066)",
    (outcome, text, body) => {
      render(
        <SessionReplyCard
          target={target}
          outcome={outcome}
          text={text}
          truncated={false}
          timestampMs={AT}
        />,
        { wrapper }
      );
      expect(screen.getByTestId("session-message-body")).toHaveTextContent(body);
    }
  );
});

describe("SessionSentCard", () => {
  const base: SessionSentCardProps = {
    target,
    callState: "sent",
    mode: "queue",
    firstLine: "Is the retry budget per request or per job?",
    error: null,
    reply: "waiting",
    timestampMs: AT,
  };

  it.each([
    ["waiting", "Waiting for reply", "Sent to Billing reviewer, waiting for reply"],
    ["completed", "Replied", "Sent to Billing reviewer, replied"],
    ["failed", "Failed", "Sent to Billing reviewer, failed"],
    ["canceled", "Canceled", "Sent to Billing reviewer, canceled"],
    ["dropped", "Dropped", "Sent to Billing reviewer, dropped"],
  ] as const)("Should read the %s reply state as words (UT-067)", (reply, word, name) => {
    render(<SessionSentCard {...base} reply={reply} />, { wrapper });
    const card = screen.getByRole("article", { name });
    expect(within(card).getByTestId("session-sent-reply-state")).toHaveTextContent(word);
  });

  it("Should show no reply state when none was asked for, and the chip for a steer (UT-067)", () => {
    render(<SessionSentCard {...base} reply="none" mode="steer" />, { wrapper });
    expect(screen.getByRole("article", { name: "Sent to Billing reviewer" })).toBeInTheDocument();
    expect(screen.queryByTestId("session-sent-reply-state")).not.toBeInTheDocument();
    expect(screen.getByText("Steered")).toBeInTheDocument();
  });

  it("Should read a failed call as Could not send with the error in place of the message (UT-067)", () => {
    render(
      <SessionSentCard
        {...base}
        callState="failed"
        error="Message chain limit reached (8 hops). Ask the operator to continue."
      />,
      { wrapper }
    );
    const card = screen.getByRole("article", { name: "Could not send to Billing reviewer" });
    expect(within(card).getByTestId("session-sent-line-two")).toHaveTextContent(
      "Message chain limit reached (8 hops)."
    );
    expect(within(card).queryByTestId("session-sent-reply-state")).not.toBeInTheDocument();
  });
});

describe("SessionQueueEntryRow session message", () => {
  const prompt: QueuedPrompt = {
    id: "inp-41aa",
    owner: null,
    position: 2,
    status: "queued",
    text: "Is the retry budget in billing.toml per request or per job?",
    sender: {
      sessionId: SENDER_ID,
      workspaceId: WS,
      agentName: "claude",
      titleAtSend: "Refactor billing",
      hop: 1,
      notifyOnComplete: true,
      replyWatchId: "rw-6e2d81a0",
    },
  };

  it("Should show the sender and keep Remove only (UT-064)", async () => {
    stubDetailReads({ [SENDER_ID]: sessionDetail(SENDER_ID, "Refactor billing") });
    const onRemove = vi.fn();
    render(
      <SessionQueueEntryRow
        prompt={prompt}
        disabled={false}
        actionsHidden={false}
        onSteer={vi.fn()}
        onEdit={vi.fn()}
        onRemove={onRemove}
      />,
      { wrapper }
    );
    const row = screen.getByTestId("composer-queued-prompt-row");
    expect(row).toHaveAttribute("data-origin", "session");
    expect(within(row).getByTestId("composer-queued-sender")).toHaveTextContent(
      /From\s*Refactor billing$/
    );
    expect(within(row).queryByTestId("composer-queued-edit")).not.toBeInTheDocument();
    expect(within(row).queryByTestId("composer-queued-steer")).not.toBeInTheDocument();
    await userEvent.click(
      within(row).getByRole("button", { name: "Remove message from Refactor billing" })
    );
    expect(onRemove).toHaveBeenCalledWith("inp-41aa");
  });

  it("Should read a deleted sender in the row (UT-064)", async () => {
    stubDetailReads({});
    render(
      <SessionQueueEntryRow
        prompt={prompt}
        disabled={false}
        actionsHidden={false}
        onSteer={vi.fn()}
        onRemove={vi.fn()}
      />,
      { wrapper }
    );
    await waitFor(() =>
      expect(screen.getByTestId("composer-queued-sender")).toHaveTextContent(
        /From\s*a deleted session$/
      )
    );
    expect(
      screen.getByRole("button", { name: "Remove message from a deleted session" })
    ).toBeInTheDocument();
  });
});
