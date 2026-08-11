import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";
import { LoopRunForm } from "../run-form/loop-run-form";
import { handlers } from "../../mocks";
import { loopDetailByName, loopEffectiveConfigFixture } from "../../mocks/fixtures";
import type { LoopEffectiveConfig } from "../../types";

const WS = "ws_default";
const loop = loopDetailByName.get("implement-tasks")!;
const watchLoop = loopDetailByName.get("review-and-fix")!;
const savedConfig: Partial<LoopEffectiveConfig> = {
  iteration_cap: 3,
  budget_on_exceeded: "escalate",
  no_progress_window: 2,
  fan_out_width: 4,
  gate_max_revisions: 2,
  reattempt_strategy: "full_body",
  human_gate_enabled: true,
};

function stubFetch() {
  const mswFetch = createMswFetch(() => handlers);
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => mswFetch(input, init));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

let fetchMock: ReturnType<typeof stubFetch>;

function renderForm(
  onRunStarted = vi.fn(),
  selectedLoop = loop,
  config: Partial<LoopEffectiveConfig> | null = null
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  render(
    <LoopRunForm
      workspaceId={WS}
      loop={selectedLoop}
      effectiveConfig={{ ...loopEffectiveConfigFixture, ...config }}
      onRunStarted={onRunStarted}
    />,
    { wrapper }
  );
  return { onRunStarted };
}

async function runRequestBody(): Promise<Record<string, unknown>> {
  const call = fetchMock.mock.calls.find(([input]) => {
    const url = input instanceof Request ? input.url : String(input);
    return url.includes(`/api/workspaces/${WS}/loops/implement-tasks/run`);
  });
  if (!call) throw new Error("run request was not issued");
  const [input, init] = call;
  const request = input instanceof Request ? input.clone() : new Request(input, init);
  return request.json() as Promise<Record<string, unknown>>;
}

