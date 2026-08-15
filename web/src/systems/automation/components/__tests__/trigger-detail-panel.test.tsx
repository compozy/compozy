// Suite: Trigger detail panel
// Invariant: The trigger detail surface renders only what the daemon can back — the rule as a
// sentence, enable gated by source, run links gated on recorded ids, the local webhook path
// without the signing secret, and public reachability that never implies a URL that answers.
// Boundary IN: Trigger API read models and the trigger detail presentation.
// Boundary OUT: job detail (automation-detail-panel.test.tsx); dispatch and persistence,
// owned by daemon/store suites.
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AnchorHTMLAttributes } from "react";
import { toast } from "sonner";
import { describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";
import type { AutomationRun, AutomationTrigger } from "../../types";

interface MockLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  params?: { id?: string; name?: string; runId?: string };
  search?: { workspace?: string };
  to?: string;
}

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, params, search, to, ...props }: MockLinkProps) => {
    let path: string;
    switch (to) {
      case "/loop-runs/$runId":
        path = `/loop-runs/${params?.runId ?? ""}`;
        break;
      case "/loops/$name":
        path = `/loops/${params?.name ?? ""}`;
        break;
      case "/session/$id":
        path = `/session/${params?.id ?? ""}`;
        break;
      case "/settings/gateway":
        path = "/settings/gateway";
        break;
      default:
        throw new Error(`Unexpected test route: ${String(to)}`);
    }
    const href = search?.workspace
      ? `${path}?workspace=${encodeURIComponent(search.workspace)}`
      : path;
    return (
      <a href={href} {...props}>
        {children}
      </a>
    );
  },
  useNavigate: () => vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

import { TriggerDetailPanel } from "../trigger-detail/trigger-detail-panel";
import {
  buildTriggerDiagnostics,
  buildTriggerDiagnosticsNote,
  buildTriggerEnvelopeSample,
} from "../../lib/trigger-inspect-model";
import { buildTriggerLede, describeTriggerWhen } from "../../lib/trigger-sentence";

const triggerDefaults: AutomationTrigger = {
  id: "trg_summarize_failures",
  name: "summarize-failures",
  agent_name: "summarizer",
  prompt: 'Session {{ .Data.session_id }} stopped with reason "{{ .Data.stop_reason }}".',
  event: "session.stopped",
  filter: { "data.stop_reason": "error" },
  scope: "workspace" as const,
  workspace_id: "ws_checkout_api",
  source: "dynamic" as const,
  target_kind: "agent",
  enabled: true,
  retry: { strategy: "backoff" as const, max_retries: 2, base_delay: "5s" },
  fire_limit: { max: 4, window: "1h" },
  webhook_secret_present: false,
  created_at: "2026-04-09T09:12:00Z",
  updated_at: "2026-04-17T16:40:00Z",
};

function makeTrigger(overrides: Partial<AutomationTrigger> = {}): AutomationTrigger {
  return { ...triggerDefaults, ...overrides };
}

const agentTrigger = makeTrigger();

const loopTrigger = makeTrigger({
  id: "trg_rerun_delivery",
  name: "rerun-delivery",
  agent_name: "",
  prompt: "",
  target_kind: "loop",
  loop_target: {
    workspace_id: "ws_checkout_api",
    loop_name: "software-delivery",
    inputs: { implementer: "code_implementer" },
    input_mapping: { slug: "data.session_name" },
  },
});

const webhookTrigger = makeTrigger({
  id: "trg_deploy_webhook",
  name: "deploy-webhook",
  agent_name: "deployer",
  prompt: 'Validate deployment {{ index .Data "sha" }}.',
  event: "webhook",
  filter: { "data.action": "deploy", "data.branch": "main" },
  endpoint_slug: "deploy",
  webhook_id: "wbh_abc123",
  webhook_secret_present: true,
  fire_limit: { max: 6, window: "1h" },
});

const runDefaults: AutomationRun = {
  id: "run_001",
  status: "completed" as const,
  attempt: 1,
  trigger_id: "trg_summarize_failures",
  session_id: "sess_9f2a1c",
  started_at: "2026-04-17T16:38:04Z",
  ended_at: "2026-04-17T16:38:22Z",
};

