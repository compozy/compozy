import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { delay, http, HttpResponse } from "msw";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { LoopConfigureDialog } from "../configure/loop-configure-dialog";
import { handlers } from "../../mocks";
import {
  loopConfigFixture,
  loopDetailByName,
  loopEffectiveConfigFixture,
} from "../../mocks/fixtures";

const WS = "ws_default";
const delivery = loopDetailByName.get("software-delivery")!;
const watch = loopDetailByName.get("reviews-watch")!;

interface CapturedConfig {
  config: Record<string, unknown>;
}

let putBodies: CapturedConfig[] = [];

function captureHandlers() {
  return [
    http.put("/api/workspaces/:workspaceId/loops/:name/config", async ({ request }) => {
      const body = (await request.json()) as CapturedConfig;
      putBodies.push(body);
      return HttpResponse.json({ config: body.config });
    }),
    ...handlers,
  ];
}

function failingHandlers() {
  return [
    http.put("/api/workspaces/:workspaceId/loops/:name/config", () =>
      HttpResponse.json({ error: "boom" }, { status: 500 })
    ),
    ...handlers,
  ];
}

function renderSheet(
  options: {
    loop?: typeof delivery;
    config?: typeof loopConfigFixture | null;
  } = {}
) {
  const loop = options.loop ?? delivery;
  const config = options.config === undefined ? loopConfigFixture : options.config;
  const onOpenChange = vi.fn<(open: boolean) => void>();
  const onOpenEditor = vi.fn<() => void>();
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  render(
    <LoopConfigureDialog
      open
      workspaceId={WS}
      loop={loop}
      config={config}
      effectiveConfig={loopEffectiveConfigFixture}
      onOpenChange={onOpenChange}
      onOpenEditor={onOpenEditor}
    />,
    { wrapper }
  );
  return { onOpenChange, onOpenEditor };
}

function rowSwitch(testId: string) {
  return within(screen.getByTestId(testId)).getByRole("switch");
}

