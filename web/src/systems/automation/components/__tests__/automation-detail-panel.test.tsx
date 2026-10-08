// Suite: Automation detail panel
// Invariant: One detail grammar serves schedules, events and links — the sentence with its
// non-optimistic switch, "How it works" (Starts / Only if / Does), the shared run list, the
// one-card rail, the lockbar, Inspect, delete by typing the name, and states with a way back —
// and it renders only what the daemon can back (Run now for jobs, secret presence, recorded ids).
// Boundary IN: AutomationDetailPanel and the pure detail/run/inspect models it renders.
// Boundary OUT: data access, toasts and navigation (use-automation-detail-page.test.tsx);
// dispatch and persistence (daemon/store suites).
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AnchorHTMLAttributes } from "react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";

interface MockLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  params?: { id?: string; name?: string; runId?: string };
  search?: { workspace?: string };
  to?: string;
}

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, params, search, to, ...props }: MockLinkProps) => {
    const path = String(to)
      .replace("$runId", params?.runId ?? "")
      .replace("$name", params?.name ?? "")
      .replace("$id", params?.id ?? "");
    const href = search?.workspace ? `${path}?workspace=${search.workspace}` : path;
    return (
      <a href={href} {...props}>
        {children}
      </a>
    );
  },
  useNavigate: () => vi.fn(),
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import {
  AutomationDetailPanel,
  type AutomationDetailPanelProps,
} from "../automation-detail/automation-detail-panel";
import type { AutomationEntity } from "../../lib/automation-detail";
import { toAutomationView } from "../../lib/automation-view";
import {
  dependencyReviewJob,
  deployWebhookTrigger,
  makeDetailJob,
  makeDetailRun,
  makeDetailTrigger,
  morningDigestJob,
  morningDigestRuns,
  releaseChecklistJob,
  rerunDeliveryRuns,
  rerunDeliveryTrigger,
  summarizeFailuresTrigger,
} from "../../mocks/detail-fixtures";

const NOW = new Date("2026-10-07T19:00:00Z");
const ctx = { workspaceName: (id: string) => (id === "ws_checkout_api" ? "checkout-api" : id) };

function renderPanel(
  entity: AutomationEntity | undefined,
  overrides: Partial<AutomationDetailPanelProps> = {}
) {
  const handlers = {
    onBack: vi.fn(),
    onDelete: vi.fn(),
    onEdit: vi.fn(),
    onRetryRuns: vi.fn(),
    onRunNow: vi.fn(),
    onSetUpRetries: vi.fn(),
    onToggleEnabled: vi.fn(),
  };
  const build = (
    next: AutomationEntity | undefined,
    extra: Partial<AutomationDetailPanelProps>
  ) => (
    <AutomationDetailPanel
      entity={next}
      lastRanAt={null}
      loopMissing={false}
      loopWorkspaceName="checkout-api"
      runs={[]}
      runsError={null}
      runsLoading={false}
      sentenceContext={ctx}
      state={{
        isDeleting: false,
        isRunNowDisabled: false,
        isRunNowPending: false,
        isTogglePending: false,
      }}
      status={next ? "ready" : "missing"}
      statusMessage="This automation is no longer available."
      view={next ? toAutomationView(next, ctx) : undefined}
      {...handlers}
      {...extra}
    />
  );
  const view = renderWithTopbar(build(entity, overrides));
  return {
    ...handlers,
    rerenderPanel: (next: AutomationEntity, extra: Partial<AutomationDetailPanelProps> = {}) =>
      view.rerender(build(next, extra)),
  };
}

