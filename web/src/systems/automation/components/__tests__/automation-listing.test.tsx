// Suite: Automations listing rows, cards and states
// Invariant: a row reads as a sentence with its time and last-run truth, shows only exception
//   badges and supported actions, never flips its switch before the daemon answers; the shell
//   tells first run, filtered empty, loading and load error apart.
// Boundary IN: AutomationView (board story fixtures) and row controls.
// Boundary OUT: data loading (use-automations-page suite) and routing.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { automationStoryJobs, automationStoryTriggers } from "../../mocks/story-fixtures";
import { toAutomationView, type AutomationView } from "../../lib/automation-view";
import type { AutomationJob, AutomationTrigger } from "../../types";
import { AutomationCard } from "../automation-card";
import { AutomationCatalogShell } from "../automation-catalog-shell";
import { AutomationRow, type AutomationItemControls } from "../automation-row";
import { scopedListingScopeFixture, aggregateListingScopeFixture } from "@/systems/profiles/mocks";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    params: _params,
    search,
    to,
    ...props
  }: {
    children?: ReactNode;
    params?: Record<string, string>;
    search?: Record<string, string>;
    to?: string;
  }) => (
    <a href={search ? `${to}?${new URLSearchParams(search).toString()}` : to} {...props}>
      {children}
    </a>
  ),
}));

const ctx = { workspaceName: () => "checkout-api" };
const views = new Map(
  [...automationStoryJobs, ...automationStoryTriggers].map(entity => [
    entity.id,
    toAutomationView(entity, ctx),
  ])
);
const view = (id: string) => views.get(id) as AutomationView;
const withEntity = (entity: AutomationJob | AutomationTrigger) => toAutomationView(entity, ctx);

function controls(overrides: Partial<AutomationItemControls> = {}): AutomationItemControls {
  return {
    detailSearch: {},
    unavailable: false,
    isRunPending: () => false,
    isTogglePending: () => false,
    onToggleEnabled: vi.fn(),
    onRunNow: vi.fn(),
    onEdit: vi.fn(),
    onDelete: vi.fn(),
    onCopyLink: vi.fn(),
    ...overrides,
  };
}

function renderRow(item: AutomationView, rowControls = controls()) {
  render(<AutomationRow controls={rowControls} view={item} />);
  return rowControls;
}

