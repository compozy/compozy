import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-clarification-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/session-clarification-api")>();
  return {
    ...actual,
    answerSessionClarification: vi.fn(),
    fetchSessionClarifications: vi.fn(),
  };
});

import { SessionApiError } from "../../adapters/session-api-errors";
import {
  answerSessionClarification,
  ClarificationNotAnswerableError,
  fetchSessionClarifications,
} from "../../adapters/session-clarification-api";
import type { AgentEventPayload, ClarificationPending, ClarifyStatus } from "../../types";
import { ClarificationDataPart } from "../clarification-data-part";

const WORKSPACE_ID = "ws_alpha";
const SESSION_ID = "sess-001";

const choiceLive: ClarificationPending = {
  agent_name: "mock",
  asked_at: "2026-07-16T00:00:00Z",
  choices: ["Fast", "Safe"],
  deadline: "2026-07-16T00:05:00Z",
  question: "Which path should the migration take?",
  request_id: "req-choice",
  session_id: SESSION_ID,
};

const textLive: ClarificationPending = {
  agent_name: "mock",
  asked_at: "2026-07-16T00:00:00Z",
  deadline: "2026-07-16T00:05:00Z",
  question: "Describe the rollback plan",
  request_id: "req-text",
  session_id: SESSION_ID,
};

function clarifyEvent(
  status: ClarifyStatus,
  request: ClarificationPending,
  answer?: { choice: number | null; text: string; fallback: boolean }
): AgentEventPayload {
  return {
    type: "clarify",
    session_id: SESSION_ID,
    turn_id: `clarify:${request.request_id}`,
    raw: { status, request, at: "2026-07-16T00:00:10Z", ...(answer ? { answer } : {}) },
  };
}

function renderPart(data: AgentEventPayload): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <ClarificationDataPart data={data} sessionId={SESSION_ID} workspaceId={WORKSPACE_ID} />
    </QueryClientProvider>
  );
}

