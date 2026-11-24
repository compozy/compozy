// Suite: Automation job editor Run on target selection and save routing (UT-094)
// Invariant: Automation job editor omits Auto in Run on, and saving a job with devbox selected
//            posts to apiClientFor("cmp_7f3k2q9x") /api/automation/jobs.
// Boundary IN: WhereSection, useCreateAutomationJob, and apiClientFor.
// Boundary OUT: Remote daemon scheduler execution (covered by Go IT-046).

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import * as apiClientModule from "@/lib/api-client";
import type { ComputerId } from "@/lib/computer-scope";
import * as computersApi from "@/systems/computers/adapters/computers-api";
import * as profileHooks from "@/systems/profiles";
import { WhereSection } from "../automation-form/where-section";
import { useCreateAutomationJob } from "../../hooks/use-automation-actions";
import { createAutomationFormDraft } from "../../lib/automation-form-draft";

describe("Automation job editor Run on (UT-094)", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("UT-094: Should omit Auto in Run on and post saved job to apiClientFor('cmp_7f3k2q9x') /api/automation/jobs", async () => {
    const user = userEvent.setup();
    vi.spyOn(computersApi, "fetchComputers").mockResolvedValue({
      local: { run_agents: true },
      computers: [
        {
          account_id: "acct_1",
          id: "cmp_7f3k2q9x",
          name: "devbox",
          platform: "linux_x86_64",
          version: "0.8.0",
          protocol: { min: 1, max: 1 },
          status: "connected",
          enabled: true,
          plan_state: "ok",
          route_in_use: null,
          routes: [],
          scopes: ["sessions:operate"],
          load_balancing_weight: 50,
          diagnosis: null,
          added_at: "2026-04-01T00:00:00Z",
          last_connected_at: "2026-04-10T00:00:00Z",
        },
      ],
    });
    vi.spyOn(profileHooks, "useProfileReadScope").mockReturnValue({
      aggregate: false,
      destination: "default",
      params: { profile: "default" },
      ownerOf: () => ({ name: "default", color: "zinc", icon: "user" }),
    } as never);

    const postSpy = vi.fn().mockResolvedValue({
      data: {
        job: {
          id: "job_remote_1",
          name: "nightly-audit",
          profile_name: "default",
          scope: "global",
          agent_name: "general",
          prompt: "Run audit",
          schedule: { mode: "every", interval: "1h" },
          enabled: true,
          source: "dynamic",
          created_at: "2026-04-10T00:00:00Z",
          updated_at: "2026-04-10T00:00:00Z",
        },
      },
      error: undefined,
      response: new Response(null, { status: 201 }),
    });
    const apiClientForSpy = vi.spyOn(apiClientModule, "apiClientFor").mockReturnValue({
      POST: postSpy,
    } as never);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    function PickerHarness() {
      const [computerId, setComputerId] = useState<ComputerId>("local");
      return (
        <WhereSection
          computerId={computerId}
          mode="create"
          number={2}
          onComputerChange={setComputerId}
          timeZone="UTC"
        />
      );
    }

    render(<PickerHarness />, { wrapper });

    const trigger = await screen.findByTestId("automation-run-on-picker");
    await user.click(trigger);

    // Jobs belong to one concrete computer — Auto must never be listed.
    expect(screen.queryByTestId("automation-run-on-picker-item-auto")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("automation-run-on-picker-item-cmp_7f3k2q9x"));
    expect(screen.getByTestId("automation-where-hint")).toHaveTextContent(
      "This job is saved on devbox and runs there, even when this app is closed."
    );

    const { result } = renderHook(() => useCreateAutomationJob(), { wrapper });
    const draft = createAutomationFormDraft(null, { computerId: "cmp_7f3k2q9x" });
    await act(async () => {
      await result.current.mutateAsync({
        name: "nightly-audit",
        agent_name: "general",
        prompt: "Run audit",
        scope: "global",
        schedule: draft.schedule,
        computerId: "cmp_7f3k2q9x",
      });
    });

    expect(apiClientForSpy).toHaveBeenCalledWith("cmp_7f3k2q9x");
    expect(postSpy).toHaveBeenCalledWith(
      "/api/automation/jobs",
      expect.objectContaining({
        body: expect.objectContaining({ name: "nightly-audit" }),
      })
    );
  });
});