function makeRun(overrides: Partial<AutomationRun> = {}): AutomationRun {
  return { ...runDefaults, ...overrides };
}

const completedRun = makeRun();

type PanelProps = Parameters<typeof TriggerDetailPanel>[0];

function renderPanel(overrides: Partial<PanelProps> = {}) {
  const onBack = vi.fn();
  const onDelete = vi.fn();
  const onEdit = vi.fn();
  const onRetryRuns = vi.fn();
  const onToggleEnabled = vi.fn();

  const render = (nextOverrides: Partial<PanelProps>) => (
    <TriggerDetailPanel
      error={null}
      loopWorkspaceName="checkout-api"
      onBack={onBack}
      onDelete={onDelete}
      onEdit={onEdit}
      onRetryRuns={onRetryRuns}
      onToggleEnabled={onToggleEnabled}
      runs={[completedRun]}
      runsError={null}
      runsLoading={false}
      state={{ isDeleting: false, isLoading: false, isTogglePending: false }}
      trigger={agentTrigger}
      workspaceName="checkout-api"
      {...nextOverrides}
    />
  );
  const view = renderWithTopbar(render(overrides));

  return {
    onBack,
    onDelete,
    onEdit,
    onRetryRuns,
    onToggleEnabled,
    rerenderPanel: (nextOverrides: Partial<PanelProps>) => view.rerender(render(nextOverrides)),
    unmount: view.unmount,
  };
}

