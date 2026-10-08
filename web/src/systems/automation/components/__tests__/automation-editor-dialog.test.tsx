// Suite: Automation editor (S3)
// Invariant: one dialog builds any automation; the draft, sentence bar, readiness and
// displayed request agree, and edit mode locks what the daemon can't change.
// Boundary IN: AutomationEditorDialog + AutomationForm + useAutomationForm over a controlled draft.
// Boundary OUT: save mutations and navigation (use-automation-editor suite).
import { agentFixtures } from "@/systems/agent/mocks";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  render as renderTestingLibrary,
  screen,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

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

const aggregateDestination = vi.hoisted(() => ({ value: null as string | null }));
vi.mock("@/systems/profiles", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles")>()),
  useAggregateDestination: () => aggregateDestination.value,
}));

import { AutomationEditorDialog } from "../automation-editor-dialog";
import {
  automationCondition,
  createAutomationFormDraft,
  type AutomationEditorSection,
  type AutomationFormDraft,
} from "../../lib/automation-form-draft";

const WORKSPACES = [
  { id: "ws_test", name: "checkout-api" },
  { id: "ws_beta", name: "beta-workspace" },
];
const AGENT = agentFixtures[0].name;

function render(ui: ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderTestingLibrary(ui, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  });
}

interface HarnessProps {
  draft?: AutomationFormDraft;
  mode?: "create" | "edit";
  lockedLoop?: string;
  section?: AutomationEditorSection;
  agents?: typeof agentFixtures;
  agentsLoading?: boolean;
  isPending?: boolean;
  submitError?: string | null;
  submitErrorField?: "name" | null;
  onSubmit?: (draft: AutomationFormDraft) => void;
}

function EditorHarness({
  draft: initial,
  mode = "create",
  lockedLoop,
  section,
  agents = agentFixtures,
  agentsLoading,
  isPending = false,
  submitError = null,
  submitErrorField = null,
  onSubmit = vi.fn(),
}: HarnessProps) {
  const [draft, setDraft] = useState<AutomationFormDraft>(
    () => initial ?? createAutomationFormDraft("ws_test")
  );
  return (
    <AutomationEditorDialog
      activeWorkspaceId="ws_test"
      agents={agents}
      agentsLoading={agentsLoading}
      editor={{
        draft,
        isPending,
        lockedLoop,
        mode,
        onCancel: vi.fn(),
        onChange: setDraft,
        onSubmit: () => onSubmit(draft),
        section,
        submitError,
        submitErrorField,
      }}
      workspaces={WORKSPACES}
    />
  );
}

function readyDraft(overrides: Partial<AutomationFormDraft> = {}): AutomationFormDraft {
  return {
    ...createAutomationFormDraft("ws_test"),
    name: "morning-digest",
    agent_name: AGENT,
    prompt: "Summarize yesterday's sessions.",
    ...overrides,
  };
}

const sentence = () => screen.getByTestId("automation-editor-sentence");
const status = () => screen.getByTestId("automation-editor-status");
const submit = () => screen.getByTestId("automation-form-submit");
const readout = () => screen.getByTestId("automation-schedule-readout");

function sectionTitles(): string[] {
  return screen
    .getAllByRole("heading", { level: 3 })
    .map(heading => heading.textContent ?? "")
    .filter(text => /^\d{2}/.test(text));
}

function pickAgent(name = AGENT) {
  fireEvent.click(screen.getByTestId("automation-agent-input"));
  fireEvent.click(screen.getByTestId(`agent-command-item-${name}`));
}

function togglePreview() {
  fireEvent.click(screen.getByTestId("automation-preview-toggle"));
}

beforeEach(() => {
  aggregateDestination.value = null;
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-10-07T19:00:00Z"));
});

afterEach(() => {
  vi.useRealTimers();
});

