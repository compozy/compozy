// Suite: Settings › Automation route composition.
// Invariant: the Manage group links to the one Automations window with combined counts, and
// the engine/limit fields keep their config keys under the plain automation labels.
// The timezone picker searches IANA choices, preserves unmatched input, and uses the daemon host zone.
// Owning layer: Settings Automation route page. Boundary OUT: the settings page model (owned
// by use-settings-automation-page.test.tsx) and the router.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type AnchorHTMLAttributes, type PropsWithChildren } from "react";
import { describe, expect, it, vi } from "vitest";

import type { SettingsAutomationSection } from "@/systems/settings";

const envelope: SettingsAutomationSection = {
  section: "automation",
  scope: "user",
  available_scopes: ["user"],
  config: {
    enabled: true,
    timezone: "UTC",
    max_concurrent_jobs: 4,
    default_fire_limit: { max: 5, window: "1m" },
  },
  runtime: {
    host_timezone: "America/Sao_Paulo",
    available: false,
    running: false,
    scheduler_running: false,
    job_enabled: 3,
    job_total: 4,
    trigger_enabled: 3,
    trigger_total: 3,
  },
  links: [],
};

vi.mock("@tanstack/react-router", () => ({
  Link: ({ to, children, ...props }: AnchorHTMLAttributes<HTMLAnchorElement> & { to: string }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("@/systems/settings/hooks/use-settings-automation-page", () => ({
  useSettingsAutomationPage: () => {
    const [draft, setDraft] = useState(envelope.config);
    return {
      envelope,
      draft,
      setDraft,
      restart: { isVisible: false },
      isLoading: false,
      error: null,
      isDirty: false,
      isSaving: false,
      saveError: null,
      warnings: [],
      lastAppliedLabel: null,
      handleSave: vi.fn(),
      handleReset: vi.fn(),
      handleRetry: vi.fn(),
    };
  },
}));

vi.mock("@/systems/settings", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/settings")>()),
  SettingsPageFrame: ({ children }: PropsWithChildren) => <main>{children}</main>,
  SettingsSaveBar: () => null,
  useSettingsSaveBarState: () => ({ kind: "clean" }),
  useSettingsTopbar: vi.fn(),
}));

import { AutomationSettingsPage } from "../-automation-settings-page";

describe("AutomationSettingsPage", () => {
  it("Should search IANA zones and explain a mismatch with the daemon host", async () => {
    const user = userEvent.setup();
    render(<AutomationSettingsPage />);
    expect(
      screen.getByText("Schedules use UTC. The machine running CompozyOS uses America/Sao_Paulo.")
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Schedule time zone" }));
    await user.type(screen.getByRole("combobox", { name: "Search time zones" }), "Tokyo");
    await user.click(await screen.findByRole("option", { name: "Asia/Tokyo" }));
    expect(screen.getByRole("button", { name: "Schedule time zone" })).toHaveTextContent(
      "Asia/Tokyo"
    );
    await user.click(
      screen.getByRole("button", { name: "Use this machine’s time zone (America/Sao_Paulo)" })
    );
    expect(screen.getByRole("button", { name: "Schedule time zone" })).toHaveTextContent(
      "America/Sao_Paulo"
    );
    expect(screen.queryByText(/Schedules use/)).not.toBeInTheDocument();
  });

  it.each(["Invalid/Zone", "+03:00"])(
    "Should preserve the zone for non-IANA search %s",
    async search => {
      const user = userEvent.setup();
      render(<AutomationSettingsPage />);
      await user.click(screen.getByRole("button", { name: "Schedule time zone" }));
      await user.type(screen.getByRole("combobox", { name: "Search time zones" }), search);
      expect(await screen.findByText("No time zones found.")).toBeInTheDocument();
      await user.keyboard("{Escape}");
      expect(screen.getByRole("button", { name: "Schedule time zone" })).toHaveTextContent("UTC");
    }
  );

  it("Should select a valid fixed-offset IANA zone missing from the browser list", async () => {
    const user = userEvent.setup();
    render(<AutomationSettingsPage />);
    await user.click(screen.getByRole("button", { name: "Schedule time zone" }));
    await user.type(screen.getByRole("combobox", { name: "Search time zones" }), "Etc/GMT+3");
    await user.click(await screen.findByRole("option", { name: "Etc/GMT+3" }));
    expect(screen.getByRole("button", { name: "Schedule time zone" })).toHaveTextContent(
      "Etc/GMT+3"
    );
  });

  it("Should link one Automations row with combined counts and plain labels [UT-113]", async () => {
    const user = userEvent.setup();
    render(<AutomationSettingsPage />);

    const manage = screen.getByTestId("settings-page-automation-operational-links");
    const row = screen.getByTestId("settings-page-automation-link-automations");
    expect(manage.querySelectorAll("a")).toHaveLength(1);
    expect(row).toHaveAttribute("href", "/automations");
    expect(row).toHaveTextContent("Automations");
    expect(row).toHaveTextContent("7 automations, 6 on · 4 scheduled, 3 on events");

    expect(screen.getByTestId("settings-page-automation-runtime-unavailable")).toHaveTextContent(
      "Turn on Run automation and restart CompozyOS. Your automations wait until then."
    );
    await user.click(screen.getByRole("button", { name: "About schedule time zone" }));
    expect(await screen.findByText("Scheduled automations use this time zone")).toBeInTheDocument();

    await user.click(screen.getByText("Advanced — limits"));
    expect(screen.getByTestId("settings-page-automation-max-concurrent")).toHaveTextContent(
      "Scheduled automations at once"
    );
    await user.click(screen.getByRole("button", { name: "About scheduled automations at once" }));
    expect(
      await screen.findByText("How many scheduled automations can run at the same time")
    ).toBeInTheDocument();
    expect(screen.getByText("automation.max_concurrent_jobs")).toBeInTheDocument();

    expect(screen.getByTestId("settings-page-automation-fire-limit-max")).toHaveTextContent(
      "Default run limit"
    );
    await user.click(screen.getByRole("button", { name: "About default run limit" }));
    expect(
      await screen.findByText("How often a new automation can start work")
    ).toBeInTheDocument();
    expect(screen.getByText("automation.default_fire_limit")).toBeInTheDocument();
  });
});