describe("AutomationDetailPanel", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  describe("head", () => {
    it("Should lead a schedule with its sentence, the On switch and a dated subhead (UT-060)", () => {
      renderPanel(morningDigestJob);

      expect(screen.getByTestId("topbar-title-text")).toHaveTextContent("morning-digest");
      expect(screen.getByTestId("automation-detail-sentence")).toHaveTextContent(
        "Every weekday at 09:00 UTC, ask summarizer."
      );
      expect(screen.getByTestId("automation-enable-label")).toHaveTextContent("On");
      expect(screen.getByRole("switch", { name: "Turn morning-digest on or off" })).toBeChecked();
      const subhead = screen.getByTestId("automation-detail-subhead");
      expect(subhead).toHaveTextContent("On a schedule");
      expect(subhead).toHaveTextContent("Project checkout-api");
      expect(subhead).toHaveTextContent("Next run in 14h");
      expect(subhead).toHaveTextContent("Updated");
    });

    it("Should date an event by when it last ran instead of a next run (UT-060)", () => {
      renderPanel(rerunDeliveryTrigger, { lastRanAt: "2026-10-07T17:00:00Z" });

      const subhead = screen.getByTestId("automation-detail-subhead");
      expect(subhead).toHaveTextContent("On an event");
      expect(subhead).toHaveTextContent("Last ran 2h ago");
      expect(subhead).not.toHaveTextContent("Next run");
    });

    it("Should announce the transition and keep the confirmed state while the switch saves (UT-061)", () => {
      const { onToggleEnabled } = renderPanel(morningDigestJob, {
        state: {
          isDeleting: false,
          isRunNowDisabled: false,
          isRunNowPending: false,
          isTogglePending: true,
        },
      });

      const track = screen.getByTestId("automation-enable-switch");
      expect(screen.getByTestId("automation-enable-label")).toHaveTextContent("Turning off…");
      expect(track).toHaveAttribute("aria-checked", "true");
      fireEvent.click(track);
      expect(onToggleEnabled).not.toHaveBeenCalled();
    });

    it.each([
      {
        entity: dependencyReviewJob,
        pause: "Off. It won't run on its schedule until you turn it on.",
      },
      {
        entity: makeDetailTrigger({ enabled: false }),
        pause: "Off. Matching events won't start it until you turn it on.",
      },
    ])("Should explain what Off means for $entity.name (UT-061)", ({ entity, pause }) => {
      const { onToggleEnabled } = renderPanel(entity);

      expect(screen.getByTestId("automation-enable-label")).toHaveTextContent("Off");
      expect(screen.getByTestId("automation-pause-line")).toHaveTextContent(pause);
      fireEvent.click(screen.getByTestId("automation-enable-switch"));
      expect(onToggleEnabled).toHaveBeenCalledWith(true);
    });

    it("Should drop the next run everywhere while a schedule is Off (UT-061)", () => {
      renderPanel(dependencyReviewJob);

      expect(screen.getByTestId("automation-detail-subhead")).toHaveTextContent("No next run");
      expect(screen.queryByTestId("automation-next-runs")).not.toBeInTheDocument();
    });
  });

  describe("how it works", () => {
    it("Should read a cron schedule in words with its zone, expression and next 3 runs (UT-063)", () => {
      renderPanel(morningDigestJob);

      const starts = screen.getByTestId("automation-rule-starts");
      expect(starts).toHaveTextContent("Every weekday at 09:00");
      expect(starts).toHaveTextContent("Monday to Friday · times in UTC · 0 9 * * 1-5");
      const nextRuns = within(screen.getByTestId("automation-next-runs")).getAllByRole("listitem");
      expect(nextRuns).toHaveLength(3);
      expect(nextRuns[0]).toHaveTextContent("in 14h");
      expect(nextRuns[0]).toHaveTextContent("Thu Oct 8, 09:00");
      expect(screen.queryByTestId("automation-rule-only-if")).not.toBeInTheDocument();
    });

    it("Should count an every schedule from turn-on and mark a past once as already ran (UT-064)", () => {
      const { rerenderPanel } = renderPanel(releaseChecklistJob);

      let starts = screen.getByTestId("automation-rule-starts");
      expect(starts).toHaveTextContent("Every 30 minutes");
      expect(starts).toHaveTextContent("Starts counting from when it was turned on");
      expect(screen.queryByTestId("automation-next-runs")).not.toBeInTheDocument();

      rerenderPanel(
        makeDetailJob({ schedule: { mode: "at", time: "2026-10-01T09:00:00Z" }, scheduler: null })
      );
      starts = screen.getByTestId("automation-rule-starts");
      expect(starts).toHaveTextContent("Once, on Thu Oct 1 at 09:00 UTC");
      expect(starts).toHaveTextContent("Already ran");
      expect(screen.queryByTestId("automation-next-runs")).not.toBeInTheDocument();
    });

    it("Should render one Only if clause per condition joined with and (UT-065)", () => {
      renderPanel(deployWebhookTrigger);

      const onlyIf = screen.getByTestId("automation-rule-only-if");
      expect(onlyIf).toHaveTextContent("Action is");
      expect(onlyIf).toHaveTextContent("and branch is");
      expect(onlyIf).toHaveTextContent("data.action");
      expect(onlyIf).toHaveTextContent("data.branch");
    });

    it("Should show a schedule's message word for word and an event's message as a template (UT-066)", () => {
      const { rerenderPanel } = renderPanel(morningDigestJob);

      const does = screen.getByTestId("automation-rule-does");
      expect(does).toHaveTextContent("Ask summarizer");
      expect(does).toHaveTextContent("word for word");
      expect(screen.getByTestId("automation-prompt-preview")).toHaveClass("line-clamp-3");
      fireEvent.click(screen.getByRole("button", { name: "Show full prompt" }));
      expect(screen.getByTestId("automation-prompt-preview")).not.toHaveClass("line-clamp-3");

      rerenderPanel(summarizeFailuresTrigger);
      expect(screen.getByTestId("automation-rule-does")).toHaveTextContent(
        "The message is filled in from each event."
      );
      expect(screen.getByText("{{ .Data.session_id }}")).toHaveClass("text-info");
    });

    it("Should list a Loop's inputs as from-the-event and always rows with a linked Loop (UT-067)", () => {
      renderPanel(rerunDeliveryTrigger);

      expect(screen.getByTestId("automation-loop-link")).toHaveAttribute(
        "href",
        "/loops/software-delivery?workspace=ws_checkout_api"
      );
      const inputs = screen.getByTestId("automation-loop-inputs");
      expect(inputs).toHaveTextContent("slug←data.session_namefrom the event");
      expect(inputs).toHaveTextContent("target_branch=mainalways");
    });

    it("Should name a task's title and owner, and give a link its endpoint and signed example (UT-068)", async () => {
      const user = userEvent.setup();
      const { rerenderPanel } = renderPanel(dependencyReviewJob);

      const task = screen.getByTestId("automation-task-details");
      expect(task).toHaveTextContent("TitleReview dependency updates");
      expect(task).toHaveTextContent("ForAgent pool reviewers");

      rerenderPanel(deployWebhookTrigger);
      const endpoint = screen.getByTestId("automation-webhook-endpoint");
      expect(endpoint).toHaveTextContent(
        "POST/api/webhooks/workspaces/ws_checkout_api/deploy--wbh_abc123"
      );
      expect(within(endpoint).getByRole("button", { name: "Copy webhook path" })).toBeVisible();
      await user.click(screen.getByTestId("automation-webhook-example-toggle"));
      expect(endpoint).toHaveTextContent("X-Compozy-Webhook-Signature");
    });

    it("Should keep a deleted Loop's name unlinked and say it is gone (UT-069)", () => {
      renderPanel(rerunDeliveryTrigger, { loopMissing: true });

      expect(screen.queryByTestId("automation-loop-link")).not.toBeInTheDocument();
      expect(screen.getByTestId("automation-loop-name")).toHaveTextContent("software-delivery");
      expect(screen.getByTestId("automation-rule-does")).toHaveTextContent(
        "This Loop no longer exists."
      );
    });
  });

  describe("runs", () => {
    it("Should read each run as glyph + shared word, with one drawer open at a time (UT-070)", () => {
      renderPanel(morningDigestJob, { runs: morningDigestRuns });

      const completed = screen.getByTestId("automation-run-run_001");
      expect(completed).toHaveTextContent("Completed");
      expect(completed).toHaveTextContent("Sessionsess_9f2a1c");
      expect(completed).toHaveTextContent("42s");
      expect(completed.querySelector("[data-state='done']")).not.toBeNull();
      expect(screen.getByTestId("automation-run-run_missed")).toHaveTextContent(
        "MissedCompozyOS was off at the start time"
      );

      fireEvent.click(completed);
      expect(screen.getByTestId("automation-run-drawer-run_001")).toBeVisible();
      fireEvent.click(screen.getByTestId("automation-run-run_missed"));
      expect(screen.getByTestId("automation-run-drawer-run_001")).not.toBeVisible();
      expect(screen.getByTestId("automation-run-drawer-run_missed")).toHaveTextContent(
        "CompozyOS was off at the start time."
      );
    });

    it("Should keep a failure's cause muted on the row and red only in its drawer (UT-071)", () => {
      const { onSetUpRetries } = renderPanel(morningDigestJob, { runs: morningDigestRuns });

      const row = screen.getByTestId("automation-run-run_failed");
      expect(within(row).getByText("Agent summarizer was not available")).not.toHaveClass(
        "text-danger"
      );
      fireEvent.click(row);
      const drawer = screen.getByTestId("automation-run-drawer-run_failed");
      expect(within(drawer).getByText("Agent summarizer was not available")).toHaveClass(
        "text-danger"
      );
      expect(drawer).toHaveTextContent("Attempt 2.");
      fireEvent.click(within(drawer).getByRole("button", { name: "Set up retries" }));
      expect(onSetUpRetries).toHaveBeenCalledOnce();
      expect(screen.getByTestId("automation-run-run_manual")).toHaveTextContent(
        "Run now · Sessionsess_4b80d2"
      );
    });

    it("Should open what each run produced: session, loop run or task (UT-072)", () => {
      const { rerenderPanel } = renderPanel(rerunDeliveryTrigger, { runs: rerunDeliveryRuns });

      fireEvent.click(screen.getByTestId("automation-run-run_handed_off"));
      expect(screen.getByTestId("automation-run-run_handed_off")).toHaveTextContent("Handed off");
      expect(screen.getByTestId("automation-run-open-run_handed_off")).toHaveAttribute(
        "href",
        "/loop-runs/looprun_8f3a2bce41d07a55?workspace=ws_checkout_api"
      );

      rerenderPanel(dependencyReviewJob, {
        runs: [
          makeDetailRun({
            id: "run_task",
            status: "delegated",
            session_id: undefined,
            task_id: "task_42",
          }),
        ],
      });
      fireEvent.click(screen.getByTestId("automation-run-run_task"));
      expect(screen.getByTestId("automation-run-open-run_task")).toHaveAttribute(
        "href",
        "/tasks/task_42"
      );

      rerenderPanel(morningDigestJob, { runs: morningDigestRuns });
      fireEvent.click(screen.getByTestId("automation-run-run_001"));
      expect(screen.getByTestId("automation-run-open-run_001")).toHaveAttribute(
        "href",
        "/session/sess_9f2a1c"
      );
    });

    it("Should say no runs yet and, for a schedule that is on, when the next one is (UT-073)", () => {
      const { rerenderPanel } = renderPanel(morningDigestJob);

      expect(screen.getByTestId("automation-run-list-empty")).toHaveTextContent("No runs yet");
      expect(screen.getByTestId("automation-run-list-empty")).toHaveTextContent("Next run in 14h");

      rerenderPanel(rerunDeliveryTrigger);
      expect(screen.getByTestId("automation-run-list-empty")).not.toHaveTextContent("Next run");
    });

    it("Should offer Try again when the runs cannot be read", () => {
      const { onRetryRuns } = renderPanel(morningDigestJob, {
        runsError: new Error("runs unavailable"),
      });

      expect(screen.getByTestId("automation-run-list-error")).toHaveTextContent("runs unavailable");
      fireEvent.click(screen.getByRole("button", { name: "Try again" }));
      expect(onRetryRuns).toHaveBeenCalledOnce();
    });
  });

  describe("actions", () => {
    it("Should offer Run now for schedules only, Starting… while pending, disabled when unavailable (UT-074)", () => {
      const { onRunNow, rerenderPanel } = renderPanel(dependencyReviewJob);

      fireEvent.click(screen.getByRole("button", { name: "Run now" }));
      expect(onRunNow).toHaveBeenCalledOnce();

      rerenderPanel(morningDigestJob, {
        state: {
          isDeleting: false,
          isRunNowDisabled: false,
          isRunNowPending: true,
          isTogglePending: false,
        },
      });
      expect(screen.getByTestId("automation-run-now-btn")).toHaveTextContent("Starting…");
      expect(screen.getByTestId("automation-run-now-btn")).toBeDisabled();

      rerenderPanel(morningDigestJob, {
        state: {
          isDeleting: false,
          isRunNowDisabled: true,
          isRunNowPending: false,
          isTogglePending: false,
        },
      });
      expect(screen.getByTestId("automation-run-now-btn")).toBeDisabled();

      rerenderPanel(summarizeFailuresTrigger);
      expect(screen.queryByTestId("automation-run-now-btn")).not.toBeInTheDocument();
      expect(screen.queryByText("Run now")).not.toBeInTheDocument();
    });

    it("Should give config automations the lockbar and On/Off only, keeping Run now (UT-077)", () => {
      renderPanel(releaseChecklistJob);

      expect(screen.getByTestId("automation-lockbar")).toHaveTextContent(
        'This automation is defined in configuration files. You can only turn it on or off here. Lives in config.toml — [[automation.jobs]] name = "release-checklist"'
      );
      expect(screen.queryByTestId("automation-edit-btn")).not.toBeInTheDocument();
      expect(screen.getByTestId("automation-run-now-btn")).toBeEnabled();
      expect(screen.getByTestId("automation-enable-switch")).toBeEnabled();
      fireEvent.click(screen.getByTestId("automation-detail-overflow"));
      expect(screen.getByTestId("automation-edit-in-config")).toHaveAttribute(
        "aria-disabled",
        "true"
      );
      expect(screen.queryByTestId("automation-delete-btn")).not.toBeInTheDocument();
    });

    it("Should name the package for package automations and cite the trigger table for config triggers", () => {
      const { rerenderPanel } = renderPanel(makeDetailTrigger({ source: "package" }));

      expect(screen.getByTestId("automation-lockbar")).toHaveTextContent(
        "This automation is provided by an installed package."
      );
      rerenderPanel(makeDetailTrigger({ source: "config" }));
      expect(screen.getByTestId("automation-lockbar")).toHaveTextContent(
        '[[automation.triggers]] name = "rerun-delivery"'
      );
    });

    it("Should report a copy failure when the browser has no Clipboard API", () => {
      const descriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
      Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
      try {
        renderPanel(morningDigestJob);
        fireEvent.click(screen.getByTestId("automation-detail-overflow"));
        fireEvent.click(screen.getByTestId("automation-copy-id-btn"));
        expect(toast.error).toHaveBeenCalledWith("Could not copy the automation id.");
      } finally {
        if (descriptor) Object.defineProperty(navigator, "clipboard", descriptor);
        else Reflect.deleteProperty(navigator, "clipboard");
      }
    });

    it("Should keep Delete disabled until the exact name is typed (UT-079)", async () => {
      vi.useRealTimers();
      const user = userEvent.setup();
      const { onDelete, onEdit } = renderPanel(morningDigestJob);

      await user.click(screen.getByTestId("automation-edit-btn"));
      expect(onEdit).toHaveBeenCalledOnce();
      fireEvent.click(screen.getByTestId("automation-detail-overflow"));
      fireEvent.click(screen.getByTestId("automation-delete-btn"));
      const dialog = screen.getByRole("dialog", { name: "Delete automation?" });
      expect(dialog).toHaveTextContent(
        "This permanently deletes morning-digest. Its schedule will stop asking summarizer. Past runs stay in the log."
      );
      const confirm = screen.getByTestId("confirm-delete-automation-btn");
      await user.type(screen.getByLabelText("Type to confirm"), "morning-diges");
      expect(confirm).toBeDisabled();
      await user.type(screen.getByLabelText("Type to confirm"), "t");
      await user.click(confirm);
      expect(onDelete).toHaveBeenCalledOnce();
    });

    it("Should keep the dialog open with the daemon's error when delete fails (UT-079)", async () => {
      vi.useRealTimers();
      const user = userEvent.setup();
      renderPanel(rerunDeliveryTrigger, {
        onDelete: () => Promise.reject(new Error("Internal server error")),
      });

      fireEvent.click(screen.getByTestId("automation-detail-overflow"));
      fireEvent.click(screen.getByTestId("automation-delete-btn"));
      expect(screen.getByRole("dialog", { name: "Delete automation?" })).toHaveTextContent(
        "Matching events will stop starting it. Past runs stay in the log."
      );
      await user.type(screen.getByLabelText("Type to confirm"), "rerun-delivery");
      await user.click(screen.getByTestId("confirm-delete-automation-btn"));
      expect(await screen.findByTestId("automation-delete-error")).toHaveTextContent(
        "Internal server error"
      );
      expect(screen.getByRole("dialog", { name: "Delete automation?" })).toBeInTheDocument();
    });
  });

  describe("rail and inspect", () => {
    it("Should give a schedule one rail card with Details, Schedule, Reliability, Identity and the CLI hint (UT-076)", () => {
      renderPanel(morningDigestJob, { lastRanAt: "2026-10-07T09:00:00Z" });

      const rail = screen.getByTestId("automation-rail");
      expect(within(rail).getByTestId("automation-rail-details")).toHaveTextContent(
        "StartsOn a scheduleDoesAsk an agentAgentsummarizerLocationProject checkout-apiSourceYou created this"
      );
      const schedule = within(rail).getByTestId("automation-rail-schedule");
      expect(schedule).toHaveTextContent("RepeatsEvery weekday at 09:00");
      expect(schedule).toHaveTextContent("Time zoneUTC");
      expect(schedule).toHaveTextContent("Missed runsSkip missed");
      expect(within(rail).getByTestId("automation-rail-reliability")).toHaveTextContent(
        "RetriesNo retriesRun limitUp to 12 runs per hour"
      );
      expect(within(rail).getByTestId("automation-rail-identity")).toHaveTextContent(
        "morning-digest"
      );
      expect(screen.getByTestId("automation-rail-cli")).toHaveTextContent(
        "compozy automation jobs get morning-digest"
      );
      expect(within(rail).queryByTestId("automation-rail-public-link")).not.toBeInTheDocument();
    });

    it("Should replace Schedule with Public link and Security for a link, never the secret (UT-076)", () => {
      const { rerenderPanel } = renderPanel(deployWebhookTrigger);

      expect(screen.queryByTestId("automation-rail-schedule")).not.toBeInTheDocument();
      expect(screen.getByTestId("automation-rail-public-link")).toHaveTextContent("StatusLive");
      expect(screen.getByTestId("automation-rail-security")).toHaveTextContent("Signing secretSet");

      rerenderPanel(
        makeDetailTrigger({
          ...deployWebhookTrigger,
          webhook_secret_present: false,
          ingress: { ...deployWebhookTrigger.ingress!, reachability: "broken" },
        })
      );
      expect(screen.getByText("Broken")).toHaveClass("text-danger");
      expect(screen.getByTestId("automation-rail-security")).toHaveTextContent(
        "Signing secretNot set"
      );
    });

    it("Should show a job's machine truth and scheduler state in Inspect (UT-078)", async () => {
      vi.useRealTimers();
      const user = userEvent.setup();
      renderPanel(morningDigestJob);

      await user.click(screen.getByTestId("automation-inspect-btn"));
      const sheet = await screen.findByTestId("automation-inspect-sheet");
      expect(sheet).toHaveTextContent("This is a job in the daemon's terms.");
      expect(screen.getByTestId("automation-inspect-tile-kind")).toHaveTextContent("job · cron");
      expect(screen.getByTestId("automation-inspect-tile-scheduler")).toHaveTextContent(
        "Registered"
      );
      expect(screen.getByTestId("automation-inspect-tile-missed")).toHaveTextContent("1");
      expect(screen.getByTestId("automation-inspect-tile-last-fire")).toHaveTextContent(
        "fire_morning_digest_118"
      );
      expect(screen.getByTestId("automation-inspect-diagnostics")).toHaveTextContent(
        "Expression 0 9 * * 1-5 · catch-up skip_missed · grace 30s · fire limit 12 / 1h."
      );
      await user.click(screen.getByRole("tab", { name: "Scheduler state" }));
      await waitFor(() =>
        expect(screen.getByTestId("automation-inspect-raw")).toHaveTextContent('"registered": true')
      );
    });

    it("Should show a trigger's sample event in Inspect without its secret (UT-078)", async () => {
      vi.useRealTimers();
      const user = userEvent.setup();
      renderPanel(deployWebhookTrigger);

      await user.click(screen.getByTestId("automation-inspect-btn"));
      const sheet = await screen.findByTestId("automation-inspect-sheet");
      expect(sheet).toHaveTextContent("a trigger in the daemon's terms");
      expect(screen.getByTestId("automation-inspect-tile-kind")).toHaveTextContent(
        "trigger · webhook"
      );
      expect(screen.getByTestId("automation-inspect-tile-secret")).toHaveTextContent("present");
      await user.click(screen.getByRole("tab", { name: "Sample event" }));
      await waitFor(() =>
        expect(screen.getByTestId("automation-inspect-raw")).toHaveTextContent(
          '"endpoint": "deploy--wbh_abc123"'
        )
      );
    });

    it("Should close overlays owned by one automation when the route changes automation", async () => {
      vi.useRealTimers();
      const user = userEvent.setup();
      const { rerenderPanel } = renderPanel(morningDigestJob);

      await user.click(screen.getByTestId("automation-inspect-btn"));
      expect(await screen.findByTestId("automation-inspect-sheet")).toBeInTheDocument();
      rerenderPanel(rerunDeliveryTrigger);
      await waitFor(() =>
        expect(screen.queryByTestId("automation-inspect-sheet")).not.toBeInTheDocument()
      );
    });
  });

  describe("states", () => {
    it.each([
      {
        status: "missing" as const,
        message: "This automation is no longer available.",
        testId: "automation-detail-empty",
        title: "Automation unavailable",
      },
      {
        status: "elsewhere" as const,
        message: "This automation belongs to another project. Switch to it to open this page.",
        testId: "automation-detail-elsewhere",
        title: "Unable to load details",
      },
    ])(
      "Should offer a way back from the $status state (UT-080)",
      ({ status, message, testId, title }) => {
        const { onBack } = renderPanel(undefined, { status, statusMessage: message });

        expect(screen.getByTestId(testId)).toHaveTextContent(title);
        expect(screen.getByTestId(testId)).toHaveTextContent(message);
        fireEvent.click(screen.getByRole("button", { name: "Back to Automations" }));
        expect(onBack).toHaveBeenCalledOnce();
      }
    );

    it("Should hold the page geometry while loading instead of guessing a sentence (UT-080)", () => {
      renderPanel(undefined, { status: "loading" });

      expect(screen.getByTestId("automation-detail-loading")).toBeInTheDocument();
      expect(screen.queryByTestId("automation-detail-sentence")).not.toBeInTheDocument();
    });
  });
});