describe("LoopConfigureDialog", () => {
  beforeEach(() => {
    putBodies = [];
    vi.stubGlobal("fetch", createMswFetch(captureHandlers));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should lock structural edits and label the workspace editor action truthfully", () => {
    const { onOpenEditor } = renderSheet({ config: null });
    expect(screen.getByTestId("loop-configure-structural-note")).toHaveTextContent(
      /steps, inputs, node kinds and the goal stay fixed/i
    );
    const editAction = screen.getByTestId("loop-configure-edit-link");
    expect(editAction).toHaveTextContent("Edit");
    fireEvent.click(editAction);
    expect(onOpenEditor).toHaveBeenCalledTimes(1);
  });

  it("Should render no cost-cap field anywhere in the dialog", () => {
    renderSheet({ config: null });
    expect(screen.queryByTestId("loop-configure-limit-cost")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-configure-dialog")).not.toHaveTextContent(/cost cap/i);
  });

  it("Should disable a command check's command field when the check is toggled off", () => {
    renderSheet({ config: null });
    const command = screen.getByTestId("loop-configure-command-verify_build");
    expect(command).not.toBeDisabled();
    fireEvent.click(rowSwitch("loop-configure-check-verify_build"));
    expect(command).toBeDisabled();
    fireEvent.click(rowSwitch("loop-configure-check-verify_build"));
    expect(command).not.toBeDisabled();
  });

  it("Should render a locked, always-on switch and no command field for a structural check", () => {
    renderSheet({ config: null });
    const locked = rowSwitch("loop-configure-check-acceptance_review");
    expect(locked).toHaveAttribute("aria-disabled", "true");
    expect(locked).toHaveAttribute("aria-checked", "true");
    expect(
      screen.queryByTestId("loop-configure-command-acceptance_review")
    ).not.toBeInTheDocument();
  });

  it("Should show an empty state when the loop declares no verification checks", () => {
    renderSheet({ loop: watch, config: null });
    expect(screen.getByTestId("loop-configure-checks-empty")).toBeInTheDocument();
  });

  it("Should select the full-body re-attempt strategy", () => {
    renderSheet({ config: null });
    expect(screen.getByTestId("loop-configure-strategy-failed-only")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    fireEvent.click(screen.getByTestId("loop-configure-strategy-full-body"));
    expect(screen.getByTestId("loop-configure-strategy-full-body")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    expect(screen.getByTestId("loop-configure-strategy-failed-only")).toHaveAttribute(
      "aria-checked",
      "false"
    );
  });

  it("Should clamp a per-loop limit override at its ceiling", () => {
    renderSheet({ config: null });
    const input = screen.getByTestId("loop-configure-limit-input-iteration_cap");
    fireEvent.change(input, { target: { value: "500" } });
    expect(input).toHaveValue(100);
  });

  it("Should reset every edit back to the inherited defaults", () => {
    renderSheet({ config: null });
    const gate = rowSwitch("loop-configure-human-gate");
    fireEvent.click(gate);
    expect(gate).toHaveAttribute("aria-checked", "true");
    fireEvent.change(screen.getByTestId("loop-configure-limit-input-iteration_cap"), {
      target: { value: "80" },
    });
    fireEvent.click(screen.getByTestId("loop-configure-reset"));
    expect(rowSwitch("loop-configure-human-gate")).toHaveAttribute("aria-checked", "false");
    expect(screen.getByTestId("loop-configure-limit-input-iteration_cap")).toHaveValue(null);
  });

  it("Should PUT the edited config to loop_config and close on save", async () => {
    const { onOpenChange } = renderSheet({ config: null });
    fireEvent.click(rowSwitch("loop-configure-human-gate"));
    fireEvent.click(screen.getByTestId("loop-configure-strategy-full-body"));
    fireEvent.change(screen.getByTestId("loop-configure-limit-input-iteration_cap"), {
      target: { value: "80" },
    });
    fireEvent.change(screen.getByTestId("loop-configure-command-verify_build"), {
      target: { value: "go test ./..." },
    });
    fireEvent.click(screen.getByTestId("loop-configure-save"));

    await waitFor(() => expect(putBodies).toHaveLength(1));
    expect(putBodies[0].config).toMatchObject({
      iteration_cap: 80,
      human_gate_enabled: true,
      reattempt_strategy: "full_body",
      budget_on_exceeded: "escalate",
      enabled_checks_json: { verify_build: { enabled: true, command: "go test ./..." } },
    });
    expect(putBodies[0].config).not.toHaveProperty("budget_usd");
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
  });

  it("Should seed the sheet from the stored config", () => {
    renderSheet({ config: loopConfigFixture });
    expect(rowSwitch("loop-configure-human-gate")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("loop-configure-limit-input-iteration_cap")).toHaveValue(16);
  });

  it("Should close without saving via the header close and Cancel", () => {
    const { onOpenChange } = renderSheet({ config: null });
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onOpenChange).toHaveBeenLastCalledWith(false);
    fireEvent.click(screen.getByTestId("loop-configure-cancel"));
    expect(onOpenChange).toHaveBeenLastCalledWith(false);
    expect(putBodies).toHaveLength(0);
  });

  it("Should persist the budget-on-exceeded policy and clear an emptied limit", async () => {
    renderSheet({ config: null });
    fireEvent.change(screen.getByTestId("loop-configure-limit-policy"), {
      target: { value: "escalate" },
    });
    const cap = screen.getByTestId("loop-configure-limit-input-iteration_cap");
    fireEvent.change(cap, { target: { value: "42" } });
    fireEvent.change(cap, { target: { value: "" } });
    fireEvent.click(screen.getByTestId("loop-configure-save"));
    await waitFor(() => expect(putBodies).toHaveLength(1));
    expect(putBodies[0].config).toMatchObject({
      budget_on_exceeded: "escalate",
      iteration_cap: null,
    });
  });

  it("Should disable the strategy cards and Save while a save is in flight", async () => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => [
        http.put("/api/workspaces/:workspaceId/loops/:name/config", async () => {
          await delay("infinite");
          return HttpResponse.json({ config: {} });
        }),
        ...handlers,
      ])
    );
    renderSheet({ config: null });
    fireEvent.click(screen.getByTestId("loop-configure-save"));
    await waitFor(() =>
      expect(screen.getByTestId("loop-configure-strategy-full-body")).toBeDisabled()
    );
    expect(screen.getByTestId("loop-configure-save")).toBeDisabled();
    // The shared shell withdraws the header close while a save is in flight
    // rather than disabling it, so the only exits are Cancel and Escape.
    expect(screen.queryByRole("button", { name: "Close" })).toBeNull();
    expect(screen.getByTestId("loop-configure-edit-link")).toBeDisabled();
  });

  it("Should keep the sheet open when the save fails", async () => {
    const onOpenChange = vi.fn<(open: boolean) => void>();
    const onOpenEditor = vi.fn<() => void>();
    vi.stubGlobal("fetch", createMswFetch(failingHandlers));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);
    render(
      <LoopConfigureDialog
        open
        workspaceId={WS}
        loop={delivery}
        config={null}
        effectiveConfig={loopEffectiveConfigFixture}
        onOpenChange={onOpenChange}
        onOpenEditor={onOpenEditor}
      />,
      { wrapper }
    );
    fireEvent.click(screen.getByTestId("loop-configure-save"));
    await waitFor(() => expect(screen.getByTestId("loop-configure-save")).not.toBeDisabled());
    expect(onOpenChange).not.toHaveBeenCalled();
  });
});
