import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { UIProvider } from "@compozy/ui";

import { terminalSettingsFixture } from "../../mocks/fixtures";
import { parsePositiveDurationMilliseconds } from "../../lib/terminal-settings-duration";
import { readTerminalSettings } from "../../lib/terminal-settings-projection";
import {
  TerminalSettingsSections,
  type TerminalSettingsConfig,
} from "../terminal-settings-sections";

/**
 * Canonical suite for the `[terminal]` settings projection (part of UT-117).
 *
 * Invariant: every documented key renders with its live value, a per-key refusal
 * is shown against the key it belongs to, and the surface offers no control the
 * runtime does not expose.
 */

function Harness({
  initial = terminalSettingsFixture,
  validationErrors = {},
}: {
  initial?: TerminalSettingsConfig;
  validationErrors?: Record<string, string | null>;
}) {
  const [draft, setDraft] = useState<TerminalSettingsConfig | null>(initial);
  if (draft === null) return null;
  return (
    <UIProvider reducedMotion="never" skipAnimations>
      <TerminalSettingsSections
        draft={draft}
        setDraft={setDraft}
        validationErrors={validationErrors}
      />
    </UIProvider>
  );
}

describe("TerminalSettingsSections", () => {
  it("Should render each documented key with its live value", () => {
    render(<Harness />);

    expect(screen.getByTestId("settings-terminal-default-shell")).toHaveValue("");
    expect(screen.getByTestId("settings-terminal-shell-integration")).toBeChecked();
    expect(screen.getByTestId("settings-terminal-detached-ttl")).toHaveValue("24h");
    expect(screen.getByTestId("settings-terminal-exit-retention")).toHaveValue("15m");
    expect(screen.getByTestId("settings-terminal-recording")).not.toBeChecked();
    expect(screen.getByTestId("settings-terminal-recording-retention")).toHaveValue("30");
    expect(screen.getByTestId("settings-terminal-max-per-workspace")).toHaveValue("8");
    expect(screen.getByTestId("settings-terminal-max-per-daemon")).toHaveValue("32");
    expect(screen.getByTestId("settings-terminal-max-subscribers")).toHaveValue("16");
    // 1 MiB, shown in the unit a person would read it in.
    expect(screen.getByLabelText("Scrollback kept per terminal")).toHaveValue("1");
    expect(screen.getByLabelText("Scrollback kept per terminal unit")).toHaveValue("MB");
  });

  it("Should never project autonomy policy, which permissions owns", () => {
    const { container } = render(<Harness />);

    // The allowlist and its tiers live with the permissions surfaces; a control
    // here would be a second policy editor for the same decision.
    expect(container.textContent).not.toContain("allowlist");
    expect(container.textContent).not.toContain("Autonomy");
  });

  it("Should show a refusal against the key it belongs to", () => {
    render(
      <Harness validationErrors={{ detached_ttl: "detached_ttl must be a duration like 24h." }} />
    );

    const row = screen.getByTestId("settings-terminal-detached-ttl-row");
    expect(row).toHaveTextContent("detached_ttl must be a duration like 24h.");
    expect(screen.getByTestId("settings-terminal-exit-retention-row")).not.toHaveTextContent(
      "must be a duration"
    );
  });

  it("Should write an edited value back into the draft", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    const shell = screen.getByTestId("settings-terminal-default-shell");
    await user.type(shell, "/bin/bash");

    expect(shell).toHaveValue("/bin/bash");
  });

  it("Should carry a limit change through to the control", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    const limit = screen.getByTestId("settings-terminal-max-per-workspace");
    await user.clear(limit);
    await user.type(limit, "4");

    expect(limit).toHaveValue("4");
  });
});

/**
 * The projection that decides whether this section renders at all.
 *
 * Invariant: the ten keys are shown only when the daemon actually projected all
 * ten, with the type each is supposed to have. A form filled with values the
 * runtime never sent would be edited and saved back as if a person had chosen
 * them.
 */
describe("readTerminalSettings", () => {
  const FULL_BLOCK = { ...terminalSettingsFixture, default_shell: "/bin/zsh" };

  it("Should keep every value the daemon projected", () => {
    expect(readTerminalSettings({ terminal: FULL_BLOCK })).toEqual(FULL_BLOCK);
  });

  it("Should treat an empty default shell as a real answer", () => {
    // Empty is how "use the login shell" is spelled — it is present, so it is
    // a value, not a gap.
    const projected = readTerminalSettings({ terminal: { ...FULL_BLOCK, default_shell: "" } });

    expect(projected?.default_shell).toBe("");
  });

  it("Should refuse a block that is missing a key", () => {
    for (const key of Object.keys(FULL_BLOCK)) {
      const partial = { ...FULL_BLOCK } as Record<string, unknown>;
      delete partial[key];

      expect(readTerminalSettings({ terminal: partial })).toBeNull();
    }
  });

  it("Should refuse a value that is not the type the key requires", () => {
    const wrong: Array<[string, unknown]> = [
      ["default_shell", 7],
      ["detached_ttl", null],
      ["exit_retention", 15],
      ["shell_integration", "true"],
      ["recording", 0],
      ["scrollback_bytes", "1048576"],
      ["recording_retention_days", "30"],
      ["max_per_workspace", Number.NaN],
      ["max_per_daemon", Infinity],
      ["max_subscribers", false],
    ];
    for (const [key, value] of wrong) {
      expect(readTerminalSettings({ terminal: { ...FULL_BLOCK, [key]: value } })).toBeNull();
    }
  });

  it("Should parse every positive duration shape accepted by the form", () => {
    expect(parsePositiveDurationMilliseconds("1h30m")).toBe(5_400_000);
    expect(parsePositiveDurationMilliseconds("1.5s")).toBe(1_500);
    expect(parsePositiveDurationMilliseconds("250ms")).toBe(250);
    expect(parsePositiveDurationMilliseconds("0s")).toBeUndefined();
    expect(parsePositiveDurationMilliseconds("15 minutes")).toBeUndefined();
  });

  it("Should render nothing at all when the block is absent", () => {
    expect(readTerminalSettings({})).toBeNull();
    expect(readTerminalSettings({ terminal: {} })).toBeNull();
    expect(readTerminalSettings(null)).toBeNull();
  });
});