describe("TriggerDetailPanel", () => {
  it("Should hold the layout width while loading instead of guessing the rule", () => {
    renderPanel({ state: { isDeleting: false, isLoading: true, isTogglePending: false } });

    expect(screen.getByTestId("automation-detail-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("trigger-detail-sentence")).not.toBeInTheDocument();
  });

  it("Should surface the daemon's own reason when the trigger cannot be read", () => {
    renderPanel({
      error: new Error("This workspace-scoped trigger belongs to another workspace."),
      trigger: undefined,
    });

    expect(screen.getByTestId("automation-detail-error")).toHaveTextContent(
      "This workspace-scoped trigger belongs to another workspace."
    );
  });

  it("Should offer a way back when the trigger no longer exists", () => {
    const { onBack } = renderPanel({ trigger: undefined });

    expect(screen.getByTestId("automation-detail-empty")).toHaveTextContent("Trigger unavailable");
    fireEvent.click(screen.getByRole("button", { name: "Back to Triggers" }));
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("Should read an agent trigger as one plain-language rule with the verb run", () => {
    renderPanel();

    expect(screen.getByTestId("trigger-detail-sentence")).toHaveTextContent(
      "When a session stops in checkout-api, if the stop reason is error, run summarizer."
    );
    expect(screen.getByTestId("trigger-rule-card")).toHaveTextContent("A session stops");
    expect(screen.getByTestId("trigger-rule-card")).toHaveTextContent("Exact match on");
    expect(screen.getByTestId("trigger-prompt-preview")).toBeInTheDocument();
    expect(screen.queryByTestId("trigger-pause-line")).not.toBeInTheDocument();
  });

  it("Should read a loop trigger as start, with mapped inputs and no prompt", () => {
    renderPanel({ runs: [], trigger: loopTrigger });

    expect(screen.getByTestId("trigger-detail-sentence")).toHaveTextContent(
      "start software-delivery."
    );
    const mapping = screen.getByTestId("trigger-loop-mapping");
    expect(mapping).toHaveTextContent("slug");
    expect(mapping).toHaveTextContent("data.session_name");
    expect(mapping).toHaveTextContent("code_implementer");
    expect(screen.queryByTestId("trigger-prompt-preview")).not.toBeInTheDocument();
    expect(screen.getByTestId("trigger-loop-link")).toHaveAttribute(
      "href",
      "/loops/software-delivery?workspace=ws_checkout_api"
    );
  });

  it("Should explain the pause and keep the switch as the control while disabled", () => {
    const { onToggleEnabled } = renderPanel({
      trigger: { ...loopTrigger, enabled: false },
      runs: [],
    });

    expect(screen.getByTestId("trigger-pause-line")).toHaveTextContent(
      "Matching events will not start the loop until this trigger is enabled."
    );
    expect(screen.getByTestId("trigger-enable-label")).toHaveTextContent("Disabled");
    fireEvent.click(screen.getByTestId("trigger-enable-switch"));
    expect(onToggleEnabled).toHaveBeenCalledWith(true);
  });

  it("Should keep the confirmed state on the track while the enable request is in flight", () => {
    const { onToggleEnabled } = renderPanel({
      state: { isDeleting: false, isLoading: false, isTogglePending: true },
      trigger: { ...agentTrigger, enabled: false },
    });

    const track = screen.getByTestId("trigger-enable-switch");
    expect(screen.getByTestId("trigger-enable-label")).toHaveTextContent("Enabling…");
    expect(track).toHaveAttribute("aria-checked", "false");
    expect(track).toHaveAttribute("aria-disabled", "true");
    expect(track).toHaveAttribute("tabindex", "-1");
    fireEvent.click(track);
    expect(onToggleEnabled).not.toHaveBeenCalled();
  });

  it("Should report copy failure when the browser has no Clipboard API", () => {
    const clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });

    try {
      renderPanel();
      fireEvent.click(screen.getByTestId("automation-detail-overflow"));
      fireEvent.click(screen.getByTestId("copy-trigger-id-btn"));

      expect(toast.error).toHaveBeenCalledWith("Could not copy the trigger id.");
    } finally {
      if (clipboardDescriptor) {
        Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
      } else {
        Reflect.deleteProperty(navigator, "clipboard");
      }
    }
  });

  it.each([
    {
      copy: "This automation is defined in configuration files. Only its enabled state can be changed here.",
      source: "config" as const,
    },
    {
      copy: "This automation is provided by an installed package. Only its enabled state can be changed here.",
      source: "package" as const,
    },
  ])(
    "Should drop edit and delete for a $source-owned trigger, never disable them",
    ({ copy, source }) => {
      const { onToggleEnabled } = renderPanel({ runs: [], trigger: makeTrigger({ source }) });

      expect(screen.getByTestId("trigger-detail-lock")).toHaveTextContent(copy);
      expect(screen.queryByTestId("edit-trigger-btn")).not.toBeInTheDocument();
      fireEvent.click(screen.getByTestId("automation-detail-overflow"));
      expect(screen.queryByTestId("edit-automation-btn")).not.toBeInTheDocument();
      expect(screen.queryByTestId("delete-automation-btn")).not.toBeInTheDocument();
      fireEvent.click(screen.getByTestId("toggle-automation-btn"));
      expect(onToggleEnabled).toHaveBeenCalledWith(false);
      expect(document.querySelector("[data-slot='topbar-actions']")).toBeNull();
    }
  );

  it("Should disable the managed overflow toggle while its update is pending", () => {
    const { onToggleEnabled } = renderPanel({
      runs: [],
      state: { isDeleting: false, isLoading: false, isTogglePending: true },
      trigger: makeTrigger({ source: "config" }),
    });

    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    expect(screen.getByTestId("toggle-automation-btn")).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(screen.getByTestId("toggle-automation-btn"));
    expect(onToggleEnabled).not.toHaveBeenCalled();
  });

  it("Should require explicit name confirmation before deleting a dynamic trigger", async () => {
    const user = userEvent.setup();
    const { onDelete, onEdit } = renderPanel({ runs: [] });

    fireEvent.click(screen.getByTestId("edit-trigger-btn"));
    expect(onEdit).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    fireEvent.click(screen.getByTestId("delete-automation-btn"));
    expect(onDelete).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Delete trigger?" })).toBeInTheDocument();

    await user.type(screen.getByLabelText("Type to confirm"), agentTrigger.name);
    await user.click(screen.getByTestId("confirm-delete-automation-btn"));
    expect(onDelete).toHaveBeenCalledOnce();
  });

  it("Should link a run only when the daemon recorded the id it points at", () => {
    renderPanel({
      runs: [
        completedRun,
        { ...completedRun, id: "run_pending", status: "scheduled" as const, session_id: undefined },
      ],
    });

    fireEvent.click(screen.getByTestId("automation-run-run_001"));
    expect(screen.getByTestId("trigger-run-open-run_001")).toHaveAttribute(
      "href",
      "/session/sess_9f2a1c"
    );

    fireEvent.click(screen.getByTestId("automation-run-run_pending"));
    expect(screen.queryByTestId("trigger-run-open-run_pending")).not.toBeInTheDocument();
    expect(screen.getByTestId("trigger-run-drawer-run_pending")).toHaveTextContent(
      "The event was accepted. No session yet"
    );
  });

  it("Should hand a delegated run to its loop run rather than a session", () => {
    renderPanel({
      runs: [
        {
          ...completedRun,
          id: "run_loop",
          status: "delegated" as const,
          session_id: undefined,
          ended_at: undefined,
          loop_run_id: "looprun_8f3a2b",
        },
      ],
      trigger: loopTrigger,
    });

    fireEvent.click(screen.getByTestId("automation-run-run_loop"));
    expect(screen.getByTestId("trigger-run-open-run_loop")).toHaveAttribute(
      "href",
      "/loop-runs/looprun_8f3a2b?workspace=ws_checkout_api"
    );
    expect(screen.getByTestId("trigger-run-drawer-run_loop")).toHaveTextContent(
      "Handed to software-delivery."
    );
  });

  it("Should say a duration only when both timestamps exist", () => {
    renderPanel({
      runs: [
        completedRun,
        { ...completedRun, id: "run_open", status: "running" as const, ended_at: undefined },
      ],
    });

    expect(screen.getByTestId("automation-run-run_001")).toHaveTextContent("18s");
    expect(screen.getByTestId("automation-run-run_open")).toHaveTextContent("—");
  });

  it("Should describe failed and canceled run drawers without losing diagnostics", () => {
    renderPanel({
      runs: [
        makeRun({
          id: "run_failed_detailed",
          status: "failed",
          attempt: 3,
          error: "Agent unavailable",
          delivery_error: "Dispatch rejected",
          session_id: undefined,
        }),
        makeRun({
          id: "run_failed_empty",
          status: "failed",
          session_id: undefined,
        }),
        makeRun({ id: "run_canceled", status: "canceled", session_id: undefined }),
      ],
    });

    fireEvent.click(screen.getByTestId("automation-run-run_failed_detailed"));
    expect(screen.getByTestId("trigger-run-drawer-run_failed_detailed")).toHaveTextContent(
      "Agent unavailable"
    );
    expect(screen.getByTestId("trigger-run-drawer-run_failed_detailed")).toHaveTextContent(
      "Delivery: Dispatch rejected"
    );
    expect(screen.getByTestId("trigger-run-drawer-run_failed_detailed")).toHaveTextContent(
      "Attempt 3."
    );

    fireEvent.click(screen.getByTestId("automation-run-run_failed_empty"));
    expect(screen.getByTestId("trigger-run-drawer-run_failed_empty")).toHaveTextContent(
      "The run failed before any detail was recorded."
    );

    fireEvent.click(screen.getByTestId("automation-run-run_canceled"));
    expect(screen.getByTestId("trigger-run-drawer-run_canceled")).toHaveTextContent(
      "This run was canceled."
    );
  });

  it("Should exclude scheduled reservations from the Last ran fact", () => {
    renderPanel({
      runs: [
        completedRun,
        makeRun({
          id: "run_reserved",
          status: "scheduled",
          session_id: undefined,
          started_at: "2026-04-18T18:00:00Z",
        }),
      ],
    });

    const times = screen.getByTestId("trigger-detail-subhead").querySelectorAll("time");
    expect(times[0]).toHaveAttribute("datetime", completedRun.started_at);
  });

  it("Should offer a retry when the run history fails and drop the count chip at zero", () => {
    const { onRetryRuns } = renderPanel({ runs: [], runsError: new Error("runs unavailable") });

    expect(screen.getByTestId("automation-run-history-error")).toHaveTextContent(
      "runs unavailable"
    );
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetryRuns).toHaveBeenCalledOnce();
  });

  it("Should invite the first activation instead of showing an empty table", () => {
    renderPanel({ runs: [] });

    expect(screen.getByTestId("automation-run-history-empty")).toHaveTextContent(
      "Runs will appear here after the first matching activation."
    );
    expect(screen.getByTestId("automation-run-history")).toHaveTextContent("no runs yet");
  });

  it("Should publish the local webhook path and the secret's presence, never its value", () => {
    renderPanel({ runs: [], trigger: webhookTrigger });

    expect(screen.getByTestId("trigger-webhook-endpoint")).toHaveTextContent(
      "/api/webhooks/workspaces/ws_checkout_api/deploy--wbh_abc123"
    );
    expect(screen.getByTestId("trigger-webhook-endpoint")).toHaveTextContent("POST");
    const rail = screen.getByTestId("trigger-detail-rail");
    expect(rail).toHaveTextContent("Signing secret");
    expect(rail).toHaveTextContent("Set");
    expect(rail).not.toHaveTextContent("sha256:deploy-webhook");
    expect(document.body.textContent).not.toContain("sha256:deploy-webhook");
  });

  it("Should read an AND filter as prose with both raw paths kept quiet", () => {
    renderPanel({ runs: [], trigger: webhookTrigger });

    const rule = screen.getByTestId("trigger-rule-card");
    expect(rule).toHaveTextContent("Action is");
    expect(rule).toHaveTextContent("and branch is");
    expect(rule).toHaveTextContent("AND of");
    expect(rule).toHaveTextContent("data.action");
    expect(rule).toHaveTextContent("data.branch");
  });

  it("Should say public delivery is off rather than show a URL that would not answer", () => {
    renderPanel({ runs: [], trigger: webhookTrigger });

    const ingress = screen.getByTestId("automation-trigger-ingress");
    expect(ingress).toHaveTextContent("Public delivery ingress is off");
    expect(ingress).toHaveTextContent("Open Gateway settings to publish one");
  });

  it("Should publish the delivery URL with its reachability once the gateway confirms it", () => {
    renderPanel({
      runs: [],
      trigger: {
        ...webhookTrigger,
        ingress: {
          confirmed_at: "2026-04-16T08:00:00Z",
          confirmed_endpoint_generation: 4,
          endpoint_generation: 4,
          reachability: "live",
          scope_kind: "workspace",
          subject_id: "trg_deploy_webhook",
          subject_kind: "webhook_trigger",
          url: "https://public.gateway.test/api/webhooks/workspaces/ws_checkout_api/deploy--wbh_abc123",
          workspace_id: "ws_checkout_api",
        },
      },
    });

    const ingress = screen.getByTestId("automation-trigger-ingress");
    expect(ingress).toHaveTextContent("Live");
    expect(ingress).toHaveTextContent(
      "https://public.gateway.test/api/webhooks/workspaces/ws_checkout_api/deploy--wbh_abc123"
    );
    expect(ingress).toHaveTextContent("There is no queue for messages sent while it is offline");
  });

  it("Should omit the public delivery card for a trigger with no webhook endpoint", () => {
    renderPanel({ runs: [] });

    expect(screen.queryByTestId("trigger-rail-public-delivery")).not.toBeInTheDocument();
    expect(screen.queryByTestId("trigger-webhook-endpoint")).not.toBeInTheDocument();
  });

  it("Should keep raw runtime enums and the activation envelope behind Inspect", async () => {
    renderPanel({ runs: [], trigger: webhookTrigger });

    expect(screen.queryByTestId("trigger-inspect-sheet")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("trigger-inspect-btn"));

    const sheet = await screen.findByTestId("trigger-inspect-sheet");
    expect(sheet).toHaveTextContent("DYNAMIC");
    expect(sheet).toHaveTextContent("WORKSPACE");
    expect(screen.getByTestId("trigger-inspect-tile-id")).toHaveTextContent(webhookTrigger.id);
    expect(screen.getByTestId("trigger-inspect-tile-id")).not.toHaveTextContent(
      webhookTrigger.name
    );
    expect(screen.getByTestId("trigger-inspect-tile-secret")).toHaveTextContent("present");
    expect(screen.getByTestId("trigger-inspect-diagnostics")).toHaveTextContent(
      "Fire limit 6 / 1h."
    );

    fireEvent.click(screen.getByTestId("trigger-inspect-tab-envelope"));
    await waitFor(() =>
      expect(screen.getByTestId("trigger-inspect-envelope")).toHaveTextContent('"kind": "webhook"')
    );
    expect(screen.getByTestId("trigger-inspect-envelope")).toHaveTextContent('"action": "deploy"');
    expect(screen.getByTestId("trigger-inspect-envelope")).toHaveTextContent(
      '"endpoint": "deploy--wbh_abc123"'
    );
  });

  it("Should reset trigger-owned overlays when the route changes trigger", async () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    try {
      const first = renderPanel({ runs: [] });

      fireEvent.click(screen.getByTestId("trigger-inspect-btn"));
      expect(await screen.findByTestId("trigger-inspect-sheet")).toBeInTheDocument();
      first.rerenderPanel({ runs: [], trigger: loopTrigger });
      await waitFor(() =>
        expect(screen.queryByTestId("trigger-inspect-sheet")).not.toBeInTheDocument()
      );
      first.unmount();

      const second = renderPanel({ runs: [] });
      fireEvent.click(screen.getByTestId("automation-detail-overflow"));
      fireEvent.click(screen.getByTestId("delete-automation-btn"));
      expect(screen.getByRole("dialog", { name: "Delete trigger?" })).toBeInTheDocument();
      second.rerenderPanel({ runs: [], trigger: loopTrigger });
      await waitFor(() =>
        expect(screen.queryByRole("dialog", { name: "Delete trigger?" })).not.toBeInTheDocument()
      );

      expect(
        consoleError.mock.calls.some(([message]) =>
          String(message).includes("Encountered two children with the same key")
        )
      ).toBe(false);
    } finally {
      consoleError.mockRestore();
    }
  });

  it("Should project non-webhook diagnostics, nested filters, and target mappings", () => {
    const trigger = makeTrigger({
      event: "ext.release.completed",
      filter: {
        kind: "ext.release.completed",
        source: "extension",
        workspace_id: "ws_checkout_api",
        "data.metadata.step": "complete",
      },
      loop_target: {
        workspace_id: "ws_checkout_api",
        loop_name: "release-loop",
        inputs: {},
        input_mapping: { step: "data.metadata.step" },
      },
      target_kind: "loop",
      agent_name: "",
      prompt: "",
    });

    expect(buildTriggerDiagnostics(trigger)).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: "retry", value: "backoff · 2 · 5s" }),
        expect.objectContaining({ id: "target", value: "loop · release-loop" }),
      ])
    );
    expect(buildTriggerDiagnosticsNote(trigger)).toContain("Mapping step ← data.metadata.step.");
    const envelope = buildTriggerEnvelopeSample(trigger);
    expect(envelope.kind).toBe("ext.release.completed");
    expect(envelope.source).toBe("extension");
    expect(envelope.workspace_id).toBe("ws_checkout_api");
    expect(envelope.data).toEqual(expect.objectContaining({ metadata: { step: "complete" } }));

    expect(
      buildTriggerDiagnostics(
        makeTrigger({ retry: { strategy: "none", max_retries: 0, base_delay: "" } })
      )
    ).toEqual(expect.arrayContaining([expect.objectContaining({ id: "retry", value: "none" })]));
  });

  it.each([
    {
      event: "memory.consolidated",
      headline: "Memory consolidated",
      phrase: "memory is consolidated",
    },
    {
      event: "hook.release.completed",
      headline: "Hook release completed",
      phrase: "the release hook completes",
    },
    {
      event: "ext.release.deployed",
      headline: "Extension event",
      phrase: "ext.release.deployed fires",
    },
  ])("Should derive supported $event trigger sentences", ({ event, headline, phrase }) => {
    const trigger = makeTrigger({ event });

    expect(
      buildTriggerLede(trigger, "checkout-api")
        .map(segment => segment.text)
        .join("")
    ).toContain(phrase);
    expect(describeTriggerWhen(trigger, "checkout-api").headline).toBe(headline);
  });

  it("Should never offer a manual fire — the daemon has no such route for triggers", () => {
    renderPanel();

    expect(screen.queryByTestId("trigger-job-btn")).not.toBeInTheDocument();
    expect(screen.queryByText("Run now")).not.toBeInTheDocument();
    expect(screen.queryByText(/Next run/)).not.toBeInTheDocument();
  });
});