describe("LoopRunForm", () => {
  beforeEach(() => {
    fetchMock = stubFetch();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should auto-generate a typed field per declared input with a type badge", () => {
    renderForm();
    const slug = screen.getByTestId("loop-run-field-slug");
    expect(slug).toHaveAttribute("data-input-type", "string");
    expect(slug).toHaveTextContent("string");
    expect(screen.getByTestId("loop-run-field-implementer")).toHaveAttribute(
      "data-input-type",
      "agent"
    );
    expect(screen.getByTestId("loop-run-field-auto_commit")).toHaveAttribute(
      "data-input-type",
      "boolean"
    );
  });

  it("Should keep Run and Dry run disabled until the required input is filled", () => {
    renderForm();
    expect(screen.getByTestId("loop-run-submit-button")).toBeDisabled();
    fireEvent.change(screen.getByTestId("loop-run-field-input-slug"), {
      target: { value: "billing-webhooks" },
    });
    expect(screen.getByTestId("loop-run-submit-button")).not.toBeDisabled();
    expect(screen.getByTestId("loop-run-dry-button")).not.toBeDisabled();
  });

  it("Should create no run when a required input is blank and submit is attempted", async () => {
    const { onRunStarted } = renderForm();
    const submit = screen.getByTestId("loop-run-submit-button");
    expect(submit).toBeDisabled();
    fireEvent.click(submit);
    fireEvent.submit(screen.getByTestId("loop-run-field-input-slug").closest("form")!);
    await waitFor(() =>
      expect(screen.getByTestId("loop-run-field-error-slug")).toBeInTheDocument()
    );
    expect(screen.getByTestId("loop-run-field-error-slug")).toHaveTextContent(
      "slug is required to run this loop."
    );
    expect(onRunStarted).not.toHaveBeenCalled();
    const runCalls = fetchMock.mock.calls.filter(([input]) => {
      const url = input instanceof Request ? input.url : String(input);
      return url.includes("/loops/implement-tasks/run");
    });
    expect(runCalls).toHaveLength(0);
  });

  it("Should render NO stop control anywhere in the form", () => {
    renderForm();
    expect(screen.queryByRole("button", { name: /^stop/i })).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-run-form")).not.toHaveTextContent(/\bstop\b/i);
  });

  it("Should render NO cost-cap input anywhere in the form", () => {
    renderForm();
    expect(screen.queryByTestId("loop-run-override-cost")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-run-form")).not.toHaveTextContent(/cost cap/i);
  });

  it("Should render the gen-1 plan on Dry run without navigating", async () => {
    const { onRunStarted } = renderForm();
    fireEvent.change(screen.getByTestId("loop-run-field-input-slug"), {
      target: { value: "billing-webhooks" },
    });
    fireEvent.click(screen.getByTestId("loop-run-dry-button"));
    await waitFor(() => expect(screen.getByTestId("loop-run-plan")).toBeInTheDocument());
    expect(screen.getAllByTestId("loop-run-plan-node").length).toBeGreaterThan(0);
    expect(screen.getByTestId("loop-run-plan")).toHaveTextContent("billing-webhooks");
    expect(screen.getByTestId("loop-run-plan")).not.toHaveTextContent("{{ .inputs.slug }}");
    expect(onRunStarted).not.toHaveBeenCalled();
  });

  it("Should start a run and hand the run id back for navigation", async () => {
    const { onRunStarted } = renderForm();
    fireEvent.change(screen.getByTestId("loop-run-field-input-slug"), {
      target: { value: "billing-webhooks" },
    });
    fireEvent.click(screen.getByTestId("loop-run-submit-button"));
    await waitFor(() => expect(onRunStarted).toHaveBeenCalledWith("looprun_running"));
    await expect(runRequestBody()).resolves.not.toHaveProperty("network_participation");
  });

  it("Should return canonical immutable snapshots from dry-run and run mock responses", async () => {
    const dryResponse = await fetch(
      `http://localhost/api/workspaces/${WS}/loops/implement-tasks/run?dry=true`,
      {
        body: JSON.stringify({
          inputs: { slug: "snapshot-round-trip" },
          network_participation: { mode: "local" },
        }),
        headers: { "content-type": "application/json" },
        method: "POST",
      }
    );
    const dryBody = (await dryResponse.json()) as {
      dry_run: {
        resolved_network_participation: {
          bounds: { max_wakes: number };
          mode: string;
          source: string;
        };
      };
    };
    expect(dryBody.dry_run.resolved_network_participation).toEqual(
      buildLocalNetworkParticipationFixture()
    );

    const runResponse = await fetch(
      `http://localhost/api/workspaces/${WS}/loops/implement-tasks/run`,
      {
        body: JSON.stringify({
          inputs: { slug: "snapshot-round-trip" },
          network_participation: {
            channel_id: " release-room ",
            channel_strategy: "named",
            mode: "live",
          },
        }),
        headers: { "content-type": "application/json" },
        method: "POST",
      }
    );
    const runBody = (await runResponse.json()) as {
      run: {
        resolved_network_participation: {
          bounds: { max_wakes: number };
          channel_id: string;
          channel_strategy: string;
          mode: string;
          source: string;
          workspace_id: string;
        };
      };
    };
    expect(runBody.run.resolved_network_participation).toMatchObject({
      bounds: { max_wakes: 8 },
      channel_id: "release-room",
      channel_strategy: "named",
      mode: "live",
      source: "explicit_request",
      workspace_id: WS,
    });
  });

  it("Should serialize an explicit Live run without legacy participation fields", async () => {
    renderForm();
    expect(screen.getByTestId("loop-run-participation-mode")).toHaveValue("local");
    fireEvent.change(screen.getByTestId("loop-run-field-input-slug"), {
      target: { value: "billing-webhooks" },
    });
    fireEvent.change(screen.getByTestId("loop-run-participation-mode"), {
      target: { value: "live" },
    });
    fireEvent.change(screen.getByTestId("loop-run-participation-channel"), {
      target: { value: "release-room" },
    });
    fireEvent.change(screen.getByTestId("loop-run-participation-strategy"), {
      target: { value: "named" },
    });
    expect(screen.getByTestId("loop-run-participation-mode")).toHaveValue("live");
    expect(screen.getByTestId("loop-run-participation-channel")).toHaveValue("release-room");

    fireEvent.click(screen.getByTestId("loop-run-submit-button"));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await expect(runRequestBody()).resolves.toEqual({
      inputs: expect.any(Object),
      config_overrides: null,
      network_participation: {
        mode: "live",
        channel_id: "release-room",
        channel_strategy: "named",
      },
    });
  });

  it("Should start a run for the selected Loop, not the fixture default", async () => {
    const { onRunStarted } = renderForm(vi.fn(), watchLoop);
    fireEvent.change(screen.getByTestId("loop-run-field-input-task_name"), {
      target: { value: "billing-webhooks" },
    });
    fireEvent.click(screen.getByTestId("loop-run-submit-button"));
    await waitFor(() => expect(onRunStarted).toHaveBeenCalledWith("looprun_review_running"));
  });

  it("Should ignore a second run submit while the first run mutation is pending", async () => {
    const { onRunStarted } = renderForm();
    fireEvent.change(screen.getByTestId("loop-run-field-input-slug"), {
      target: { value: "billing-webhooks" },
    });
    fireEvent.click(screen.getByTestId("loop-run-submit-button"));
    fireEvent.click(screen.getByTestId("loop-run-submit-button"));
    await waitFor(() => expect(onRunStarted).toHaveBeenCalledTimes(1));
    const runRequests = fetchMock.mock.calls.filter(call => {
      const input = call[0];
      const url = input instanceof Request ? input.url : String(input);
      return url.includes(`/api/workspaces/${WS}/loops/implement-tasks/run`);
    });
    expect(runRequests).toHaveLength(1);
  });

  it("Should keep saved Loop defaults and leave an untouched run override-free", async () => {
    renderForm(vi.fn(), loop, savedConfig);

    fireEvent.click(screen.getByRole("button", { name: /Limits/ }));
    expect(screen.getByTestId("loop-run-override-input-iteration_cap")).toHaveAttribute(
      "placeholder",
      "3"
    );
    expect(screen.getByTestId("loop-run-override-policy")).toHaveValue("escalate");
    expect(screen.getByTestId("loop-run-overrides-badge")).toHaveTextContent("loop defaults");

    fireEvent.change(screen.getByTestId("loop-run-field-input-slug"), {
      target: { value: "billing-webhooks" },
    });
    fireEvent.click(screen.getByTestId("loop-run-submit-button"));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await expect(runRequestBody()).resolves.toMatchObject({ config_overrides: null });
  });

  it("Should apply an explicit per-run cap without changing the saved Loop baseline", async () => {
    renderForm(vi.fn(), loop, savedConfig);

    fireEvent.click(screen.getByRole("button", { name: /Limits/ }));
    const capInput = screen.getByTestId("loop-run-override-input-iteration_cap");
    expect(capInput).toHaveAttribute("placeholder", "3");

    fireEvent.change(capInput, { target: { value: "4" } });
    expect(screen.getByTestId("loop-run-overrides-badge")).toHaveTextContent("overrides set");
    expect(capInput).toHaveAttribute("placeholder", "3");

    fireEvent.change(capInput, { target: { value: "" } });
    expect(screen.getByTestId("loop-run-overrides-badge")).toHaveTextContent("loop defaults");

    fireEvent.change(capInput, { target: { value: "4" } });
    fireEvent.change(screen.getByTestId("loop-run-field-input-slug"), {
      target: { value: "billing-webhooks" },
    });
    fireEvent.click(screen.getByTestId("loop-run-submit-button"));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await expect(runRequestBody()).resolves.toMatchObject({
      config_overrides: { iteration_cap: 4 },
    });
  });
});
