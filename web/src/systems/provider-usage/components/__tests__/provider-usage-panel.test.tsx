// Suite: Provider usage components
// Invariant: The tray control, popover panel, rows, detail, and status line render only what
// the envelope backs — named meters with text values, no bar for unknown or unsupported data,
// money in native units, countdowns from absolute times on one shared ticker, actions the host
// can honor, and a daemon-unavailable banner that withdraws Refresh but keeps cached rows.
// Boundary IN: domain components fed view models built from `_dx.md` fixtures.
// Boundary OUT: request wiring (hooks suite), popover/overlay placement (os desktop-menubar).
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { PROVIDER_USAGE_CLOCK_INTERVAL_MS } from "../../hooks/use-usage-clock";
import { buildProviderUsageViewModel } from "../../lib/view-model";
import {
  PROVIDER_USAGE_FIXTURE_NOW as NOW,
  claudeOkTarget,
  codexOkTarget,
  defaultUsageEnvelope,
  edgeCaseTarget,
  statusTargets,
  usageEnvelope,
  usageTarget,
  usageTray,
  usageWindow,
} from "../../mocks/fixtures";
import type { ProviderUsageTarget } from "../../types";
import { ProviderUsagePanel, type ProviderUsagePanelProps } from "../provider-usage-panel";
import { ProviderUsageTrayMeter } from "../provider-usage-tray-meter";

const MINUTE = 60_000;
const iso = (offsetMs: number) => new Date(NOW + offsetMs).toISOString();

function viewModelOf(targets: ProviderUsageTarget[]) {
  return buildProviderUsageViewModel(usageEnvelope(targets, {}, NOW));
}

function renderPanel(props: Partial<ProviderUsagePanelProps> = {}) {
  const onRefresh = vi.fn();
  const onAction = vi.fn();
  const utils = render(
    <ProviderUsagePanel
      chrome="window"
      viewModel={buildProviderUsageViewModel(defaultUsageEnvelope(NOW))}
      isMacHost
      onRefresh={onRefresh}
      onAction={onAction}
      {...props}
    />
  );
  return { ...utils, onAction, onRefresh };
}

function row(providerName: string): HTMLElement {
  const trigger = screen.getByRole("button", { name: new RegExp(`^${providerName}`) });
  const element = trigger.closest<HTMLElement>("[data-slot=provider-usage-row]");
  if (!element) throw new Error(`no row for ${providerName}`);
  return element;
}