beforeEach(() => {
  vi.mocked(fetchSessionClarifications).mockResolvedValue([choiceLive]);
  vi.mocked(answerSessionClarification).mockResolvedValue({
    choice: null,
    fallback: false,
    text: "",
  });
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("ClarificationDataPart — pending", () => {
  it("Should render the choice card only after the live GET confirms the request", async () => {
    renderPart(clarifyEvent("pending", choiceLive));

    const card = await screen.findByTestId("clarification-card");
    expect(within(card).getByTestId("clarification-question")).toHaveTextContent(
      choiceLive.question
    );
    expect(screen.getAllByTestId("clarification-choice")).toHaveLength(2);
  });

  it("Should never render a card for a pending event absent from live truth", async () => {
    vi.mocked(fetchSessionClarifications).mockResolvedValue([]);
    renderPart(clarifyEvent("pending", choiceLive));

    await waitFor(() => expect(fetchSessionClarifications).toHaveBeenCalled());
    expect(screen.queryByTestId("clarification-card")).not.toBeInTheDocument();
  });

  it("Should preserve the pending question and retry when the live GET fails", async () => {
    vi.mocked(fetchSessionClarifications).mockRejectedValueOnce(
      new SessionApiError("broker unavailable", 503, SESSION_ID)
    );
    renderPart(clarifyEvent("pending", choiceLive));

    const error = await screen.findByTestId("clarification-load-error");
    expect(within(error).getByText(choiceLive.question)).toBeInTheDocument();

    fireEvent.click(within(error).getByRole("button", { name: "Retry clarification" }));

    expect(await screen.findByTestId("clarification-card")).toBeInTheDocument();
    expect(fetchSessionClarifications).toHaveBeenCalledTimes(2);
  });

  it("Should submit the zero-based choice index and disable controls while submitting", async () => {
    let resolveAnswer: (() => void) | undefined;
    vi.mocked(answerSessionClarification).mockReturnValue(
      new Promise(resolve => {
        resolveAnswer = () => resolve({ choice: 1, fallback: false, text: "" });
      })
    );
    renderPart(clarifyEvent("pending", choiceLive));

    const choices = await screen.findAllByTestId("clarification-choice");
    fireEvent.click(choices[1]!);

    await waitFor(() => {
      expect(screen.getAllByTestId("clarification-choice")[0]).toBeDisabled();
    });
    fireEvent.click(choices[0]!);

    expect(answerSessionClarification).toHaveBeenCalledTimes(1);
    expect(answerSessionClarification).toHaveBeenCalledWith(
      WORKSPACE_ID,
      SESSION_ID,
      "req-choice",
      {
        choice_index: 1,
      }
    );
    resolveAnswer?.();
  });

  it("Should hide the card after a successful answer", async () => {
    renderPart(clarifyEvent("pending", choiceLive));

    const choices = await screen.findAllByTestId("clarification-choice");
    fireEvent.click(choices[0]!);

    await waitFor(() => {
      expect(screen.queryByTestId("clarification-card")).not.toBeInTheDocument();
    });
  });

  it("Should keep the question answerable and surface an inline error after a retryable failure", async () => {
    vi.mocked(answerSessionClarification).mockRejectedValue(
      new SessionApiError("boom", 503, SESSION_ID)
    );
    renderPart(clarifyEvent("pending", choiceLive));

    const choices = await screen.findAllByTestId("clarification-choice");
    fireEvent.click(choices[0]!);

    const error = await screen.findByTestId("clarification-error");
    expect(error).toBeInTheDocument();
    expect(screen.getByTestId("clarification-card")).toBeInTheDocument();
    expect(screen.getAllByTestId("clarification-choice")[0]).not.toBeDisabled();
  });

  it("Should hide the card when the request is no longer answerable", async () => {
    vi.mocked(answerSessionClarification).mockRejectedValue(
      new ClarificationNotAnswerableError(SESSION_ID, "req-choice")
    );
    renderPart(clarifyEvent("pending", choiceLive));

    const choices = await screen.findAllByTestId("clarification-choice");
    fireEvent.click(choices[0]!);

    await waitFor(() => {
      expect(screen.queryByTestId("clarification-card")).not.toBeInTheDocument();
    });
    expect(screen.queryByTestId("clarification-error")).not.toBeInTheDocument();
  });

  it("Should submit trimmed free text for a choiceless question", async () => {
    vi.mocked(fetchSessionClarifications).mockResolvedValue([textLive]);
    renderPart(clarifyEvent("pending", textLive));

    const input = await screen.findByTestId("clarification-text-input");
    fireEvent.change(input, { target: { value: "  roll back to v3  " } });
    fireEvent.click(screen.getByTestId("clarification-submit"));

    await waitFor(() => {
      expect(answerSessionClarification).toHaveBeenCalledWith(
        WORKSPACE_ID,
        SESSION_ID,
        "req-text",
        {
          text: "roll back to v3",
        }
      );
    });
  });
});

describe("ClarificationDataPart — terminal receipts", () => {
  it("Should render a resolved receipt with the chosen answer and no controls", () => {
    renderPart(clarifyEvent("resolved", choiceLive, { choice: 0, fallback: false, text: "" }));

    const receipt = screen.getByTestId("clarification-receipt");
    expect(receipt).toHaveAttribute("data-status", "resolved");
    expect(within(receipt).getByTestId("clarification-receipt-answer")).toHaveTextContent("Fast");
    expect(within(receipt).queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByTestId("clarification-text-input")).not.toBeInTheDocument();
  });

  it("Should render a timed-out receipt without inventing an answer", () => {
    renderPart(clarifyEvent("timed_out", choiceLive, { choice: null, fallback: true, text: "" }));

    const receipt = screen.getByTestId("clarification-receipt");
    expect(receipt).toHaveAttribute("data-status", "timed_out");
    expect(screen.queryByTestId("clarification-receipt-answer")).not.toBeInTheDocument();
    expect(within(receipt).queryByRole("button")).not.toBeInTheDocument();
  });

  it("Should render a canceled receipt", () => {
    renderPart(clarifyEvent("canceled", choiceLive));

    const receipt = screen.getByTestId("clarification-receipt");
    expect(receipt).toHaveAttribute("data-status", "canceled");
    expect(within(receipt).queryByRole("button")).not.toBeInTheDocument();
  });
});
