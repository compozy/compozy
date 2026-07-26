// Suite: Automation Job form
// Invariant: The live preview and displayed request preserve the selected Job execution target.
// Boundary IN: controlled Job form, pure preview projection, and rendered preview cards.
// Boundary OUT: HTTP submission and persisted reads, owned by route and detail suites.
import { agentFixtures } from "@/systems/agent/mocks";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/loops/hooks/use-loops", async () => {
  const { loopCatalogFixtures } = await import("@/systems/loops/mocks/fixtures");
  return {
    useLoops: () => ({
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isError: false,
      isFetchingNextPage: false,
      isLoading: false,
      loops: loopCatalogFixtures,
    }),
  };
});

import { AutomationJobForm } from "../automation-job-form";
import { createAutomationJobDraft } from "../../lib/automation-drafts";
import type { CreateAutomationJobRequest } from "../../types";

const WORKSPACE_ID = "ws_alpha";
const STORY_AGENTS = agentFixtures;
const WORKSPACES = [
  { id: WORKSPACE_ID, name: "alpha" },
  { id: "ws_beta", name: "beta" },
];

afterEach(() => {
  vi.useRealTimers();
});

interface RenderJobFormOptions {
  activeWorkspaceId?: string | null;
  draft?: CreateAutomationJobRequest;
  isPending?: boolean;
  mode?: "create" | "edit";
  agents?: typeof agentFixtures;
  workspaces?: typeof WORKSPACES;
}

/**
 * Controlled harness: holds the draft via useState and syncs every `onChange`
 * back into state so multi-step interactions (output-mode and schedule-mode
 * switches) re-render against the updated draft.
 */
