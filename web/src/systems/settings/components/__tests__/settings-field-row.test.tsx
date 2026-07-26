import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PillGroup } from "@agh/ui";

import { SettingRow } from "../setting-row";
import { ModalSettingsFieldRow, SettingsFieldRow } from "../settings-field-row";

describe("SettingsFieldRow", () => {
  it("renders labels, descriptions, and controls without jargon chips", () => {
    render(
      <SettingsFieldRow
        label="Default provider"
        description="Used for new sessions"
        control={<input />}
        data-testid="field-row"
      />
    );

    const row = screen.getByTestId("field-row");
    expect(row).toHaveTextContent("Default provider");
    expect(row).toHaveTextContent("Used for new sessions");
    expect(screen.getByLabelText("Default provider")).toBeInTheDocument();
  });

  it("forwards the error message when provided", () => {
    render(
      <SettingsFieldRow
        label="API key"
        error="required"
        control={<input />}
        data-testid="field-row"
      />
    );

    const row = screen.getByTestId("field-row");
    expect(row).toHaveTextContent("required");
    expect(screen.getByLabelText("API key")).toHaveAttribute("aria-invalid", "true");
  });

  it("renders the srow anatomy by default and the Field container in ModalSettingsFieldRow", () => {
    render(
      <SettingsFieldRow label="Session timeout" control={<input />} data-testid="field-row" />
    );
    expect(screen.getByTestId("field-row")).toHaveAttribute("data-slot", "setting-row");

    render(
      <ModalSettingsFieldRow
        control={<input />}
        data-testid="field-row-modal"
        label="Display name"
      />
    );
    expect(screen.getByTestId("field-row-modal")).toHaveAttribute("data-slot", "field");
  });

  it("labels composite control groups with the field label", () => {
    render(
      <SettingsFieldRow
        label="Burst limit"
        description="Applies to queue and request windows"
        control={
          <div>
            <input aria-label="requests" />
            <input aria-label="queue" />
          </div>
        }
        data-testid="field-row"
      />
    );

    expect(screen.getByRole("group", { name: "Burst limit" })).toHaveAttribute(
      "aria-describedby",
      expect.stringContaining("description")
    );
  });

  it("labels custom grouped controls through aria-labelledby", () => {
    render(
      <SettingsFieldRow
        label="Catalog scope"
        control={
          <PillGroup
            items={[
              { value: "global", label: "Global" },
              { value: "workspace", label: "Workspace" },
            ]}
            onChange={() => undefined}
            value="global"
          />
        }
        data-testid="field-row"
      />
    );

    expect(screen.getByRole("group", { name: "Catalog scope" })).toBeInTheDocument();
  });

  it.each([
    ["SettingRow", SettingRow],
    ["SettingsFieldRow", SettingsFieldRow],
  ])("associates a Fragment control through a native group root in %s", (_, Row) => {
    render(
      <Row
        label="Session timeout"
        description="Ends inactive sessions"
        error="Enter a whole number."
        control={
          <>
            <input aria-label="Seconds" />
            <span>seconds</span>
          </>
        }
        data-testid="fragment-row"
      />
    );

    const group = screen.getByRole("group", { name: "Session timeout" });
    expect(group).toHaveAttribute("aria-describedby", expect.stringContaining("description"));
    expect(group).toHaveAttribute("aria-describedby", expect.stringContaining("error"));
    expect(group).not.toHaveAttribute("aria-invalid");
  });
});
