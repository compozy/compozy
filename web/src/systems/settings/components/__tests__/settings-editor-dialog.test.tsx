import { fireEvent, render, screen } from "@testing-library/react";
import { KeyRound } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { SettingsEditorDialog } from "../settings-editor-dialog";

function baseProps() {
  return {
    open: true,
    mode: "create" as const,
    icon: KeyRound,
    eyebrow: "System · Widgets",
    size: "sm" as const,
    title: "New widget",
    slug: "widgets",
    canSave: true,
    isSaving: false,
    onSave: vi.fn(),
    onOpenChange: vi.fn(),
  };
}

describe("SettingsEditorDialog", () => {
  it("renders title, body, and create save label in create mode", () => {
    const props = baseProps();
    render(
      <SettingsEditorDialog {...props}>
        <div data-testid="body">hello</div>
      </SettingsEditorDialog>
    );
    expect(screen.getByTestId("settings-widgets-editor-title")).toHaveTextContent("New widget");
    expect(screen.getByTestId("body")).toBeInTheDocument();
    expect(screen.getByTestId("settings-widgets-editor-save")).toHaveTextContent("Create");
  });

  it("switches the save label to the replace action in edit mode", () => {
    const props = { ...baseProps(), mode: "edit" as const, title: "Edit widget" };
    render(
      <SettingsEditorDialog {...props}>
        <div />
      </SettingsEditorDialog>
    );
    expect(screen.getByTestId("settings-widgets-editor-save")).toHaveTextContent("Save changes");
  });

  it("disables save when canSave is false", () => {
    const props = { ...baseProps(), canSave: false };
    render(
      <SettingsEditorDialog {...props}>
        <div />
      </SettingsEditorDialog>
    );
    expect(screen.getByTestId("settings-widgets-editor-save")).toBeDisabled();
  });

  it("surfaces validation errors inline", () => {
    const props = { ...baseProps(), error: "Field missing" };
    render(
      <SettingsEditorDialog {...props}>
        <div />
      </SettingsEditorDialog>
    );
    expect(screen.getByTestId("settings-widgets-editor-error")).toHaveTextContent("Field missing");
  });

  it("renders warnings when no error is present", () => {
    const props = { ...baseProps(), warnings: ["restart required", "applied to new sessions"] };
    render(
      <SettingsEditorDialog {...props}>
        <div />
      </SettingsEditorDialog>
    );
    const warnings = screen.getByTestId("settings-widgets-editor-warnings");
    expect(warnings).toHaveTextContent("restart required");
    expect(warnings).toHaveTextContent("applied to new sessions");
  });

  it("invokes onSave and onOpenChange from the footer controls", () => {
    const props = baseProps();
    render(
      <SettingsEditorDialog {...props}>
        <div />
      </SettingsEditorDialog>
    );
    fireEvent.click(screen.getByTestId("settings-widgets-editor-save"));
    expect(props.onSave).toHaveBeenCalled();
    fireEvent.click(screen.getByTestId("settings-widgets-editor-cancel"));
    expect(props.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should expose the editor title and eyebrow through the dialog", () => {
    render(
      <SettingsEditorDialog {...baseProps()}>
        <div />
      </SettingsEditorDialog>
    );
    expect(screen.getByRole("dialog", { name: "New widget" })).toBeInTheDocument();
    expect(screen.getByText("System · Widgets")).toBeInTheDocument();
  });

  it("Should render the footer consequence hint", () => {
    const props = { ...baseProps(), hint: "Stored write-only; AGH returns presence only." };
    render(
      <SettingsEditorDialog {...props}>
        <div />
      </SettingsEditorDialog>
    );
    expect(screen.getByText("Stored write-only; AGH returns presence only.")).toBeVisible();
  });

  it("Should omit the feedback region entirely when there is nothing to report", () => {
    render(
      <SettingsEditorDialog {...baseProps()}>
        <div />
      </SettingsEditorDialog>
    );
    // An always-present wrapper would open a dead grid row between body and footer.
    expect(screen.queryByTestId("settings-widgets-editor-feedback")).not.toBeInTheDocument();
  });
});