function renderJobForm({
  activeWorkspaceId = WORKSPACE_ID,
  draft = createAutomationJobDraft(activeWorkspaceId),
  isPending = false,
  mode = "create" as "create" | "edit",
  agents = STORY_AGENTS,
  workspaces = WORKSPACES,
}: RenderJobFormOptions = {}) {
  const onCancel = vi.fn();
  const onChange = vi.fn();
  const onSubmit = vi.fn();

  function Harness() {
    const [currentDraft, setCurrentDraft] = useState<CreateAutomationJobRequest>(draft);

    return (
      <AutomationJobForm
        activeWorkspaceId={activeWorkspaceId}
        agents={agents}
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

  const view = render(<Harness />);

  return { onCancel, onChange, onSubmit, ...view };
}

/**
 * The live preview now swaps in for the form body behind the footer toggle.
 * Both helpers are idempotent so a test can move between the two views freely.
 */
function showPreview() {
  if (screen.queryByTestId("job-preview")) return;
  fireEvent.click(screen.getByTestId("job-preview-toggle"));
}

function showForm() {
  if (screen.queryByTestId("job-name-input")) return;
  fireEvent.click(screen.getByTestId("job-preview-toggle"));
}

describe("AutomationJobForm", () => {
  it("Should update the job name through onChange", () => {
    const { onChange } = renderJobForm();

    fireEvent.change(screen.getByTestId("job-name-input"), {
      target: { value: "nightly-docs" },
    });

    expect(screen.getByTestId("job-name-input")).toHaveValue("nightly-docs");
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ name: "nightly-docs" }));
  });

  it("Should switch scope to global and clear workspace_id", () => {
    const { onChange } = renderJobForm();

    fireEvent.click(screen.getByTestId("job-scope-global"));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: "global", workspace_id: undefined })
    );
    expect(screen.getByTestId("job-scope-global")).toHaveAttribute("aria-pressed", "true");
    // The picker withdrawing *is* the statement that a global job has no
    // workspace — the prose that used to say so pointed at a field it removes.
    expect(screen.queryByTestId("job-workspace-select")).toBeNull();
  });

  it("Should choose a workspace from the command selector", async () => {
    const user = userEvent.setup();
    const { onChange } = renderJobForm();

    await user.click(screen.getByTestId("job-workspace-select"));
    await user.click(screen.getByTestId("job-workspace-item-ws_beta"));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: "workspace", workspace_id: "ws_beta" })
    );
    expect(screen.getByTestId("workspace-switcher-name")).toHaveTextContent("beta");
  });

  it("Should keep scope immutable and omit it from the Job edit preview", () => {
    const { onChange } = renderJobForm({
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        name: "nightly-docs",
      },
      mode: "edit",
    });

    expect(screen.getByTestId("job-scope-global")).toBeDisabled();
    expect(screen.getByTestId("job-scope-workspace")).toBeDisabled();
    expect(screen.getByTestId("job-workspace-select")).toBeDisabled();
    fireEvent.click(screen.getByTestId("job-scope-global"));
    expect(onChange).not.toHaveBeenCalled();

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).not.toHaveTextContent('"scope"');
  });

  it("Should switch output mode to task, reveal task fields, set draft.task, and hide the agent prompt", () => {
    const { onChange } = renderJobForm({
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        name: "reconcile",
        agent_name: "auditor",
        prompt: "Reconcile the ledger.",
      },
    });

    expect(screen.getByTestId("job-prompt-input")).toBeInTheDocument();
    expect(screen.queryByTestId("job-task-title")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("job-target-task"));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        task: expect.objectContaining({ title: "", description: "", owner: null }),
        retry: expect.objectContaining({ strategy: "none" }),
      })
    );
    expect(screen.getByTestId("job-task-title")).toBeInTheDocument();
    expect(screen.getByTestId("job-task-desc")).toBeInTheDocument();
    expect(screen.getByTestId("job-owner-kind")).toBeInTheDocument();
    expect(screen.queryByTestId("job-prompt-input")).not.toBeInTheDocument();
    expect(screen.getByTestId("job-target-task")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should switch back to agent mode and clear draft.task", () => {
    const { onChange } = renderJobForm({
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        name: "reconcile",
        task: { title: "Reconcile", description: "", owner: null },
        retry: { strategy: "none", max_retries: 0, base_delay: "" },
      },
    });

    expect(screen.getByTestId("job-task-title")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("job-target-agent"));

    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ task: undefined }));
    expect(screen.getByTestId("job-prompt-input")).toBeInTheDocument();
    expect(screen.queryByTestId("job-task-title")).not.toBeInTheDocument();
    expect(screen.getByTestId("job-target-agent")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should serialize the Automation Task participation control without legacy fields", () => {
    const { onChange } = renderJobForm();
    fireEvent.click(screen.getByTestId("job-target-task"));

    expect(screen.getByTestId("job-task-participation-mode")).toHaveValue("local");
    fireEvent.change(screen.getByTestId("job-task-participation-mode"), {
      target: { value: "live" },
    });
    fireEvent.change(screen.getByTestId("job-task-participation-channel"), {
      target: { value: "release-room" },
    });
    fireEvent.change(screen.getByTestId("job-task-participation-strategy"), {
      target: { value: "named" },
    });

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        task: expect.objectContaining({
          network_participation: {
            mode: "live",
            channel_id: "release-room",
            channel_strategy: "named",
          },
        }),
      })
    );

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).not.toHaveTextContent(
      /"channel"|"network_channel"|"coordination_channel_id"/
    );
    expect(screen.getByTestId("job-preview-task-participation")).toHaveTextContent("Live");
    expect(screen.getByTestId("job-preview-task-channel")).toHaveTextContent("release-room");
  });

  it("Should switch the target to Run loop with a static input form and no payload mapping (§9.14)", () => {
    const { onChange } = renderJobForm();

    fireEvent.click(screen.getByTestId("job-target-loop"));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        target_kind: "loop",
        loop_target: expect.objectContaining({
          loop_name: "",
          network_participation: { mode: "local" },
        }),
        task: undefined,
      })
    );
    expect(screen.getByTestId("loop-target-fields")).toBeInTheDocument();
    expect(screen.queryByTestId("job-prompt-input")).not.toBeInTheDocument();

    fireEvent.change(screen.getByTestId("loop-target-select"), {
      target: { value: "software-delivery" },
    });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        loop_target: expect.objectContaining({ loop_name: "software-delivery" }),
      })
    );
    // Jobs fire on a schedule, not an event, so there is no payload mapping table.
    expect(screen.queryByTestId("loop-input-mapping")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("loop-input-control").length).toBeGreaterThan(0);

    fireEvent.change(screen.getByTestId("loop-target-participation-mode"), {
      target: { value: "live" },
    });
    expect(screen.getByTestId("submit-job-form")).toBeDisabled();
    fireEvent.change(screen.getByTestId("loop-target-participation-channel"), {
      target: { value: "loop-release-room" },
    });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        loop_target: expect.objectContaining({
          network_participation: {
            mode: "live",
            channel_id: "loop-release-room",
            channel_strategy: "named",
          },
        }),
      })
    );
  });

  it("Should offer only Loops that declare schedule starts", () => {
    renderJobForm();

    fireEvent.click(screen.getByTestId("job-target-loop"));

    const select = screen.getByRole("combobox", { name: "Loop" });
    expect(select).toHaveTextContent("software-delivery");
    expect(select).not.toHaveTextContent("reviews-watch");
  });

  it("Should preserve the explicit Loop workspace when a Job becomes global", () => {
    const { onChange } = renderJobForm();

    fireEvent.click(screen.getByTestId("job-scope-global"));
    fireEvent.click(screen.getByTestId("job-target-loop"));
    fireEvent.change(screen.getByTestId("loop-target-select"), {
      target: { value: "software-delivery" },
    });

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        scope: "global",
        workspace_id: undefined,
        loop_target: expect.objectContaining({
          workspace_id: WORKSPACE_ID,
          loop_name: "software-delivery",
        }),
      })
    );

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      `"workspace_id": "${WORKSPACE_ID}"`
    );
  });

  it("Should atomically rebind a workspace Job and its Loop target", async () => {
    const user = userEvent.setup();
    const { onChange } = renderJobForm();

    fireEvent.click(screen.getByTestId("job-target-loop"));
    await user.click(screen.getByTestId("job-workspace-select"));
    await user.click(screen.getByTestId("job-workspace-item-ws_beta"));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        scope: "workspace",
        workspace_id: "ws_beta",
        loop_target: expect.objectContaining({ workspace_id: "ws_beta" }),
      })
    );

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"workspace_id": "ws_beta"'
    );
  });

  it("Should preserve an incompatible edit target while blocking its preview and submit", () => {
    const { onSubmit } = renderJobForm({
      mode: "edit",
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        name: "reviews-watch-daily",
        agent_name: "",
        prompt: "",
        target_kind: "loop",
        loop_target: {
          workspace_id: WORKSPACE_ID,
          loop_name: "reviews-watch",
          inputs: {},
          input_mapping: {},
        },
      },
    });

    expect(screen.getByRole("combobox", { name: "Loop" })).toHaveValue("reviews-watch");
    expect(screen.getByRole("combobox", { name: "Loop" })).toBeDisabled();
    expect(screen.getByTestId("job-target-agent")).toBeDisabled();
    expect(screen.getByTestId("job-target-task")).toBeDisabled();
    expect(screen.getByTestId("job-target-loop")).toBeDisabled();
    expect(within(screen.getByTestId("loop-target-fields")).getByRole("alert")).toHaveTextContent(
      "reviews-watch does not declare the schedule start kind"
    );

    showPreview();
    expect(screen.getByTestId("job-preview")).toHaveTextContent("Request blocked");
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      "Blocked · PATCH /api/automation/jobs/{id}"
    );
    expect(screen.getByTestId("submit-job-form")).toBeDisabled();

    showForm();
    fireEvent.submit(screen.getByTestId("automation-job-form"));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Should preview a Loop target and its typed request without projecting an empty agent", () => {
    renderJobForm({
      mode: "edit",
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        name: "software-delivery-daily-qa",
        agent_name: "",
        prompt: "",
        target_kind: "loop",
        loop_target: {
          workspace_id: WORKSPACE_ID,
          loop_name: "software-delivery",
          inputs: { slug: "helix-v1-launch", dry_run: false },
          input_mapping: {},
        },
      },
    });

    showPreview();
    const preview = screen.getByTestId("job-preview");
    expect(preview).toHaveTextContent("start Loop software-delivery");
    expect(preview).toHaveTextContent("helix-v1-launch");
    expect(preview).toHaveTextContent("dry_run");
    expect(preview).not.toHaveTextContent("Prompt the agent receives");

    const request = screen.getByTestId("automation-request-payload");
    expect(request).toHaveTextContent("PATCH /api/automation/jobs/{id}");
    expect(request).not.toHaveTextContent('"target_kind"');
    expect(request).toHaveTextContent('"loop_target"');
    expect(request).toHaveTextContent('"slug": "helix-v1-launch"');
  });

  it("Should preserve the agent target in the live preview and displayed request", () => {
    renderJobForm({
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        name: "daily-review",
        agent_name: "reviewer",
        prompt: "Review recent changes.",
      },
    });

    showPreview();
    const preview = screen.getByTestId("job-preview");
    expect(preview).toHaveTextContent("run agent reviewer");
    expect(preview).toHaveTextContent("Prompt the agent receives");
    expect(preview).toHaveTextContent("Review recent changes.");
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"target_kind": "agent"'
    );
  });

  it("Should refresh cron-relative labels at the next minute boundary", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-21T12:00:59.000Z"));
    const { unmount } = renderJobForm({
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        schedule: { mode: "cron", expr: "* * * * *" },
      },
    });

    showPreview();
    expect(screen.getByTestId("job-preview")).toHaveTextContent("in 1s");
    act(() => vi.advanceTimersByTime(1_000));
    expect(screen.getByTestId("job-preview")).toHaveTextContent("in 1 min");

    unmount();
  });

  it("Should align the preview boundary after an invalid cron becomes valid", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-21T12:00:10.000Z"));
    const { unmount } = renderJobForm({
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        schedule: { mode: "cron", expr: "invalid" },
      },
    });

    act(() => vi.advanceTimersByTime(30_000));
    fireEvent.change(screen.getByLabelText("Cron expression"), {
      target: { value: "* * * * *" },
    });

    showPreview();
    expect(screen.getByTestId("job-preview")).toHaveTextContent("in 20s");
    act(() => vi.advanceTimersByTime(20_000));
    expect(screen.getByTestId("job-preview")).toHaveTextContent("in 1 min");

    unmount();
  });

  it("Should switch the schedule mode to every and then to at", () => {
    const { onChange } = renderJobForm();

    fireEvent.click(screen.getByTestId("job-schedule-mode-every"));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ schedule: { mode: "every", interval: "30m" } })
    );
    expect(screen.getByLabelText("Interval")).toHaveValue("30m");
    expect(screen.getByTestId("job-schedule-mode-every")).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(screen.getByTestId("job-schedule-mode-at"));

    const lastCall = onChange.mock.calls.at(-1)?.[0] as CreateAutomationJobRequest;
    expect(lastCall.schedule.mode).toBe("at");
    expect(typeof lastCall.schedule.time).toBe("string");
    expect(lastCall.schedule.time).not.toBe("");
    expect(screen.getByLabelText("Run date and time")).toBeInTheDocument();
    expect(screen.getByTestId("job-schedule-mode-at")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should preserve recurring catch-up and grace across schedule-mode switches but omit them for one-shot at", () => {
    renderJobForm();
    fireEvent.click(screen.getByTestId("job-governance-toggle"));

    expect(screen.getByRole("switch", { name: "Enabled on create" })).toBe(
      screen.getByTestId("job-enabled-toggle")
    );

    // Recurring (cron) exposes catch-up + grace; entering values serializes them.
    expect(screen.getByTestId("job-catch-up-field")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("job-catch-up-replay"));
    fireEvent.change(screen.getByTestId("job-misfire-grace"), { target: { value: "45" } });

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"catch_up_policy": "replay"'
    );
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"misfire_grace_seconds": 45'
    );

    // One-shot `at` hides the controls and drops both fields from the request.
    showForm();
    fireEvent.click(screen.getByTestId("job-schedule-mode-at"));
    expect(screen.queryByTestId("job-catch-up-field")).not.toBeInTheDocument();
    expect(screen.queryByTestId("job-misfire-grace")).not.toBeInTheDocument();
    showPreview();
    expect(screen.getByTestId("automation-request-payload")).not.toHaveTextContent(
      "catch_up_policy"
    );
    expect(screen.getByTestId("automation-request-payload")).not.toHaveTextContent(
      "misfire_grace_seconds"
    );

    // Returning to cron restores the entered recurring values.
    showForm();
    fireEvent.click(screen.getByTestId("job-schedule-mode-cron"));
    expect(screen.getByTestId("job-catch-up-replay")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("job-misfire-grace")).toHaveValue(45);
    showPreview();
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"catch_up_policy": "replay"'
    );
  });

  it("Should omit catch_up_policy when the target-aware Default is selected", () => {
    renderJobForm();
    fireEvent.click(screen.getByTestId("job-governance-toggle"));

    fireEvent.click(screen.getByTestId("job-catch-up-coalesce"));

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"catch_up_policy": "coalesce"'
    );

    showForm();
    fireEvent.click(screen.getByTestId("job-catch-up-default"));
    expect(screen.getByTestId("job-catch-up-default")).toHaveAttribute("aria-pressed", "true");
    showPreview();
    expect(screen.getByTestId("automation-request-payload")).not.toHaveTextContent(
      "catch_up_policy"
    );
  });

  it("Should treat zero or empty grace as the scheduler default by omitting misfire_grace_seconds", () => {
    renderJobForm();
    fireEvent.click(screen.getByTestId("job-governance-toggle"));

    fireEvent.change(screen.getByTestId("job-misfire-grace"), { target: { value: "30" } });

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).toHaveTextContent(
      '"misfire_grace_seconds": 30'
    );

    // Re-query after the swap: the preview unmounts the form body, so a handle
    // captured before it points at a detached node.
    showForm();
    const grace = screen.getByTestId("job-misfire-grace");
    fireEvent.change(grace, { target: { value: "0" } });
    expect(grace).toHaveValue(0);

    showPreview();
    expect(screen.getByTestId("automation-request-payload")).not.toHaveTextContent(
      "misfire_grace_seconds"
    );
  });

  it("Should keep a decodable cron expression editable after selecting Custom", () => {
    const { onChange } = renderJobForm();
    const cronInput = screen.getByLabelText("Cron expression");

    expect(cronInput).toHaveAttribute("readonly");

    fireEvent.click(screen.getByRole("button", { name: "Custom" }));
    expect(cronInput).not.toHaveAttribute("readonly");

    fireEvent.change(cronInput, { target: { value: "*/15 * * * *" } });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ schedule: { mode: "cron", expr: "*/15 * * * *" } })
    );
    expect(cronInput).not.toHaveAttribute("readonly");

    fireEvent.click(screen.getByRole("button", { name: "Hourly" }));
    expect(cronInput).toHaveAttribute("readonly");
  });

  it("Should disable submit when name, agent, or prompt are empty and enable + call onSubmit when valid", () => {
    const { onChange, onSubmit } = renderJobForm({
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        name: "",
        agent_name: "",
        prompt: "",
      },
    });

    expect(screen.getByTestId("submit-job-form")).toBeDisabled();

    fireEvent.change(screen.getByTestId("job-name-input"), {
      target: { value: "daily-review" },
    });
    expect(screen.getByTestId("submit-job-form")).toBeDisabled();

    // The agent target is a searchable catalog selector, not a text input.
    fireEvent.click(screen.getByTestId("job-agent-input"));
    fireEvent.click(screen.getByTestId(`agent-command-item-${agentFixtures[0].name}`));
    expect(screen.getByTestId("submit-job-form")).toBeDisabled();

    fireEvent.change(screen.getByTestId("job-prompt-input"), {
      target: { value: "Review recent changes." },
    });

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        name: "daily-review",
        agent_name: agentFixtures[0].name,
        prompt: "Review recent changes.",
      })
    );
    expect(screen.getByTestId("submit-job-form")).toBeEnabled();

    fireEvent.click(screen.getByTestId("submit-job-form"));
    expect(onSubmit).toHaveBeenCalledOnce();
  });

  it("Should not submit while a pending save is in flight", () => {
    const { onSubmit } = renderJobForm({
      draft: {
        ...createAutomationJobDraft(WORKSPACE_ID),
        name: "daily-review",
        agent_name: "reviewer",
        prompt: "Review recent changes.",
      },
      isPending: true,
    });

    expect(screen.getByTestId("submit-job-form")).toHaveTextContent("Saving...");

    fireEvent.submit(screen.getByTestId("automation-job-form"));

    expect(onSubmit).not.toHaveBeenCalled();
  });
});