describe("AutomationRow", () => {
  it("Should read a schedule as glyph, sentence, meta and next-run stat", () => {
    renderRow(view("morning-digest"));
    const row = screen.getByTestId("automation-row-job-morning-digest");
    expect(row.querySelector('[data-start="schedule"]')).not.toBeNull();
    expect(screen.getByTestId("automation-sentence-morning-digest")).toHaveTextContent(
      "Every weekday at 09:00 UTC, ask summarizer to summarize yesterday's sessions."
    );
    expect(row).toHaveTextContent("Project checkout-api");
    expect(screen.getByTestId("automation-last-run-morning-digest")).toHaveTextContent(
      /Last run completed 1[45]h ago/
    );
    expect(screen.getByTestId("automation-stat-morning-digest")).toHaveTextContent(
      /In 1[34]hnext run/
    );
    expect(within(row).queryByTestId("automation-off-badge")).toBeNull();
    expect(within(row).queryByTestId("automation-source-badge")).toBeNull();
  });

  it("Should carry the listing search into the detail link so Back restores it (UT-062)", () => {
    const detailSearch = { start: "schedule", q: "digest", view: "cards" } as const;
    render(<AutomationRow controls={controls({ detailSearch })} view={view("morning-digest")} />);
    render(<AutomationCard controls={controls({ detailSearch })} view={view("morning-digest")} />);

    for (const link of screen.getAllByRole("link", { name: "Open morning-digest" })) {
      expect(link).toHaveAttribute(
        "href",
        "/automations/jobs/$jobId?start=schedule&q=digest&view=cards"
      );
    }
  });

  it("Should show event and webhook glyphs with the last-ran stat", () => {
    renderRow(view("summarize-failures"));
    renderRow(view("deploy-webhook"));
    expect(
      screen
        .getByTestId("automation-row-trigger-summarize-failures")
        .querySelector('[data-start="event"]')
    ).not.toBeNull();
    expect(screen.getByTestId("automation-stat-summarize-failures")).toHaveTextContent(
      "2h agolast ran"
    );
    const webhook = screen.getByTestId("automation-row-trigger-deploy-webhook");
    expect(webhook.querySelector('[data-start="webhook"]')).not.toBeNull();
    expect(webhook).toHaveTextContent("Public link live");
  });

  it("Should mark a failed last run in danger and keep skips neutral", () => {
    renderRow(view("nightly-delivery"));
    renderRow(view("release-checklist"));
    const failed = screen.getByTestId("automation-last-run-nightly-delivery");
    expect(failed).toHaveTextContent(/Last run failed 7h ago/);
    expect(failed).toHaveAttribute("data-tone", "danger");
    const skipped = screen.getByTestId("automation-last-run-release-checklist");
    expect(skipped).toHaveTextContent("Last run skipped — the one before was still going");
    expect(skipped).toHaveAttribute("data-tone", "neutral");
  });

  it("Should render nothing for a never-run automation and a faint dash for its stat", () => {
    const { last_run: _lastRun, ...neverRan } = automationStoryTriggers[0];
    renderRow(withEntity(neverRan));
    expect(screen.queryByTestId("automation-last-run-summarize-failures")).toBeNull();
    expect(screen.getByTestId("automation-stat-summarize-failures")).toHaveTextContent("—");
  });

  it("Should show Running now, Off and the config lock as exceptions only", () => {
    renderRow(
      withEntity({
        ...automationStoryJobs[0],
        id: "busy",
        last_run: { id: "run_1", status: "running", started_at: new Date().toISOString() },
      })
    );
    expect(screen.getByTestId("automation-last-run-busy")).toHaveTextContent("Running now");

    renderRow(view("dependency-review"));
    const off = screen.getByTestId("automation-row-job-dependency-review");
    expect(within(off).getByTestId("automation-off-badge")).toHaveTextContent("Off");
    expect(screen.getByTestId("automation-stat-dependency-review")).toHaveTextContent("—");

    renderRow(view("release-checklist"));
    expect(
      within(screen.getByTestId("automation-row-job-release-checklist")).getByTestId(
        "automation-source-badge"
      )
    ).toHaveTextContent("From config");
  });

  it("Should keep the switch on its confirmed state while a flip is pending", () => {
    renderRow(view("morning-digest"), controls({ isTogglePending: () => true }));
    const toggle = screen.getByRole("switch", { name: "Turn morning-digest on or off" });
    expect(toggle).toHaveAttribute("aria-checked", "true");
    expect(toggle).toHaveAttribute("aria-disabled", "true");
  });

  it("Should flip from the keyboard through the named switch", async () => {
    const user = userEvent.setup();
    const rowControls = renderRow(view("morning-digest"));
    screen.getByRole("switch", { name: "Turn morning-digest on or off" }).focus();
    await user.keyboard(" ");
    expect(rowControls.onToggleEnabled).toHaveBeenCalledWith(view("morning-digest"), false);
  });

  it("Should disable the switch and Run now while automations are unavailable", async () => {
    const user = userEvent.setup();
    renderRow(view("morning-digest"), controls({ unavailable: true }));
    expect(screen.getByRole("switch", { name: "Turn morning-digest on or off" })).toHaveAttribute(
      "aria-disabled",
      "true"
    );
    await user.click(screen.getByRole("button", { name: "More actions for morning-digest" }));
    expect(await screen.findByTestId("automation-run-now-morning-digest")).toHaveAttribute(
      "aria-disabled",
      "true"
    );
  });

  it.each([
    ["morning-digest", ["Run now", "Edit", "Delete automation…"]],
    ["summarize-failures", ["Edit", "Delete automation…"]],
    ["deploy-webhook", ["Copy link", "Edit", "Delete automation…"]],
    ["release-checklist", ["Run now", "Edit in config.toml"]],
  ])("Should offer only supported actions for %s", async (id, labels) => {
    const user = userEvent.setup();
    renderRow(view(id));
    await user.click(screen.getByRole("button", { name: `More actions for ${id}` }));
    const items = await screen.findAllByRole("menuitem");
    expect(items.map(item => item.textContent?.trim())).toEqual(labels);
  });
});

