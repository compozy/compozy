import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { DirectoryPickerInput, type DirectoryPickerSuggestion } from "../directory-picker-input";

const TREE: Record<string, string[]> = {
  "/home/ada/": ["code", "notes"],
  "/home/ada/code/": ["app", "api"],
};

function suggestionsFor(value: string): DirectoryPickerSuggestion[] {
  const cut = value.lastIndexOf("/") + 1;
  const dir = value.slice(0, cut);
  const prefix = value.slice(cut);
  return (TREE[dir] ?? [])
    .filter(name => name.startsWith(prefix))
    .map(name => ({ value: `${dir}${name}/`, label: name }));
}

function Harness({ onBrowse }: { onBrowse?: () => void }) {
  const [value, setValue] = useState("/home/ada/");
  return (
    <DirectoryPickerInput
      aria-label="Project folder"
      onBrowse={onBrowse}
      onValueChange={setValue}
      suggestions={suggestionsFor(value)}
      value={value}
    />
  );
}

describe("DirectoryPickerInput", () => {
  it("Should complete the highlighted folder with Tab and keep listing its children", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const input = screen.getByRole("combobox", { name: "Project folder" });

    await user.click(input);
    await user.type(input, "c");
    expect(await screen.findByRole("option", { name: "code" })).toBeInTheDocument();

    await user.keyboard("{Tab}");
    expect(input).toHaveValue("/home/ada/code/");
    expect(input).toHaveFocus();
    expect(await screen.findByRole("option", { name: "app" })).toBeInTheDocument();

    await user.keyboard("{ArrowDown}{ArrowDown}{Enter}");
    expect(input).toHaveValue("/home/ada/code/api/");
  });

  it("Should close the list on Escape without changing the value", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const input = screen.getByRole("combobox", { name: "Project folder" });

    await user.click(input);
    await user.type(input, "n");
    expect(await screen.findByRole("option", { name: "notes" })).toBeInTheDocument();
    await user.keyboard("{Escape}");

    await waitFor(() => expect(screen.queryByRole("option")).not.toBeInTheDocument());
    expect(input).toHaveValue("/home/ada/n");
  });

  it("Should offer Browse only when a native chooser is provided", async () => {
    const user = userEvent.setup();
    const onBrowse = vi.fn();
    const { rerender } = render(<Harness />);
    expect(screen.queryByRole("button", { name: "Browse…" })).not.toBeInTheDocument();

    rerender(<Harness onBrowse={onBrowse} />);
    await user.click(screen.getByRole("button", { name: "Browse…" }));
    expect(onBrowse).toHaveBeenCalledTimes(1);
  });

  it("Should mark the field invalid for the caller's validation message", () => {
    render(
      <DirectoryPickerInput
        aria-label="Project folder"
        invalid
        onValueChange={() => {}}
        suggestions={[]}
        value="/missing"
      />
    );
    expect(screen.getByRole("combobox", { name: "Project folder" })).toHaveAttribute(
      "aria-invalid",
      "true"
    );
  });
});
