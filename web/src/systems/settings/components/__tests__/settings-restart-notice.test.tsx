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
          isFailed: false,
          isPolling: false,
          isRestartRequired: true,
          isSuccessful: false,
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
