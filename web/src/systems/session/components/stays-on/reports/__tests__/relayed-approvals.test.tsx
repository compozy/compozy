// Invariant: a relayed ask is answered on the session that asked, and only a main session relays.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { AgentInstancePayload } from "@/systems/agent";
import { agentInstanceFixture } from "@/systems/agent/mocks";

import type { SessionPendingInteraction } from "../../../../types";
import { RelayedApprovalDock, RelayedWaitingMarker } from "../relayed-approvals";

const reads = vi.hoisted(() => ({
  instances: vi.fn(async () => [] as AgentInstancePayload[]),
  pending: vi.fn(async () => [] as SessionPendingInteraction[]),
  approve: vi.fn(async () => undefined),
}));

vi.mock("@/systems/agent/adapters/agent-instance-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent/adapters/agent-instance-api")>()),
  fetchAgentInstances: reads.instances,
}));

vi.mock("../../../../adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../../../adapters/session-api")>()),
  fetchSessionPendingWithDescendants: reads.pending,
  approveSession: reads.approve,
}));

vi.mock("../../../../hooks/use-sessions", async importOriginal => ({
  ...(await importOriginal<typeof import("../../../../hooks/use-sessions")>()),
  useSession: () => ({ data: { name: "Migrate tokens", agent_name: "code-engineer" } }),
}));

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <QueryClientProvider client={client}>
      <UIProvider reducedMotion="never" skipAnimations>
        {children}
      </UIProvider>
    </QueryClientProvider>
  );
}

function ask(overrides: Partial<SessionPendingInteraction>): SessionPendingInteraction {
  return {
    session_id: "sess-child",
    interaction_id: "int-1",
    kind: "permission",
    provider_request_id: "perm-1",
    title: "make migrate",
    decisions: ["allow-once", "reject-once"],
    status: "pending",
    created_at: "2026-11-01T00:00:00Z",
    ...overrides,
  } as SessionPendingInteraction;
}

describe("Relayed approvals", () => {
  beforeEach(() => {
    reads.approve.mockClear();
    reads.instances.mockResolvedValue([agentInstanceFixture({ main_session_id: "sess-main" })]);
    reads.pending.mockResolvedValue([ask({})]);
  });

  it("Should answer on the original session and record the decision [US-023.AC-2]", async () => {
    render(<RelayedApprovalDock sessionId="sess-main" workspaceId="ws-1" />, { wrapper });

    expect(await screen.findByText(/In Migrate tokens/)).toBeInTheDocument();
    await userEvent.click(screen.getByTestId("permission-allow-once"));

    await waitFor(() => expect(reads.approve).toHaveBeenCalledOnce());
    expect(JSON.stringify(reads.approve.mock.calls[0])).toContain("sess-child");
    expect(JSON.stringify(reads.approve.mock.calls[0])).toContain("perm-1");
    expect(await screen.findByText(/You allowed/)).toBeInTheDocument();
  });

  it("Should leave this session's own asks to its own dock", async () => {
    reads.pending.mockResolvedValue([ask({ session_id: "sess-main" })]);
    render(<RelayedWaitingMarker sessionId="sess-main" workspaceId="ws-1" />, { wrapper });

    await waitFor(() => expect(reads.pending).toHaveBeenCalled());
    expect(screen.queryByText(/waiting for you/)).toBeNull();
  });

  it("Should not read descendants for a session that isn't a main session", async () => {
    render(<RelayedWaitingMarker sessionId="sess-other" workspaceId="ws-1" />, { wrapper });

    await waitFor(() => expect(reads.instances).toHaveBeenCalled());
    expect(reads.pending).not.toHaveBeenCalled();
  });
});