describe("AutomationEditorDialog", () => {
  it("UT-090 shows Name, Starts, Does and Options, and an event start inserts Only if before Does", () => {
    render(<EditorHarness />);

    const header = screen
      .getByTestId("automation-editor-dialog")
      .querySelector('[data-slot="dialog-header"]') as HTMLElement;
    expect(within(header).getByText("Automation")).toBeInTheDocument();
    expect(within(header).getByText("New automation")).toBeInTheDocument();
    expect(header).toHaveTextContent(
      "Choose when it starts and what it does. You can turn it off any time."
    );
    expect(sectionTitles()).toEqual(["01Name", "02Starts", "03Does"]);
    expect(screen.getByTestId("automation-form-options")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("automation-start-event"));

    expect(sectionTitles()).toEqual(["01Name", "02Starts", "03Only if", "04Does"]);
  });

  it("UT-091 rewrites the sentence on each change and gates Create on Ready", () => {
    const onSubmit = vi.fn();
    render(<EditorHarness onSubmit={onSubmit} />);

    expect(sentence()).toHaveAttribute("aria-live", "polite");
    expect(sentence()).toHaveTextContent("Every day at 09:00 UTC, ask an agent.");
    expect(status()).toHaveTextContent("Needs a fix");
    expect(submit()).toBeDisabled();
    expect(submit()).toHaveTextContent("Create automation");

    fireEvent.change(screen.getByTestId("automation-name-input"), {
      target: { value: "morning-digest" },
    });
    pickAgent();
    fireEvent.change(screen.getByTestId("automation-prompt-input"), {
      target: { value: "Summarize yesterday." },
    });

    expect(sentence()).toHaveTextContent(
      `Every day at 09:00 UTC, ask ${AGENT} to summarize yesterday.`
    );
    expect(status()).toHaveTextContent("Ready");
    expect(submit()).toBeEnabled();

    fireEvent.click(submit());
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ name: "morning-digest", agent_name: AGENT, start: "schedule" })
    );
  });

  it("UT-092 states where the automation lands: the project, or Global for links", () => {
    render(<EditorHarness />);

    expect(screen.getByTestId("automation-destination")).toHaveTextContent(
      "Creates in checkout-api."
    );

    fireEvent.click(screen.getByTestId("automation-start-webhook"));

    expect(screen.getByTestId("automation-destination")).toHaveTextContent(
      "Creates a Global automation."
    );
  });

  it("Should show the aggregate destination for a new automation", () => {
    aggregateDestination.value = "default";
    render(<EditorHarness />);

    expect(screen.getByTestId("profile-destination-chip")).toHaveTextContent("default");
  });

  it("UT-093 puts a name conflict on the Name field and any other save error in the dialog", () => {
    const message = "An automation named morning-digest already exists.";
    const { unmount } = render(
      <EditorHarness draft={readyDraft()} submitError={message} submitErrorField="name" />
    );

    const nameField = screen.getByTestId("automation-name-input").closest('[data-slot="field"]');
    expect(nameField).toHaveTextContent(message);
    expect(screen.queryByTestId("automation-form-error")).not.toBeInTheDocument();
    unmount();

    render(
      <EditorHarness draft={readyDraft()} mode="edit" submitError="automation: job was changed" />
    );
    expect(screen.getByTestId("automation-form-error")).toHaveTextContent(
      "automation: job was changed"
    );
    expect(screen.getByTestId("automation-name-input")).toHaveValue("morning-digest");
  });

  it("UT-094 builds a schedule from quick picks and days, and an empty day set needs a fix", () => {
    render(<EditorHarness draft={readyDraft()} />);

    fireEvent.click(screen.getByRole("button", { name: "Weekdays 9am" }));
    expect(readout()).toHaveTextContent("Every weekday at 09:00 UTC · next in 14h · 0 9 * * 1-5");
    expect(screen.getByRole("button", { name: "Weekdays 9am" })).toHaveAttribute(
      "aria-pressed",
      "true"
    );

    fireEvent.click(screen.getByRole("button", { name: "Mondays 8am" }));
    fireEvent.click(screen.getByRole("button", { name: "Monday" }));

    expect(sentence()).toHaveTextContent("on some days");
    expect(readout()).toHaveTextContent("Pick at least one day.");
    expect(status()).toHaveTextContent("Needs a fix");

    fireEvent.click(screen.getByRole("button", { name: "Friday" }));
    expect(readout()).toHaveTextContent("Every Friday at 08:00 UTC · next in 1d 13h · 0 8 * * 5");
    expect(status()).toHaveTextContent("Ready");
  });

  it("UT-095 reads out Every… and Once in plain words", () => {
    render(<EditorHarness draft={readyDraft()} />);

    fireEvent.click(screen.getByTestId("automation-schedule-mode-every"));
    fireEvent.click(screen.getByRole("button", { name: "30m" }));
    expect(readout()).toHaveTextContent("Runs every 30 minutes, starting right after you save.");

    fireEvent.click(screen.getByTestId("automation-schedule-mode-at"));
    fireEvent.change(screen.getByLabelText("Date and time"), {
      target: { value: "2026-10-08T09:00" },
    });
    expect(readout()).toHaveTextContent("Runs once, in 14h (Thu Oct 8, 09:00 UTC), then stops.");
    expect(sentence()).toHaveTextContent("Once on Thu Oct 8 at 09:00 UTC,");
  });

  it("UT-096 flags a past time and a malformed expression, and offers no time-zone control", () => {
    render(<EditorHarness draft={readyDraft()} />);

    fireEvent.click(screen.getByRole("button", { name: "Edit expression" }));
    fireEvent.change(screen.getByLabelText("Cron expression"), { target: { value: "0 9 * *" } });
    expect(screen.getByLabelText("Cron expression")).toHaveAttribute("aria-invalid", "true");
    expect(readout()).toHaveTextContent("Needs 5 parts: min · hour · day · month · weekday.");
    expect(status()).toHaveTextContent("Needs a fix");

    fireEvent.click(screen.getByTestId("automation-schedule-mode-at"));
    fireEvent.change(screen.getByLabelText("Date and time"), {
      target: { value: "2026-10-01T09:00" },
    });
    expect(readout()).toHaveTextContent("That time is in the past. It would never run.");
    expect(screen.queryByLabelText(/time zone/i)).not.toBeInTheDocument();
  });

  it("UT-097 offers exactly four events, and a hook start needs its name", () => {
    render(<EditorHarness draft={readyDraft()} />);
    fireEvent.click(screen.getByTestId("automation-start-event"));

    const events = within(screen.getByRole("radiogroup", { name: "What happens" }))
      .getAllByRole("radio")
      .map(card => card.textContent);
    expect(events).toEqual([
      expect.stringContaining("A session starts"),
      expect.stringContaining("A session stops"),
      expect.stringContaining("A hook finishes"),
      expect.stringContaining("An extension sends an event"),
    ]);
    expect(screen.queryByText(/memory/i)).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("automation-event-hook.completed"));

    expect(screen.getByLabelText("Hook name")).toBeInTheDocument();
    expect(screen.getByText(/fires\./)).toHaveTextContent("Runs when hook.<name>.completed fires.");
    expect(sentence()).toHaveTextContent("When a hook completes");
    expect(status()).toHaveTextContent("Needs a fix");

    fireEvent.change(screen.getByLabelText("Hook name"), { target: { value: "transform" } });
    expect(sentence()).toHaveTextContent("When the transform hook completes in checkout-api");
    expect(status()).toHaveTextContent("Ready");
  });

  it("UT-098 shows the link fields with a write-only secret the request redacts", () => {
    render(<EditorHarness draft={readyDraft()} />);
    fireEvent.click(screen.getByTestId("automation-start-webhook"));

    expect(screen.getByTestId("automation-webhook-global-note")).toHaveTextContent(
      "Link automations are always Global. They aren't tied to one project."
    );
    fireEvent.change(screen.getByLabelText("Link name"), { target: { value: "deploy" } });
    fireEvent.change(screen.getByLabelText("Webhook id"), { target: { value: "wbh_abc123" } });
    const secret = screen.getByLabelText(/Signing secret/);
    expect(secret).toHaveAttribute("type", "password");
    expect(screen.getByText(/CompozyOS never shows it again\./)).toBeInTheDocument();
    fireEvent.change(secret, { target: { value: "whsec_demo" } });

    expect(sentence()).toHaveTextContent("When another app calls the deploy link,");
    expect(status()).toHaveTextContent("Ready");

    togglePreview();
    const request = screen.getByTestId("automation-request-payload");
    expect(request).toHaveTextContent("POST /api/automation/triggers");
    expect(request).toHaveTextContent('"scope": "global"');
    expect(request).toHaveTextContent("[redacted]");
    expect(request).not.toHaveTextContent("whsec_demo");
  });

  it("UT-099 keeps conditions to events and links, names fields plainly, and flags empty values", () => {
    render(<EditorHarness draft={readyDraft()} />);
    expect(screen.queryByTestId("automation-form-only-if")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("automation-start-event"));
    const onlyIf = screen.getByTestId("automation-form-only-if");
    expect(onlyIf).toHaveTextContent("Optional. Every condition must match.");

    fireEvent.click(screen.getByTestId("automation-condition-add"));
    const field = screen.getByTestId("automation-condition-field-0");
    expect(
      within(field).getByRole("option", { name: "Stop reason (data.stop_reason)" })
    ).toBeInTheDocument();
    fireEvent.change(field, { target: { value: "data.stop_reason" } });

    expect(onlyIf).toHaveTextContent("Add a value or remove this condition.");
    expect(status()).toHaveTextContent("Needs a fix");

    fireEvent.change(screen.getByTestId("automation-condition-value-0"), {
      target: { value: "timeout" },
    });
    expect(sentence()).toHaveTextContent("with stop reason timeout");
    expect(status()).toHaveTextContent("Ready");

    fireEvent.click(screen.getByTestId("automation-event-session.created"));
    expect(screen.queryByTestId("automation-condition-field-0")).not.toBeInTheDocument();
  });

  it("UT-100 shows the agent message hint per start and the Loop inputs with event mapping", () => {
    render(<EditorHarness draft={readyDraft()} />);
    expect(screen.getByText("Sent as written.")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("automation-start-event"));
    const details = screen.getByRole("group", { name: "Event details" });
    expect(
      within(details)
        .getAllByRole("button")
        .map(chip => chip.textContent)
    ).toEqual(["session_id", "agent_name", "stop_reason", "workspace"]);
    const prompt = screen.getByTestId("automation-prompt-input") as HTMLTextAreaElement;
    prompt.setSelectionRange(prompt.value.length, prompt.value.length);
    fireEvent.click(within(details).getByRole("button", { name: "Insert stop_reason" }));
    expect(screen.getByTestId("automation-prompt-input")).toHaveValue(
      "Summarize yesterday's sessions.{{ .Data.stop_reason }}"
    );

    fireEvent.click(screen.getByTestId("automation-does-loop"));
    fireEvent.click(screen.getByTestId("loop-target-select"));
    fireEvent.click(screen.getByText("review-and-fix", { selector: "[cmdk-item] *" }));
    expect(screen.getByTestId("loop-target-fields")).toBeInTheDocument();
    expect(screen.getByTestId("loop-input-mapping")).toBeInTheDocument();
    expect(sentence()).toHaveTextContent("start the Loop review-and-fix.");
  });

  it("UT-101 keeps Create a task to schedules and moves a task selection to Ask an agent", () => {
    render(<EditorHarness draft={readyDraft()} />);

    fireEvent.click(screen.getByTestId("automation-does-task"));
    expect(screen.getByTestId("automation-task-title")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("automation-start-event"));

    const task = screen.getByTestId("automation-does-task");
    expect(task).toBeDisabled();
    expect(task).toHaveTextContent("Only scheduled automations can create tasks");
    expect(screen.getByTestId("automation-does-agent")).toHaveAttribute("aria-checked", "true");
    expect(screen.queryByTestId("automation-task-title")).not.toBeInTheDocument();
  });

  it("UT-102 needs a fix when the Loop can't start this way or no agent exists", () => {
    const { unmount } = render(
      <EditorHarness
        draft={{
          ...createAutomationFormDraft("ws_test", { start: "event", loop: "implement-tasks" }),
          name: "release-on-stop",
        }}
        lockedLoop="implement-tasks"
      />
    );

    expect(within(screen.getByTestId("loop-target-fields")).getByRole("alert")).toHaveTextContent(
      "implement-tasks can't be started by an event. Choose a Loop that allows event starts, or start it on a schedule."
    );
    expect(status()).toHaveTextContent("Needs a fix");
    expect(screen.getByTestId("automation-does-agent")).toBeDisabled();
    // m-13: the lock says why — the Loop page chose the target.
    expect(screen.getByTestId("automation-form-does")).toHaveTextContent(
      "Chosen from the Loop page."
    );
    expect(screen.getByTestId("automation-does-agent")).toHaveTextContent(
      "Chosen from the Loop page"
    );
    unmount();

    render(<EditorHarness agents={[]} draft={readyDraft({ agent_name: "" })} />);
    expect(screen.getByTestId("automation-agent-input")).toBeInTheDocument();
    expect(status()).toHaveTextContent("Needs a fix");
    expect(submit()).toBeDisabled();
  });

  it("UT-103 folds Options behind a summary and opens it when editing, retrying or off", () => {
    const { unmount } = render(<EditorHarness draft={readyDraft()} />);

    expect(screen.getByTestId("automation-options-summary")).toHaveTextContent(
      "No retries · up to 12/hour · skip missed · on"
    );
    expect(screen.queryByTestId("automation-enabled-toggle")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("automation-options-toggle"));
    expect(screen.getByTestId("automation-missed-runs")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("automation-retry-backoff"));
    expect(screen.getByTestId("automation-options-summary")).toHaveTextContent(
      "Up to 3 retries · up to 12/hour · skip missed · on"
    );

    fireEvent.click(screen.getByTestId("automation-does-task"));
    expect(screen.getByTestId("automation-retry-backoff")).toBeDisabled();
    expect(screen.getByTestId("automation-form-options")).toHaveTextContent(
      "The task handles its own retries."
    );

    fireEvent.click(screen.getByTestId("automation-start-event"));
    expect(screen.queryByTestId("automation-missed-runs")).not.toBeInTheDocument();
    unmount();

    const { unmount: unmountOff } = render(
      <EditorHarness draft={readyDraft({ enabled: false })} />
    );
    expect(screen.getByTestId("automation-enabled-toggle")).toBeInTheDocument();
    unmountOff();

    render(<EditorHarness draft={readyDraft()} mode="edit" />);
    expect(screen.getByTestId("automation-enabled-toggle")).toBeInTheDocument();
  });

  it("Should open at Options when the editor is asked for that section", () => {
    render(<EditorHarness draft={readyDraft()} section="options" />);

    expect(screen.getByTestId("automation-retry-backoff")).toBeInTheDocument();
  });

  it("UT-104 swaps the body for the preview, keeping values, per start kind", () => {
    render(
      <EditorHarness
        draft={readyDraft({
          conditions: [automationCondition("data.stop_reason", "completed")],
          event: "session.stopped",
        })}
      />
    );

    togglePreview();
    expect(screen.queryByTestId("automation-name-input")).not.toBeInTheDocument();
    expect(screen.getByTestId("automation-preview-toggle")).toHaveTextContent("Back to form");
    const preview = screen.getByTestId("automation-preview");
    expect(preview).toHaveTextContent("Thu Oct 8, 09:00");
    expect(preview).toHaveTextContent("Sun Oct 11, 09:00");
    expect(preview).not.toHaveTextContent("Mon Oct 12, 09:00");
    expect(preview).toHaveTextContent("POST /api/automation/jobs");

    togglePreview();
    expect(screen.getByTestId("automation-name-input")).toHaveValue("morning-digest");

    fireEvent.click(screen.getByTestId("automation-start-event"));
    togglePreview();
    expect(screen.getByTestId("automation-preview")).toHaveTextContent(
      "won't start on this sample"
    );
    expect(screen.getByTestId("automation-preview")).toHaveTextContent(
      "Stop reason must be completed."
    );
    expect(screen.getByTestId("automation-preview")).toHaveTextContent(
      "POST /api/automation/triggers"
    );
  });

  it("UT-105 locks the start and target kind in edit mode and says why", () => {
    render(<EditorHarness draft={readyDraft()} mode="edit" />);

    const header = screen
      .getByTestId("automation-editor-dialog")
      .querySelector('[data-slot="dialog-header"]') as HTMLElement;
    expect(within(header).getByText("Edit automation")).toBeInTheDocument();
    expect(header).toHaveTextContent("Changes apply from the next run.");
    expect(submit()).toHaveTextContent("Save changes");
    expect(screen.getByTestId("automation-destination")).toHaveTextContent(
      "Saves to checkout-api."
    );

    const starts = screen.getByTestId("automation-form-starts");
    expect(starts).toHaveTextContent("Can't change after creating. Make a new automation instead.");
    expect(screen.getByTestId("automation-start-schedule")).toHaveTextContent(
      "Every day at 09:00 UTC"
    );
    expect(screen.getByTestId("automation-start-event")).toBeDisabled();
    expect(screen.getByTestId("automation-start-event")).toHaveTextContent("Locked");
    expect(screen.getByTestId("automation-start-webhook")).toBeDisabled();

    expect(screen.getByTestId("automation-form-does")).toHaveTextContent(
      "The agent and the kind of target stay. The message can change."
    );
    expect(screen.getByTestId("automation-does-agent")).toHaveTextContent(AGENT);
    expect(screen.getByTestId("automation-does-loop")).toBeDisabled();
    expect(screen.getByTestId("automation-does-task")).toBeDisabled();
    expect(screen.getByTestId("automation-agent-input")).toBeDisabled();
    expect(screen.getByTestId("automation-prompt-input")).toBeEnabled();
  });

  it("Should preserve the agent catalog loading state through the agent selector", () => {
    render(<EditorHarness agents={[]} agentsLoading />);

    const selector = screen.getByTestId("automation-agent-input");
    expect(selector).toBeDisabled();
    expect(selector).toHaveAttribute("aria-busy", "true");
  });

  it("m-14 keeps conditions as rows: one field twice needs a fix, blank rows never collide", () => {
    render(<EditorHarness draft={readyDraft({ start: "event" })} />);

    fireEvent.click(screen.getByTestId("automation-condition-add"));
    fireEvent.click(screen.getByTestId("automation-condition-add"));
    fireEvent.change(screen.getByTestId("automation-condition-field-0"), {
      target: { value: "data.stop_reason" },
    });
    fireEvent.change(screen.getByTestId("automation-condition-field-1"), {
      target: { value: "data.stop_reason" },
    });
    fireEvent.change(screen.getByTestId("automation-condition-value-0"), {
      target: { value: "error" },
    });
    fireEvent.change(screen.getByTestId("automation-condition-value-1"), {
      target: { value: "timeout" },
    });

    expect(screen.getByTestId("automation-form-only-if")).toHaveTextContent(
      "This field already has a condition. Remove one of them."
    );
    expect(status()).toHaveTextContent("Needs a fix");

    fireEvent.click(screen.getByTestId("automation-condition-remove-1"));
    expect(screen.queryByTestId("automation-condition-field-1")).not.toBeInTheDocument();
    expect(status()).toHaveTextContent("Ready");
  });

  it("m-15 disables the days and the clock when the schedule isn't a days-and-time shape", () => {
    render(<EditorHarness draft={readyDraft()} />);

    fireEvent.click(screen.getByRole("button", { name: "Every hour" }));

    expect(screen.getByRole("button", { name: "Monday" })).toBeDisabled();
    expect(screen.getByLabelText("Time")).toBeDisabled();
    expect(readout()).toHaveTextContent("0 * * * *");

    fireEvent.click(screen.getByRole("button", { name: "Weekdays 9am" }));
    expect(screen.getByRole("button", { name: "Monday" })).toBeEnabled();
  });

  it("m-16 saves an edit that keeps a one-shot time already past, but not a new past time", () => {
    render(
      <EditorHarness
        draft={readyDraft({ schedule: { mode: "at", time: "2026-10-01T09:00:00Z" } })}
        mode="edit"
      />
    );

    expect(status()).toHaveTextContent("Ready");
    expect(submit()).toBeEnabled();

    fireEvent.change(screen.getByLabelText("Date and time"), {
      target: { value: "2026-10-02T09:00" },
    });
    expect(status()).toHaveTextContent("Needs a fix");
  });

  it("m-19 never submits while a save is pending", () => {
    const onSubmit = vi.fn();
    render(<EditorHarness draft={readyDraft()} isPending onSubmit={onSubmit} />);

    expect(submit()).toBeDisabled();
    expect(submit()).toHaveTextContent("Saving…");
    fireEvent.submit(screen.getByTestId("automation-form"));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("m-19 keeps a save error visible across the preview swap", () => {
    render(<EditorHarness draft={readyDraft()} mode="edit" submitError="automation: changed" />);

    togglePreview();
    expect(screen.getByTestId("automation-form-error")).toHaveTextContent("automation: changed");
    togglePreview();
    expect(screen.getByTestId("automation-form-error")).toHaveTextContent("automation: changed");
  });

  it("m-19 carries missed-run settings across Repeats, Once and back", () => {
    render(
      <EditorHarness
        draft={readyDraft({
          schedule: {
            mode: "cron",
            expr: "0 9 * * *",
            catch_up_policy: "replay",
            misfire_grace_seconds: 30,
          },
        })}
      />
    );

    fireEvent.click(screen.getByTestId("automation-schedule-mode-at"));
    fireEvent.click(screen.getByTestId("automation-schedule-mode-every"));
    fireEvent.click(screen.getByTestId("automation-schedule-mode-cron"));

    togglePreview();
    const request = screen.getByTestId("automation-request-payload");
    expect(request).toHaveTextContent('"catch_up_policy": "replay"');
    expect(request).toHaveTextContent('"misfire_grace_seconds": 30');
  });

  it("m-19 refreshes the relative readout at the next minute boundary", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-08T08:58:30Z"));
    render(<EditorHarness draft={readyDraft({ schedule: { mode: "cron", expr: "0 9 * * *" } })} />);
    expect(readout()).toHaveTextContent("next in 2 min");

    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(readout()).toHaveTextContent("next in 1 min");
  });

  it("Should keep the dialog open through a nested picker interaction", async () => {
    const user = userEvent.setup({ advanceTimers: () => undefined });
    render(<EditorHarness />);

    await user.click(screen.getByTestId("automation-start-event"));
    await user.click(screen.getByTestId("automation-event-ext"));

    expect(screen.getByTestId("automation-editor-dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("Extension")).toBeInTheDocument();
    expect(sentence()).toHaveTextContent("When an extension event fires");
  });

  it("Should not render the dialog content when editor is null", () => {
    render(<AutomationEditorDialog editor={null} />);

    expect(screen.queryByTestId("automation-editor-dialog")).not.toBeInTheDocument();
  });
});
