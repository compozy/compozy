// Suite: Automation Trigger form
// Invariant: The live preview and displayed request preserve the selected Trigger target union.
// Boundary IN: controlled Trigger form, pure preview projection, and rendered preview cards.
// Boundary OUT: HTTP submission and persisted reads, owned by route and detail suites.
import { agentFixtures } from "@/systems/agent/mocks";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const loopsState = vi.hoisted(() => ({ current: [] as unknown[] }));

vi.mock("@/systems/loops/hooks/use-loops", () => ({
  useLoops: () => ({
    error: null,
    fetchNextPage: vi.fn(),
    hasNextPage: false,
    isError: false,
    isFetchingNextPage: false,
    isLoading: false,
    loops: loopsState.current,
  }),
}));

import { AutomationTriggerForm } from "../automation-trigger-form";
import { createAutomationTriggerDraft } from "../../lib/automation-drafts";
import type { CreateAutomationTriggerRequest } from "../../types";

interface RenderTriggerFormOptions {
  activeWorkspaceId?: string | null;
  draft?: CreateAutomationTriggerRequest;
  isPending?: boolean;
  mode?: "create" | "edit";
  workspaces?: Array<{ id: string; name: string }>;
}

const WORKSPACES = [
  { id: "ws_alpha", name: "alpha" },
  { id: "ws_beta", name: "beta" },
];

function renderTriggerForm({
  activeWorkspaceId = "ws_alpha",
  draft = createAutomationTriggerDraft(activeWorkspaceId),
  isPending = false,
  mode = "create" as "create" | "edit",
  workspaces = WORKSPACES,
}: RenderTriggerFormOptions = {}) {
  const onCancel = vi.fn();
  const onChange = vi.fn();
  const onSubmit = vi.fn();

  function Harness() {
    const [currentDraft, setCurrentDraft] = useState<CreateAutomationTriggerRequest>(draft);

    return (
      <AutomationTriggerForm
        activeWorkspaceId={activeWorkspaceId}
        agents={agentFixtures}
        draft={currentDraft}
        isPending={isPending}
        mode={mode}
        onCancel={onCancel}
        onChange={nextDraft => {
          onChange(nextDraft);
          setCurrentDraft(nextDraft);
        }}
        onSubmit={onSubmit}
        workspaces={workspaces}
      />
    );
  }

  render(<Harness />);

  return { onCancel, onChange, onSubmit };
}

function fillIdentity() {
  fireEvent.change(screen.getByTestId("trigger-name-input"), {
    target: { value: "push-review" },
  });
  // The agent target is a searchable catalog selector, not a text input.
  fireEvent.click(screen.getByTestId("trigger-agent-input"));
  fireEvent.click(screen.getByTestId(`agent-command-item-${agentFixtures[0].name}`));
  fireEvent.change(screen.getByTestId("trigger-prompt-input"), {
    target: { value: "Review {{ .Kind }} for session {{ .Data.session_id }}." },
  });
}

/**
 * The live preview now swaps in for the form body behind the footer toggle.
 * Both helpers are idempotent so a test can move between the two views freely.
 */
function showPreview() {
  if (screen.queryByTestId("trigger-preview")) return;
  fireEvent.click(screen.getByTestId("trigger-preview-toggle"));
}

function showForm() {
  if (screen.queryByTestId("trigger-name-input")) return;
  fireEvent.click(screen.getByTestId("trigger-preview-toggle"));
}

