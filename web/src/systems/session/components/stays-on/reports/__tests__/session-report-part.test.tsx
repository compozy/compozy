// Invariant: a report starts nothing; only choosing sends, and it sends an ordinary prompt.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { AgentInstanceAway, AgentInstancePayload, AgentInstanceReport } from "@/systems/agent";
import { agentInstanceFixture } from "@/systems/agent/mocks";

import { SessionReportSendContext } from "../../../../lib/session-report-send";
import type { SessionReportData } from "../../../../types";
import { SessionReportPart } from "../session-report-part";

const reads = vi.hoisted(() => ({
  instances: vi.fn(async () => [] as AgentInstancePayload[]),
  reports: vi.fn(async () => [] as AgentInstanceReport[]),
  away: vi.fn(async () => null as AgentInstanceAway | null),
}));

vi.mock("@/systems/agent/adapters/agent-instance-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent/adapters/agent-instance-api")>()),
  fetchAgentInstances: reads.instances,
  fetchAgentInstanceReports: reads.reports,
  fetchAgentInstanceAway: reads.away,
}));

vi.mock("@assistant-ui/react", async importOriginal => ({
  ...(await importOriginal<typeof import("@assistant-ui/react")>()),
  useAuiState: () => undefined,
}));

vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => vi.fn(),
}));

function harness(send: ((text: string) => void) | null) {
  return function Wrapper({ children }: { children: ReactNode }) {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return (
      <QueryClientProvider client={client}>
        <UIProvider reducedMotion="never" skipAnimations>
          <SessionReportSendContext value={send}>{children}</SessionReportSendContext>
        </UIProvider>
      </QueryClientProvider>
    );
  };
}

const INSTANCE = agentInstanceFixture({ id: "ainst-1" });

const PROPOSAL: SessionReportData = {
  report_id: "arpt-1",
  instance_id: "ainst-1",
  kind: "proposal",
  created_at: "2026-10-31T23:30:04Z",
  payload: {
    proposals: [
      { title: "Unblock the token migration", body: "Run 12 waits on a review." },
      { title: "Close the stale design issue", body: "#918 was fixed in #931." },
    ],
  },
};

describe("SessionReportPart", () => {
  beforeEach(() => {
    reads.instances.mockResolvedValue([INSTANCE]);
    reads.reports.mockResolvedValue([]);
    reads.away.mockResolvedValue(null);
  });

  it("Should send nothing until a choice is picked, then its title as a prompt [UT-106]", async () => {
    const send = vi.fn();
    render(<SessionReportPart data={PROPOSAL} />, { wrapper: harness(send) });

    expect(send).not.toHaveBeenCalled();
    await userEvent.click(
      await screen.findByRole("button", { name: /Unblock the token migration/ })
    );
    expect(send).toHaveBeenCalledExactlyOnceWith("Unblock the token migration");
  });

  it("Should offer no way to send from a conversation that moved on", async () => {
    render(<SessionReportPart data={PROPOSAL} />, { wrapper: harness(null) });

    expect(await screen.findByRole("button", { name: /Unblock/ })).toBeDisabled();
  });

  it("Should say a report may not have been received [US-022.EC-5]", async () => {
    reads.reports.mockResolvedValue([
      { id: "arpt-1", kind: "proposal", state: "indeterminate", created_at: PROPOSAL.created_at },
    ]);
    render(<SessionReportPart data={PROPOSAL} />, { wrapper: harness(vi.fn()) });

    expect(await screen.findByText(/may not have reached/)).toBeInTheDocument();
  });

  it("Should tell a provider-ended request apart from an unanswered one [US-023.EC-1]", async () => {
    render(
      <SessionReportPart
        data={{
          ...PROPOSAL,
          kind: "approval_expired",
          payload: { tool: "make migrate", expired_by: "provider" },
        }}
      />,
      { wrapper: harness(vi.fn()) }
    );

    expect(await screen.findByText(/request ended early/)).toBeInTheDocument();
    expect(screen.getByText("Didn't happen")).toBeInTheDocument();
    expect(screen.getByText("make migrate")).toBeInTheDocument();
  });

  it("Should fold a told report into the digest and keep the decision", async () => {
    reads.away.mockResolvedValue({
      last_seen_at: "2026-10-31T20:00:00Z",
      quiet_check_ins: 20,
      reports: [
        { id: "arpt-0", kind: "failure", state: "pending", created_at: "2026-10-31T21:00:00Z" },
        { id: "arpt-1", kind: "proposal", state: "pending", created_at: PROPOSAL.created_at },
      ],
    });
    const failure = { ...PROPOSAL, report_id: "arpt-0", kind: "failure", payload: { detail: "x" } };
    render(
      <>
        <SessionReportPart data={failure} />
        <SessionReportPart data={PROPOSAL} />
      </>,
      { wrapper: harness(vi.fn()) }
    );

    expect(await screen.findByRole("region", { name: "While you were away" })).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole("article", { name: "I couldn't finish this check" })).toBeNull()
    );
    expect(await screen.findByRole("article", { name: /asks/ })).toBeInTheDocument();
  });
});
