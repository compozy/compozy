import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SettingsRestartNotice } from "../settings-restart-notice";

describe("SettingsRestartNotice", () => {
  it("Should expose restart-request progress through the action contract", () => {
    render(
      <SettingsRestartNotice
        restart={{
          activeSessionCount: 0,
          dismiss: vi.fn(),
          isRestartRequired: true,
          isTriggerPending: true,
          isVisible: true,
          operationId: null,
          status: null,
          trigger: vi.fn(),
        }}
        slug="general"
      />
    );

    const trigger = screen.getByTestId("settings-page-general-restart-trigger");
    expect(trigger).toBeDisabled();
    expect(trigger).toHaveAttribute("aria-busy", "true");
    expect(trigger).toHaveTextContent("Starting…");
    expect(screen.queryByTestId("settings-page-general-restart-dismiss")).toBeNull();
  });
});

// Invariant: a failed restart shows the daemon reason and active-session warning
// while retaining an operable retry action. Owner: SettingsRestartNotice component.
it("Should retain the failure reason, active-session warning, and working retry control", async () => {
  const { default: userEvent } = await import("@testing-library/user-event");
  const user = userEvent.setup();
  const trigger = vi.fn();
  render(
    <SettingsRestartNotice
      restart={{
        activeSessionCount: 2,
        dismiss: vi.fn(),
        failureReason: "browser restart fault injection",
        isRestartRequired: true,
        isTriggerPending: false,
        isVisible: true,
        operationId: "op-settings-failed",
        status: "failed",
        trigger,
      }}
      slug="general"
    />
  );

  const notice = screen.getByTestId("settings-page-general-restart-notice");
  expect(notice).toHaveTextContent("Restart failed");
  expect(notice).toHaveTextContent("browser restart fault injection");
  expect(notice).toHaveTextContent("2 active sessions");
  const retry = screen.getByRole("button", { name: "Try again" });
  expect(retry).toBeEnabled();
  await user.click(retry);
  expect(trigger).toHaveBeenCalledOnce();
});