describe("AutomationTriggerForm", () => {
  beforeEach(async () => {
    const { loopCatalogFixtures } = await import("@/systems/loops/mocks/fixtures");
    const softwareDelivery = loopCatalogFixtures.find(loop => loop.name === "software-delivery");
    const reviewsWatch = loopCatalogFixtures.find(loop => loop.name === "reviews-watch");
    if (!softwareDelivery || !reviewsWatch) throw new Error("Loop fixtures are incomplete");

    loopsState.current = [
      { ...softwareDelivery, start: [{ kind: "schedule" }, { kind: "trigger" }] },
      {
        ...reviewsWatch,
        inputs: { pr: { type: "number", required: true } },
        start: [{ kind: "trigger" }],
      },
      { ...reviewsWatch, name: "webhook-intake", start: [{ kind: "webhook" }] },
    ];
  });

  it("selects a webhook event, forces global scope, and submits the webhook payload", () => {
    const { onCancel, onChange, onSubmit } = renderTriggerForm();

    const footer = document.body.querySelector('[data-slot="dialog-footer"]');
    expect(footer).not.toBeNull();
    expect(footer).toHaveAttribute("data-variant", "ruled");
    expect(screen.getByTestId("submit-trigger-form")).toBeDisabled();

    fillIdentity();
    // session.stopped (the default) in a workspace is already submittable.
    expect(screen.getByTestId("submit-trigger-form")).toBeEnabled();

    fireEvent.click(screen.getByTestId("trigger-event-webhook"));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ event: "webhook", scope: "global", workspace_id: undefined })
    );
    // webhook needs its signed-endpoint fields before it can be created.
    expect(screen.getByTestId("submit-trigger-form")).toBeDisabled();

    fireEvent.change(screen.getByTestId("trigger-endpoint-slug-input"), {
      target: { value: "repo-push" },
    });
    fireEvent.change(screen.getByTestId("trigger-webhook-id-input"), {
      target: { value: "wbh_repo_push" },
    });
    fireEvent.change(screen.getByTestId("trigger-webhook-secret-value-input"), {
      target: { value: "shared-secret" },
    });

    expect(screen.getByTestId("submit-trigger-form")).toBeEnabled();

    fireEvent.click(screen.getByTestId("submit-trigger-form"));
    fireEvent.click(screen.getByText("Cancel"));

    expect(onSubmit).toHaveBeenCalledOnce();
    expect(onSubmit.mock.calls[0]).toEqual([]);
    expect(onCancel).toHaveBeenCalledOnce();
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        event: "webhook",
        scope: "global",
        workspace_id: undefined,
        endpoint_slug: "repo-push",
        webhook_id: "wbh_repo_push",
        webhook_secret_value: "shared-secret",
      })
    );

    showPreview();
    const requestPayload = screen.getByTestId("automation-request-payload");
    expect(requestPayload).not.toHaveTextContent("shared-secret");
    expect(requestPayload).toHaveTextContent("[redacted]");
    expect(requestPayload).toHaveTextContent("write-only values redacted");
  });

  it("composes a hook event id from the inline sub-config", () => {
    const { onChange } = renderTriggerForm();

    fireEvent.click(screen.getByTestId("trigger-event-hook.completed"));
    fireEvent.change(screen.getByTestId("trigger-hook-name-input"), {
      target: { value: "transform" },
    });

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ event: "hook.transform.completed" })
    );
  });

  it("toggles scope between workspace and global for non-webhook events", () => {
    const { onChange } = renderTriggerForm();

    fireEvent.click(screen.getByTestId("trigger-scope-global"));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: "global", workspace_id: undefined })
    );

    fireEvent.click(screen.getByTestId("trigger-scope-workspace"));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: "workspace", workspace_id: "ws_alpha" })
    );
  });

  it("selects a different workspace from the command selector", async () => {
    const user = userEvent.setup();
    const { onChange } = renderTriggerForm();

    await user.click(screen.getByTestId("trigger-workspace-select"));
    await user.click(screen.getByTestId("trigger-workspace-item-ws_beta"));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: "workspace", workspace_id: "ws_beta" })
    );
    expect(screen.getByTestId("workspace-switcher-name")).toHaveTextContent("beta");
  });

  it("Should keep scope immutable and omit it from the Trigger edit preview", () => {
    const { onChange } = renderTriggerForm({
      draft: {
        ...createAutomationTriggerDraft("ws_alpha"),
        name: "review-completions",
      },
      mode: "edit",
    });

    expect(screen.getByTestId("trigger-scope-global")).toBeDisabled();
    expect(screen.getByTestId("trigger-scope-workspace")).toBeDisabled();
    expect(screen.getByTestId("trigger-workspace-select")).toBeDisabled();
    fireEvent.click(screen.getByTestId("trigger-scope-global"));
    expect(onChange).not.toHaveBeenCalled();

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).not.toHaveTextContent('"scope"');
  });

  it("Should keep webhook unavailable while editing a workspace-scoped Trigger", () => {
    const { onChange } = renderTriggerForm({
      draft: {
        ...createAutomationTriggerDraft("ws_alpha"),
        name: "review-completions",
      },
      mode: "edit",
    });

    const webhook = screen.getByTestId("trigger-event-webhook");
    expect(webhook).toBeDisabled();
    fireEvent.click(webhook);
    expect(onChange).not.toHaveBeenCalled();

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).not.toHaveTextContent("webhook");
  });

  it("adds and edits structured filter conditions as an AND map", () => {
    const { onChange } = renderTriggerForm();

    fireEvent.click(screen.getByRole("button", { name: "Add condition" }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ filter: { kind: "" } }));

    fireEvent.change(screen.getByTestId("trigger-filter-value-0"), {
      target: { value: "session.stopped" },
    });
    fireEvent.change(screen.getByTestId("trigger-filter-key-0"), {
      target: { value: "data.stop_reason" },
    });

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ filter: { "data.stop_reason": "session.stopped" } })
    );

    fireEvent.click(screen.getByTestId("trigger-filter-remove-0"));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ filter: {} }));
  });

  it("hides webhook sub-config fields for non-webhook events", () => {
    renderTriggerForm({
      draft: {
        ...createAutomationTriggerDraft("ws_alpha"),
        name: "session-review",
        agent_name: agentFixtures[0].name,
        prompt: "Review stopped session",
        event: "session.stopped",
      },
    });

    expect(screen.queryByTestId("trigger-endpoint-slug-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("trigger-webhook-id-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("trigger-webhook-secret-value-input")).not.toBeInTheDocument();
  });

  it("resets trigger retry values when switching back to none", () => {
    const { onChange } = renderTriggerForm();

    fireEvent.click(screen.getByTestId("trigger-governance-toggle"));

    expect(screen.getByRole("switch", { name: "Trigger enabled" })).toBe(
      screen.getByTestId("trigger-enabled-toggle")
    );

    expect(screen.getByTestId("trigger-retry-max")).toBeDisabled();
    expect(screen.getByTestId("trigger-retry-max")).toHaveValue(0);
    expect(screen.getByTestId("trigger-retry-delay")).toHaveValue("");

    fireEvent.click(screen.getByTestId("trigger-retry-strategy-backoff"));
    fireEvent.change(screen.getByTestId("trigger-retry-max"), {
      target: { value: "6" },
    });
    fireEvent.change(screen.getByTestId("trigger-retry-delay"), {
      target: { value: "9s" },
    });
    fireEvent.click(screen.getByTestId("trigger-retry-strategy-none"));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        retry: { strategy: "none", max_retries: 0, base_delay: "" },
      })
    );
    expect(screen.getByTestId("trigger-retry-max")).toBeDisabled();
    expect(screen.getByTestId("trigger-retry-max")).toHaveValue(0);
    expect(screen.getByTestId("trigger-retry-delay")).toHaveValue("");
  });

  it("renders edit and pending labels without submitting", () => {
    const { onSubmit } = renderTriggerForm({
      draft: {
        ...createAutomationTriggerDraft("ws_alpha"),
        name: "push-review",
        agent_name: agentFixtures[0].name,
        prompt: "Review push event.",
        event: "session.stopped",
      },
      isPending: true,
      mode: "edit",
    });

    expect(screen.getByTestId("submit-trigger-form")).toHaveTextContent("Saving...");

    fireEvent.submit(screen.getByTestId("automation-trigger-form"));

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("switches the Then step to Run loop, seeds a loop target, and picks a loop (§9.14)", () => {
    const { onChange } = renderTriggerForm();

    // Agent target is the default.
    expect(screen.getByTestId("trigger-agent-input")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("target-mode-loop"));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        target_kind: "loop",
        loop_target: expect.objectContaining({
          loop_name: "",
          network_participation: { mode: "local" },
        }),
      })
    );
    // The loop picker + payload mapping table replace the agent prompt.
    expect(screen.getByTestId("loop-target-fields")).toBeInTheDocument();
    expect(screen.queryByTestId("trigger-agent-input")).not.toBeInTheDocument();

    fireEvent.change(screen.getByTestId("loop-target-participation-mode"), {
      target: { value: "live" },
    });
    fireEvent.change(screen.getByTestId("loop-target-participation-channel"), {
      target: { value: "trigger-room" },
    });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        loop_target: expect.objectContaining({
          network_participation: {
            mode: "live",
            channel_id: "trigger-room",
            channel_strategy: "named",
          },
        }),
      })
    );

    fireEvent.change(screen.getByTestId("loop-target-select"), {
      target: { value: "software-delivery" },
    });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        loop_target: expect.objectContaining({ loop_name: "software-delivery" }),
      })
    );
    // A trigger fires from an event, so the payload mapping table is present.
    expect(screen.getByTestId("loop-input-mapping")).toBeInTheDocument();
  });

  it("Should filter Loop targets by trigger versus webhook start kinds", () => {
    const { onChange } = renderTriggerForm();
    fireEvent.click(screen.getByTestId("target-mode-loop"));

    let select = screen.getByRole("combobox", { name: "Loop" });
    expect(select).toHaveTextContent("software-delivery");
    expect(select).toHaveTextContent("reviews-watch");
    expect(select).not.toHaveTextContent("webhook-intake");

    fireEvent.click(screen.getByTestId("trigger-event-webhook"));
    select = screen.getByRole("combobox", { name: "Loop" });
    expect(select).toHaveTextContent("webhook-intake");
    expect(select).not.toHaveTextContent("software-delivery");
    expect(select).not.toHaveTextContent("reviews-watch");
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        loop_target: expect.objectContaining({ loop_name: "" }),
        target_kind: "loop",
      })
    );
  });

  it("Should bind a non-webhook Loop target to the selected workspace after leaving webhook", () => {
    const { onChange, onSubmit } = renderTriggerForm();

    fireEvent.change(screen.getByTestId("trigger-name-input"), {
      target: { value: "review-on-stop" },
    });
    fireEvent.click(screen.getByTestId("trigger-event-webhook"));
    fireEvent.click(screen.getByTestId("trigger-event-session.stopped"));
    fireEvent.click(screen.getByTestId("target-mode-loop"));
    fireEvent.change(screen.getByTestId("loop-target-select"), {
      target: { value: "reviews-watch" },
    });
    fireEvent.change(screen.getByTestId("loop-input-field-pr"), {
      target: { value: "2" },
    });
    fireEvent.click(screen.getByTestId("trigger-scope-workspace"));
    fireEvent.click(screen.getByTestId("trigger-workspace-select"));
    fireEvent.click(screen.getByTestId("trigger-workspace-item-ws_beta"));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        loop_target: expect.objectContaining({
          inputs: { pr: 2 },
          loop_name: "reviews-watch",
          workspace_id: "ws_beta",
        }),
        scope: "workspace",
        workspace_id: "ws_beta",
      })
    );
    showPreview();
    expect(screen.getByTestId("automation-target-details")).toHaveTextContent("ws_beta");
    expect(screen.getByTestId("automation-target-details")).not.toHaveTextContent("Not selected");

    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"workspace_id": "ws_beta"'
    );
    expect(screen.getByTestId("submit-trigger-form")).toBeEnabled();

    fireEvent.click(screen.getByTestId("submit-trigger-form"));
    expect(onSubmit).toHaveBeenCalledOnce();
  });

  it("Should retain an edit target when an event change makes its start kind incompatible", () => {
    const { onSubmit } = renderTriggerForm({
      mode: "edit",
      draft: {
        ...createAutomationTriggerDraft(null),
        event: "session.stopped",
        name: "delivery-trigger",
        agent_name: "",
        prompt: "",
        target_kind: "loop",
        loop_target: {
          workspace_id: "ws_alpha",
          loop_name: "software-delivery",
          inputs: { goal: "Ship it" },
          input_mapping: {},
        },
      },
    });

    expect(screen.getByTestId("submit-trigger-form")).toBeEnabled();
    fireEvent.click(screen.getByTestId("trigger-event-webhook"));

    expect(screen.getByRole("combobox", { name: "Loop" })).toHaveValue("software-delivery");
    expect(screen.getByRole("combobox", { name: "Loop" })).toBeDisabled();
    expect(screen.getByTestId("target-mode-agent")).toBeDisabled();
    expect(screen.getByTestId("target-mode-loop")).toBeDisabled();
    expect(within(screen.getByTestId("loop-target-fields")).getByRole("alert")).toHaveTextContent(
      "software-delivery does not declare the webhook start kind"
    );

    showPreview();
    expect(screen.getByTestId("trigger-preview")).toHaveTextContent("Request blocked");
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      "Blocked · PATCH /api/automation/triggers/{id}"
    );
    expect(screen.getByTestId("submit-trigger-form")).toBeDisabled();

    showForm();
    fireEvent.submit(screen.getByTestId("automation-trigger-form"));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Should preview a Loop target with static inputs, event mappings, and its request union", () => {
    renderTriggerForm({
      mode: "edit",
      draft: {
        ...createAutomationTriggerDraft("ws_alpha"),
        name: "delivery-on-stop",
        agent_name: "",
        prompt: "",
        target_kind: "loop",
        loop_target: {
          workspace_id: "ws_alpha",
          loop_name: "software-delivery",
          inputs: { slug: "helix-v1-launch" },
          input_mapping: { branch: "data.branch" },
        },
      },
    });

    showPreview();
    const preview = screen.getByTestId("trigger-preview");
    expect(preview).toHaveTextContent("start Loop software-delivery");
    expect(preview).toHaveTextContent("helix-v1-launch");
    expect(preview).toHaveTextContent("data.branch");
    expect(preview).not.toHaveTextContent("Prompt the agent receives");

    const request = screen.getByTestId("automation-request-payload");
    expect(request).toHaveTextContent("PATCH /api/automation/triggers/{id}");
    expect(request).not.toHaveTextContent('"target_kind"');
    expect(request).toHaveTextContent('"input_mapping"');
  });

  it("Should preserve the agent target in the Trigger preview and displayed request", () => {
    renderTriggerForm({
      draft: {
        ...createAutomationTriggerDraft("ws_alpha"),
        name: "push-review",
        agent_name: agentFixtures[0].name,
        prompt: "Review stopped session.",
      },
    });

    showPreview();
    const preview = screen.getByTestId("trigger-preview");
    expect(preview).toHaveTextContent(`run ${agentFixtures[0].name}`);
    expect(preview).toHaveTextContent("Prompt the agent receives");
    expect(preview).toHaveTextContent("Review stopped session.");
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"target_kind": "agent"'
    );
  });
});
