import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type {
  CreateNotificationPresetRequest,
  NotificationPresetEntry,
} from "@/systems/notifications";
import { NotificationPresetsPanel } from "../notification-presets-panel";

const builtInPreset: NotificationPresetEntry = {
  name: "task_terminal",
  events: ["task.run_*"],
  targets: [{ bridge_id: "bridge_slack_ops", canonical_route: "channel:ops" }],
  filter: "",
  enabled: false,
  built_in: true,
  default_version: "1",
  default_hash: "sha256:task-terminal",
  user_modified: false,
  default_update_available: false,
  created_at: "2026-05-21T10:00:00Z",
  updated_at: "2026-05-21T10:00:00Z",
};

const customPreset: NotificationPresetEntry = {
  ...builtInPreset,
  name: "custom_failure",
  events: ["provider.*"],
  targets: [],
  built_in: false,
  user_modified: true,
  enabled: true,
};

const secondCustomPreset: NotificationPresetEntry = {
  ...customPreset,
  name: "custom_warning",
  enabled: false,
};

describe("NotificationPresetsPanel", () => {
  it("renders seeded built-ins and prevents deleting them", () => {
    render(
      <NotificationPresetsPanel
        presets={[builtInPreset]}
        isLoading={false}
        error={null}
        pendingName={null}
        canMutate
        onCreate={vi.fn()}
        onToggle={vi.fn()}
        onDelete={vi.fn()}
      />
    );

    expect(
      screen.getByTestId("settings-page-hooks-notification-preset-row-task_terminal")
    ).toHaveTextContent("built-in");
    expect(
      screen.getByTestId("settings-page-hooks-notification-preset-row-task_terminal-delete")
    ).toBeDisabled();
  });

  it("creates presets with explicit event and bridge target fields", () => {
    const onCreate = vi.fn<(body: CreateNotificationPresetRequest) => void>();
    render(
      <NotificationPresetsPanel
        presets={[]}
        isLoading={false}
        error={null}
        pendingName={null}
        canMutate
        onCreate={onCreate}
        onToggle={vi.fn()}
        onDelete={vi.fn()}
      />
    );

    fireEvent.click(screen.getByTestId("settings-page-hooks-notification-preset-new"));
    fireEvent.change(screen.getByTestId("settings-page-hooks-notification-preset-name"), {
      target: { value: "custom_task" },
    });
    fireEvent.change(screen.getByTestId("settings-page-hooks-notification-preset-events"), {
      target: { value: "task.run_*, provider.auth_failed" },
    });
    fireEvent.change(screen.getByTestId("settings-page-hooks-notification-preset-target"), {
      target: { value: "bridge_slack_ops:channel:ops" },
    });
    fireEvent.click(screen.getByTestId("settings-page-hooks-notification-preset-enabled"));
    fireEvent.click(screen.getByTestId("settings-page-hooks-notification-preset-create"));

    expect(onCreate).toHaveBeenCalledWith({
      name: "custom_task",
      events: ["task.run_*", "provider.auth_failed"],
      targets: [
        {
          bridge_id: "bridge_slack_ops",
          canonical_route: "channel:ops",
          delivery_mode: "direct-send",
        },
      ],
      filter: "",
      enabled: true,
    });
  });

  it("routes toggle and delete actions for custom presets", () => {
    const onToggle = vi.fn();
    const onDelete = vi.fn();
    render(
      <NotificationPresetsPanel
        presets={[customPreset]}
        isLoading={false}
        error={null}
        pendingName={null}
        canMutate
        onCreate={vi.fn()}
        onToggle={onToggle}
        onDelete={onDelete}
      />
    );

    fireEvent.click(
      screen.getByTestId("settings-page-hooks-notification-preset-row-custom_failure-toggle")
    );
    fireEvent.click(
      screen.getByTestId("settings-page-hooks-notification-preset-row-custom_failure-delete")
    );

    expect(onToggle).toHaveBeenCalledWith(customPreset, false);
    expect(onDelete).toHaveBeenCalledWith(customPreset);
  });

  it("validates target syntax before calling create", () => {
    const onCreate = vi.fn();
    render(
      <NotificationPresetsPanel
        presets={[]}
        isLoading={false}
        error={null}
        pendingName={null}
        canMutate
        onCreate={onCreate}
        onToggle={vi.fn()}
        onDelete={vi.fn()}
      />
    );

    fireEvent.click(screen.getByTestId("settings-page-hooks-notification-preset-new"));
    fireEvent.change(screen.getByTestId("settings-page-hooks-notification-preset-name"), {
      target: { value: "custom_task" },
    });
    fireEvent.change(screen.getByTestId("settings-page-hooks-notification-preset-target"), {
      target: { value: "bridge_without_route" },
    });
    fireEvent.click(screen.getByTestId("settings-page-hooks-notification-preset-create"));

    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByTestId("settings-page-hooks-notification-presets-error")).toHaveTextContent(
      "bridge_id:canonical_route"
    );
    expect(screen.getByRole("alert")).toHaveAttribute("id", "notification-preset-create-error");
    expect(screen.getByTestId("settings-page-hooks-notification-preset-target")).toHaveAttribute(
      "aria-describedby",
      "notification-preset-create-error"
    );
    expect(screen.getByTestId("settings-page-hooks-notification-preset-target")).toHaveAttribute(
      "aria-invalid",
      "true"
    );
  });

  it("prevents another create while a notification mutation is pending", () => {
    const onCreate = vi.fn();
    render(
      <NotificationPresetsPanel
        presets={[]}
        isLoading={false}
        error={null}
        pendingName="first_request"
        canMutate
        onCreate={onCreate}
        onToggle={vi.fn()}
        onDelete={vi.fn()}
      />
    );

    expect(screen.getByTestId("settings-page-hooks-notification-preset-new")).toBeDisabled();
    expect(
      screen.queryByTestId("settings-page-hooks-notification-preset-createform")
    ).not.toBeInTheDocument();
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("disables every mutation control while one preset mutation is pending", () => {
    render(
      <NotificationPresetsPanel
        presets={[customPreset, secondCustomPreset]}
        isLoading={false}
        error={null}
        pendingName={customPreset.name}
        canMutate
        onCreate={vi.fn()}
        onToggle={vi.fn()}
        onDelete={vi.fn()}
      />
    );

    expect(screen.getByTestId("settings-page-hooks-notification-preset-new")).toBeDisabled();
    expect(
      screen.getByTestId("settings-page-hooks-notification-preset-row-custom_warning-toggle")
    ).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByTestId("settings-page-hooks-notification-preset-row-custom_warning-delete")
    ).toBeDisabled();
  });

  it("announces operational errors without invalidating the create fields", () => {
    render(
      <NotificationPresetsPanel
        presets={[]}
        isLoading={false}
        error="Failed to load notification presets."
        pendingName={null}
        canMutate
        onCreate={vi.fn()}
        onToggle={vi.fn()}
        onDelete={vi.fn()}
      />
    );

    fireEvent.click(screen.getByTestId("settings-page-hooks-notification-preset-new"));

    const nameInput = screen.getByTestId("settings-page-hooks-notification-preset-name");
    expect(nameInput).toHaveAttribute("aria-invalid", "false");
    expect(nameInput).not.toHaveAttribute("aria-describedby");
    expect(
      screen.getByTestId("settings-page-hooks-notification-presets-operational-error")
    ).toHaveTextContent("Failed to load notification presets.");
  });
});
