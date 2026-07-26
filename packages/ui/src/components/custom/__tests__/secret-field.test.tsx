import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";

import { DIALOG_TOUCH_TARGET_CLASS } from "../../../lib/dialog-shell";
import { SecretField, type SecretFieldProps } from "../secret-field";

/**
 * Controlled harness — the component is fully controlled, so the value and
 * editing flags must round-trip through state for the rotate/cancel cycle to be
 * observable.
 */
function SecretFieldHarness({
  onValueChangeSpy,
  ...props
}: Partial<SecretFieldProps> & { onValueChangeSpy?: (next: string) => void }) {
  const [value, setValue] = React.useState("");
  const [editing, setEditing] = React.useState(false);
  return (
    <SecretField
      editing={editing}
      id="api-key"
      label="API key"
      onEditingChange={setEditing}
      onValueChange={next => {
        setValue(next);
        onValueChangeSpy?.(next);
      }}
      testIdPrefix="secret"
      value={value}
      {...props}
    />
  );
}

describe("SecretField", () => {
  it("Should render a write-only input on the create path", async () => {
    const user = userEvent.setup();
    render(<SecretFieldHarness />);

    const root = screen.getByTestId("secret");
    expect(root).toHaveAttribute("data-state", "absent");

    const input = screen.getByLabelText("API key");
    expect(input).toHaveAttribute("type", "password");
    expect(input).toHaveValue("");

    await user.type(input, "sk-live");
    expect(screen.getByLabelText("API key")).toHaveValue("sk-live");
    expect(screen.queryByTestId("secret-presence")).not.toBeInTheDocument();
  });

  it("Should cycle present to editing and back to present on cancel", async () => {
    const user = userEvent.setup();
    const onValueChangeSpy = vi.fn();
    render(
      <SecretFieldHarness
        onValueChangeSpy={onValueChangeSpy}
        present
        presenceLabel="vault:mcp/github"
      />
    );

    expect(screen.getByTestId("secret")).toHaveAttribute("data-state", "present");
    expect(screen.getByTestId("secret-presence")).toHaveTextContent("vault:mcp/github");
    expect(screen.queryByLabelText("API key")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("secret-replace"));
    expect(screen.getByTestId("secret")).toHaveAttribute("data-state", "editing");
    await user.type(screen.getByLabelText("API key"), "rotated");

    await user.click(screen.getByTestId("secret-cancel"));
    expect(screen.getByTestId("secret")).toHaveAttribute("data-state", "present");
    expect(screen.getByTestId("secret-presence")).toHaveTextContent("vault:mcp/github");
    // Cancelling clears the draft so the existing binding is preserved untouched.
    expect(onValueChangeSpy).toHaveBeenLastCalledWith("");
  });

  it("Should never expose a stored value as text", () => {
    render(<SecretFieldHarness present presenceLabel="vault:mcp/github" />);

    const presence = screen.getByTestId("secret-presence");
    expect(presence).toHaveTextContent("••••••••");
    expect(presence.textContent).not.toContain("sk-");
  });

  it("Should report the invalid, saving, and rotated states", () => {
    const { rerender } = render(<SecretFieldHarness error="Value is required." />);
    expect(screen.getByTestId("secret")).toHaveAttribute("data-state", "invalid");
    expect(screen.getByText("Value is required.")).toBeInTheDocument();

    rerender(<SecretFieldHarness saving />);
    expect(screen.getByTestId("secret")).toHaveAttribute("data-state", "saving");
    expect(screen.getByLabelText("API key")).toBeDisabled();

    rerender(<SecretFieldHarness present rotated />);
    expect(screen.getByTestId("secret")).toHaveAttribute("data-state", "rotated");
    expect(screen.getByText("rotated")).toBeInTheDocument();
  });

  it("Should associate description and error through aria-describedby", () => {
    const { rerender } = render(
      <SecretFieldHarness description="Required by this server." required />
    );

    const input = screen.getByLabelText("API key");
    expect(input).toHaveAttribute("aria-describedby", "api-key-description");
    expect(input).not.toHaveAttribute("aria-invalid");

    rerender(
      <SecretFieldHarness
        description="Required by this server."
        error="Choose a present ref or enter a value."
        required
      />
    );
    const invalidInput = screen.getByLabelText("API key");
    expect(invalidInput).toHaveAttribute("aria-describedby", "api-key-description api-key-error");
    expect(invalidInput).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("Choose a present ref or enter a value.")).toHaveAttribute(
      "id",
      "api-key-error"
    );
  });

  it("Should announce a stored-reference lookup failure", () => {
    render(
      <SecretFieldHarness
        binding={{
          error: "Stored references could not be loaded.",
          onSelectRef: vi.fn(),
          selectedRef: "",
          sources: [],
        }}
        mode="source"
        onModeChange={vi.fn()}
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Stored references could not be loaded.");
  });

  it("Should bind an existing reference instead of a typed value", async () => {
    const user = userEvent.setup();
    const onModeChange = vi.fn();
    const onSelectRef = vi.fn();
    render(
      <SecretFieldHarness
        binding={{
          namespaceLabel: "namespace=mcp",
          onSelectRef,
          selectedRef: "",
          sources: [
            { present: true, ref: "vault:mcp/github" },
            { present: false, ref: "vault:mcp/stale" },
          ],
        }}
        mode="source"
        onModeChange={onModeChange}
        sourceModeLabel="Use Vault"
      />
    );

    expect(screen.getByTestId("secret-sources")).toBeInTheDocument();
    expect(screen.queryByLabelText("API key")).not.toBeInTheDocument();

    const options = screen.getAllByRole("radio");
    expect(options).toHaveLength(2);
    expect(options[1]).toBeDisabled();

    await user.click(options[0]);
    expect(onSelectRef).toHaveBeenCalledWith("vault:mcp/github");

    await user.click(screen.getByTestId("secret-mode-value"));
    expect(onModeChange).toHaveBeenCalledWith("value");
  });

  it("Should move selection and focus through stored references with radio keys", async () => {
    const user = userEvent.setup();
    const onSelectRef = vi.fn();
    render(
      <SecretFieldHarness
        binding={{
          onSelectRef,
          selectedRef: "vault:mcp/github",
          sources: [
            { present: true, ref: "vault:mcp/github" },
            { present: false, ref: "vault:mcp/stale" },
            { present: true, ref: "vault:mcp/gitlab" },
          ],
        }}
        mode="source"
        onModeChange={vi.fn()}
      />
    );

    const [github, stale, gitlab] = screen.getAllByRole("radio");
    github.focus();
    await user.keyboard("{ArrowDown}");
    expect(onSelectRef).toHaveBeenLastCalledWith("vault:mcp/gitlab");
    expect(gitlab).toHaveFocus();

    await user.keyboard("{Home}");
    expect(onSelectRef).toHaveBeenLastCalledWith("vault:mcp/github");
    expect(github).toHaveFocus();

    await user.keyboard("{End}");
    expect(onSelectRef).toHaveBeenLastCalledWith("vault:mcp/gitlab");
    expect(gitlab).toHaveFocus();
    expect(stale).toBeDisabled();
  });

  it("Should lock the binding method while an inline secret create is pending", async () => {
    const user = userEvent.setup();
    const onModeChange = vi.fn();
    render(
      <SecretFieldHarness
        binding={{
          create: {
            onOpenChange: vi.fn(),
            onSubmit: vi.fn(),
            onValueChange: vi.fn(),
            open: true,
            pending: true,
            ref: "vault:mcp/new-key",
            value: "pending",
          },
          onSelectRef: vi.fn(),
          selectedRef: "",
          sources: [],
        }}
        mode="source"
        onModeChange={onModeChange}
      />
    );

    const enterValue = screen.getByTestId("secret-mode-value");
    expect(enterValue).toBeDisabled();
    await user.click(enterValue);
    expect(onModeChange).not.toHaveBeenCalled();
  });

  it("Should mark the selected source neutrally, never with accent", () => {
    render(
      <SecretFieldHarness
        binding={{
          onSelectRef: vi.fn(),
          selectedRef: "vault:mcp/github",
          sources: [
            { present: true, ref: "vault:mcp/github" },
            { present: true, ref: "vault:mcp/other" },
          ],
        }}
        mode="source"
        onModeChange={vi.fn()}
      />
    );

    const [selected, unselected] = screen.getAllByRole("radio");
    expect(selected).toHaveAttribute("aria-checked", "true");
    expect(unselected).toHaveAttribute("aria-checked", "false");
    // Accent is reserved for true CTAs; selection reads as glaze + rim.
    expect(selected.className).toContain("data-[selected]:bg-row-selected");
    expect(selected.className).toContain("data-[selected]:border-line-strong");
    expect(selected.className).not.toMatch(/accent/);
    expect(selected).toHaveClass(DIALOG_TOUCH_TARGET_CLASS);
  });
});