describe("AutomationRow last-run instant", () => {
  it("Should date the last-ran stat and the last-run truth line by the same instant", () => {
    const startedAt = new Date(Date.now() - 150_000).toISOString();
    const endedAt = new Date(Date.now() - 90_000).toISOString();
    renderRow(
      withEntity({
        ...automationStoryTriggers[2],
        last_run: { id: "run_x", status: "failed", started_at: startedAt, ended_at: endedAt },
      })
    );
    const stat = screen.getByTestId("automation-stat-deploy-webhook").querySelector("time");
    const truth = screen.getByTestId("automation-last-run-deploy-webhook").querySelector("time");
    expect(stat).toHaveAttribute("datetime", startedAt);
    expect(truth).toHaveAttribute("datetime", startedAt);
    expect(stat?.textContent).toBe(truth?.textContent);
  });
});

describe("AutomationCard", () => {
  it("Should caption the start kind and exceptions without an overflow", () => {
    for (const id of ["morning-digest", "summarize-failures", "deploy-webhook"]) {
      render(<AutomationCard controls={controls()} view={view(id)} />);
    }
    render(<AutomationCard controls={controls()} view={view("dependency-review")} />);
    render(<AutomationCard controls={controls()} view={view("release-checklist")} />);
    expect(screen.getByTestId("automation-card-caption-morning-digest")).toHaveTextContent(
      "Scheduled"
    );
    expect(screen.getByTestId("automation-card-caption-summarize-failures")).toHaveTextContent(
      "On an event"
    );
    expect(screen.getByTestId("automation-card-caption-deploy-webhook")).toHaveTextContent(
      "Webhook"
    );
    expect(screen.getByTestId("automation-card-caption-dependency-review")).toHaveTextContent(
      "Scheduled · Off"
    );
    expect(screen.getByTestId("automation-card-caption-release-checklist")).toHaveTextContent(
      "Scheduled · From config"
    );
    expect(screen.queryByRole("button", { name: /More actions/ })).toBeNull();
    expect(screen.getAllByRole("switch")).toHaveLength(5);
    expect(screen.getByTestId("automation-card-foot-morning-digest")).toHaveTextContent(
      /^Next run in 1[34]h$/
    );
    expect(screen.getByTestId("automation-card-foot-summarize-failures")).toHaveTextContent(
      /^Last ran 2h ago$/
    );
    expect(screen.getByTestId("automation-card-foot-dependency-review")).toHaveTextContent("—");
  });
});

describe("AutomationCatalogShell", () => {
  const base = {
    view: "rows" as const,
    isLoading: false,
    loadError: null,
    hasActiveFilters: false,
    pagination: { hasNextPage: false, isFetchingNextPage: false },
    onClearFilters: vi.fn(),
    firstRunActions: <button type="button">On a schedule</button>,
    profileScope: scopedListingScopeFixture,
    children: null,
  };

  it("Should teach the model on first run", () => {
    render(<AutomationCatalogShell {...base} itemCount={0} />);
    const empty = screen.getByTestId("automations-list-empty");
    expect(empty).toHaveTextContent("No automations in");
    expect(empty).toHaveTextContent(
      "An automation runs an agent, a Loop or a task on a schedule, or when something happens."
    );
    expect(screen.getByRole("button", { name: "On a schedule" })).toBeVisible();
  });

  it("Should name every profile in aggregate mode", () => {
    render(
      <AutomationCatalogShell {...base} itemCount={0} profileScope={aggregateListingScopeFixture} />
    );
    expect(screen.getByTestId("automations-list-empty")).toHaveTextContent("in any profile yet");
  });

  it("Should offer Clear filters when filters empty the list", async () => {
    const onClearFilters = vi.fn();
    render(
      <AutomationCatalogShell
        {...base}
        hasActiveFilters
        itemCount={0}
        onClearFilters={onClearFilters}
      />
    );
    expect(screen.getByTestId("automations-list-filtered-empty")).toHaveTextContent(
      "No automations match"
    );
    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(onClearFilters).toHaveBeenCalledOnce();
  });

  it("Should show skeleton rows while loading and Try again on a load error", async () => {
    const { unmount } = render(<AutomationCatalogShell {...base} isLoading itemCount={0} />);
    expect(screen.getByTestId("automations-list-loading")).toHaveAttribute("aria-busy", "true");
    unmount();

    const onRetry = vi.fn();
    render(
      <AutomationCatalogShell
        {...base}
        itemCount={0}
        loadError={{ message: "daemon down", onRetry }}
      />
    );
    expect(screen.getByTestId("automations-list-error")).toHaveTextContent(
      "Unable to load automations"
    );
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });
});