describe("Provider usage components", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true, toFake: ["Date", "setInterval", "clearInterval"] });
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  describe("ProviderUsageTrayMeter", () => {
    it("Should name the tightest window, percent, and reset (UT-050)", () => {
      render(<ProviderUsageTrayMeter tray={usageTray({}, NOW)} />);
      const button = screen.getByRole("button", {
        name: "Codex · Weekly 72% used · resets in 2d 4h",
      });
      expect(button).toHaveAttribute("aria-haspopup", "dialog");
      expect(button.querySelector("[data-slot=progress]")).toHaveAttribute("data-tone", "neutral");
    });

    it("Should hold the meter's shape, unmeasured, while the first read is pending", () => {
      render(<ProviderUsageTrayMeter tray={null} loading />);
      const button = screen.getByRole("button", { name: "Provider usage" });

      expect(button).toHaveAttribute("data-loading", "true");
      expect(button.querySelector("[data-slot=skeleton]")).not.toBeNull();
      expect(within(button).queryByRole("progressbar")).toBeNull();
    });

    it("Should map warning and exhausted to their tones and say limit reached", () => {
      const { rerender } = render(
        <ProviderUsageTrayMeter tray={usageTray({ used_percent: 88, severity: "warning" }, NOW)} />
      );
      expect(screen.getByRole("button").querySelector("[data-slot=progress]")).toHaveAttribute(
        "data-tone",
        "warning"
      );
      rerender(
        <ProviderUsageTrayMeter
          tray={usageTray({ used_percent: 112, severity: "exhausted" }, NOW)}
        />
      );
      const button = screen.getByRole("button", { name: /Codex · Weekly limit reached/ });
      expect(button.querySelector("[data-slot=progress]")).toHaveAttribute("data-tone", "danger");
      expect(button).not.toHaveAccessibleName(/112/);
    });

    it("Should render the glyph without a meter when no window is known (UT-051)", () => {
      render(<ProviderUsageTrayMeter tray={null} />);
      const button = screen.getByRole("button", { name: "Provider usage" });
      expect(button.querySelector("[data-slot=progress]")).toBeNull();
    });

    it("Should mark stale data by shape and words, not hue alone", () => {
      render(
        <ProviderUsageTrayMeter
          tray={usageTray({ stale: true }, NOW)}
          measuredAt={iso(-25 * MINUTE)}
        />
      );
      const button = screen.getByRole("button", { name: /updated 25 min ago$/ });
      expect(button).toHaveAttribute("data-stale", "true");
      expect(button.querySelector("[data-slot=status-dot]")).toHaveAttribute(
        "data-variant",
        "ring"
      );
    });
  });

  describe("ProviderUsagePanel", () => {
    it("Should list rows in daemon order with header freshness", () => {
      renderPanel();
      const list = screen.getByRole("list", { name: "Providers" });
      const [first, second] = within(list).getAllByRole("button");
      expect(first).toHaveAccessibleName(
        "Claude, Max 20x, Weekly · Opus 88% used, resets in 3d 2h"
      );
      expect(second).toHaveAccessibleName("Codex, Pro, Weekly 72% used, resets in 2d 4h");
      expect(screen.getByText("Updated 2 min ago")).toBeInTheDocument();
      expect(within(row("Claude")).getByText("88% used")).toBeInTheDocument();
    });

    it("Should collapse every row, signed-out ones included, to one line with a status cue", async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      const status = statusTargets(NOW);
      renderPanel({ viewModel: viewModelOf([codexOkTarget(NOW), status.signedOut]) });

      const measured = screen.getByRole("button", { name: /^Codex/ });
      const signedOut = screen.getByRole("button", { name: "Cursor, Signed out" });
      for (const trigger of [measured, signedOut]) {
        expect(trigger).toHaveAttribute("aria-expanded", "false");
      }
      expect(within(row("Codex")).getByText("72% used")).toBeInTheDocument();
      expect(within(row("Cursor")).getByText("Signed out")).toBeInTheDocument();
      expect(document.querySelector("[data-slot=provider-usage-status-line]")).toBeNull();
      expect(document.querySelector("[data-slot=provider-usage-detail]")).toBeNull();
      expect(screen.queryByText(/Sign in with cursor-agent/)).toBeNull();

      await user.click(signedOut);
      expect(signedOut).toHaveAttribute("aria-expanded", "true");
      expect(
        within(row("Cursor")).getByText("Signed out · Sign in with cursor-agent to read usage")
      ).toBeInTheDocument();
      await user.click(measured);
      expect(screen.getByRole("list", { name: "Codex limits" })).toBeInTheDocument();
    });

    it("Should fold unsupported rows into one closed trailing group, text only (UT-056)", async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      const status = statusTargets(NOW);
      renderPanel({
        viewModel: viewModelOf([status.unsupported, codexOkTarget(NOW), status.noPlan]),
      });

      const providers = screen.getByRole("list", { name: "Providers" });
      expect(within(providers).getAllByRole("listitem")).toHaveLength(1);
      const toggle = screen.getByRole("button", { name: "2 providers without usage data" });
      expect(toggle).toHaveAttribute("aria-expanded", "false");
      expect(screen.queryByText("Usage not available for this provider")).toBeNull();

      await user.click(toggle);
      const quiet = screen.getByRole("list", { name: "2 providers without usage data" });
      const names = within(quiet)
        .getAllByRole("listitem")
        .map(item => item.getAttribute("data-provider"));
      expect(names).toEqual(["pi", "zai"]);
      expect(within(quiet).getByText("Usage not available for this provider")).toBeInTheDocument();
      expect(quiet.querySelector("[data-slot=progress]")).toBeNull();
      expect(quiet.querySelector("[data-slot=status-dot]")).toBeNull();
      expect(within(quiet).getByRole("button", { name: "Open settings" })).toBeInTheDocument();
    });

    it("Should keep a row expanded while new revisions arrive (UT-055)", async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      const { rerender } = renderPanel();
      await user.click(screen.getByRole("button", { name: /^Codex/ }));
      expect(screen.getByRole("list", { name: "Codex limits" })).toBeInTheDocument();

      const next = codexOkTarget(NOW);
      next.windows = next.windows.map(window =>
        window.id === "weekly" ? { ...window, used_percent: 75 } : window
      );
      rerender(
        <ProviderUsagePanel
          chrome="window"
          viewModel={buildProviderUsageViewModel(
            usageEnvelope([claudeOkTarget(NOW), next], { revision: 15 }, NOW)
          )}
          isMacHost
        />
      );
      const limits = screen.getByRole("list", { name: "Codex limits" });
      expect(within(limits).getByText(/75% used/)).toBeInTheDocument();
    });

    it("Should label every window and keep money in its units (UT-057)", async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      renderPanel({ viewModel: viewModelOf([claudeOkTarget(NOW), codexOkTarget(NOW)]) });
      await user.click(screen.getByRole("button", { name: /^Claude/ }));
      await user.click(screen.getByRole("button", { name: /^Codex/ }));
      const claudeLimits = screen.getByRole("list", { name: "Claude limits" });
      expect(within(claudeLimits).getAllByRole("progressbar")).toHaveLength(3);
      expect(
        within(claudeLimits).getByRole("progressbar", { name: "Weekly · Opus" })
      ).toHaveAttribute("aria-valuetext", "88% used");
      expect(screen.getByText("$12.40 of $50.00")).toBeInTheDocument();
      expect(screen.getByText("120 credits")).toBeInTheDocument();
      const detail = within(
        row("Claude").querySelector<HTMLElement>("[data-slot=provider-usage-detail]") ??
          document.body
      );
      expect(detail.getByText("Claude Code OAuth")).toBeInTheDocument();
      expect(detail.getByText("Max 20x")).toBeInTheDocument();
      expect(detail.getByText("pedro@example.com")).toBeInTheDocument();
    });

    it("Should draw no bar for unknown percent and never call a missing cap unlimited (UT-058, UT-059)", async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      renderPanel({ viewModel: viewModelOf([edgeCaseTarget(NOW)]) });
      await user.click(screen.getByRole("button", { name: /^OpenRouter/ }));
      const limits = screen.getByRole("list", { name: "OpenRouter limits" });
      const daily = within(limits).getByText("Daily").closest("li");
      expect(daily).toHaveTextContent("Usage unknown · resets in 3h");
      expect(daily?.querySelector("[data-slot=progress]")).toBeNull();
      expect(screen.getByText("$8.10 spent")).toBeInTheDocument();
      expect(screen.queryByText(/unlimited/i)).toBeNull();
      // Two currencies, two lines, no total.
      expect(screen.getByText("$8.10")).toBeInTheDocument();
      expect(screen.getByText("CN¥30.00")).toBeInTheDocument();
      // Reset text verbatim, no countdown; >100 % only in the detail.
      expect(
        within(limits).getByText(/112% used · limit reached · Resets with your next billing cycle/)
      ).toBeInTheDocument();
      expect(within(limits).getByText(/Resetting…/)).toBeInTheDocument();
      expect(within(limits).getByText(/Resets Oct 28/)).toBeInTheDocument();
    });

    it("Should update countdowns on the shared ticker without fetching (UT-060)", () => {
      const fetchSpy = vi.spyOn(globalThis, "fetch");
      const soon = usageTarget({
        provider_id: "codex",
        provider_name: "Codex",
        measured_at: iso(0),
        windows: [
          usageWindow({
            id: "session",
            label: "Session",
            used_percent: 10,
            resets_at: iso(2 * MINUTE),
          }),
        ],
      });
      renderPanel({ viewModel: viewModelOf([soon]) });
      const trigger = screen.getByRole("button", { name: /^Codex/ });
      expect(trigger).toHaveAccessibleName("Codex, Session 10% used, resets in 2m");
      act(() => {
        vi.advanceTimersByTime(PROVIDER_USAGE_CLOCK_INTERVAL_MS);
      });
      expect(trigger).toHaveAccessibleName("Codex, Session 10% used, resets in 1m");
      act(() => {
        vi.advanceTimersByTime(4 * PROVIDER_USAGE_CLOCK_INTERVAL_MS);
      });
      expect(trigger).toHaveAccessibleName("Codex, Session 10% used, Resetting…");
      expect(fetchSpy).not.toHaveBeenCalled();
    });

    it("Should keep cached rows under a banner and withdraw actions when the daemon is gone (UT-062)", async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      renderPanel({
        daemonUnavailable: true,
        viewModel: viewModelOf([codexOkTarget(NOW), statusTargets(NOW).error]),
      });
      await user.click(screen.getByRole("button", { name: /^Cursor/ }));
      expect(within(row("Cursor")).getByText("Couldn't load usage")).toBeInTheDocument();
      expect(screen.getByRole("alert")).toHaveTextContent("Daemon unavailable");
      expect(screen.queryByRole("button", { name: /Refresh provider usage/ })).toBeNull();
      expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
      expect(within(row("Codex")).getByText("72% used")).toBeInTheDocument();
    });

    it("Should offer status actions and Allow only on a macOS host (UT-063)", async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      const status = statusTargets(NOW);
      const { onAction, onRefresh, rerender } = renderPanel({
        viewModel: viewModelOf([status.needsConsent, status.error]),
      });
      await user.click(screen.getByRole("button", { name: "Claude, Needs access" }));
      await user.click(screen.getByRole("button", { name: "Cursor, Couldn't load" }));
      await user.click(within(row("Claude")).getByRole("button", { name: "Allow" }));
      expect(onAction).toHaveBeenCalledWith("claude", "allow_keychain");
      await user.click(within(row("Cursor")).getByRole("button", { name: "Retry" }));
      expect(onAction).toHaveBeenCalledWith("cursor", "retry");
      await user.click(screen.getByRole("button", { name: "Refresh provider usage" }));
      expect(onRefresh).toHaveBeenCalledTimes(1);

      rerender(
        <ProviderUsagePanel
          chrome="window"
          viewModel={viewModelOf([status.needsConsent])}
          isMacHost={false}
          onAction={onAction}
        />
      );
      expect(within(row("Claude")).getByText("Needs Keychain access")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Allow" })).toBeNull();
    });

    it("Should show a busy header Refresh while an explicit refresh runs", () => {
      renderPanel({ refreshing: true });
      const button = screen.getByRole("button", { name: "Refreshing provider usage" });
      expect(button).toHaveAttribute("aria-busy", "true");
      expect(button).toBeDisabled();
    });

    it("Should hold the rows' place on the first read and show the turned-off state", () => {
      const { rerender } = renderPanel({ viewModel: null, loading: true });
      // One named loading area in the rows' own shape, claiming nothing about the providers.
      const region = screen.getByRole("status");
      expect(region).toHaveAttribute("aria-busy", "true");
      expect(region).toHaveTextContent("Loading provider usage");
      expect(region.querySelectorAll("li")).toHaveLength(3);
      expect(screen.queryByText("No providers to show")).toBeNull();
      expect(screen.queryByText("Provider usage is turned off")).toBeNull();
      const onOpenSettings = vi.fn();
      rerender(
        <ProviderUsagePanel
          chrome="window"
          viewModel={buildProviderUsageViewModel(usageEnvelope([], { enabled: false }, NOW))}
          isMacHost
          onRefresh={vi.fn()}
          onOpenSettings={onOpenSettings}
        />
      );
      expect(screen.queryByRole("status")).toBeNull();
      expect(screen.getByText("Provider usage is turned off")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /Refresh provider usage/ })).toBeNull();
    });
  });
});
