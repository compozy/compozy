import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";

import { MarkdownEditor, type MarkdownEditorProps } from "../markdown-editor";

function Harness(props: Partial<MarkdownEditorProps> & { initial?: string }) {
  const [value, setValue] = React.useState(props.initial ?? "");
  return (
    <MarkdownEditor
      aria-label="Note"
      {...props}
      onValueChange={next => {
        setValue(next);
        props.onValueChange?.(next);
      }}
      value={value}
    />
  );
}

describe("MarkdownEditor", () => {
  it("Should render Markdown as formatted text", async () => {
    render(<Harness initial={"Waiting on **review**\n\n- one\n- two"} />);
    const box = await screen.findByRole("textbox", { name: "Note" });
    expect(box.querySelector("strong")).toHaveTextContent("review");
    expect(box.querySelectorAll("li")).toHaveLength(2);
  });

  it("Should write a toolbar mark back as Markdown", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Harness initial="ship it" onValueChange={onValueChange} />);
    const box = await screen.findByRole("textbox", { name: "Note" });
    await user.click(box);
    await user.keyboard("{Control>}a{/Control}");
    await user.click(screen.getByRole("button", { name: "Bold" }));
    await waitFor(() => expect(onValueChange).toHaveBeenLastCalledWith("**ship it**"));
    expect(screen.getByRole("button", { name: "Bold" })).toHaveAttribute("aria-pressed", "true");
    await user.click(screen.getByRole("button", { name: "Bulleted list" }));
    await waitFor(() => expect(onValueChange).toHaveBeenLastCalledWith("- **ship it**"));
  });

  it("Should submit on Mod+Enter", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<Harness initial="note" onSubmit={onSubmit} />);
    await user.click(await screen.findByRole("textbox", { name: "Note" }));
    // jsdom reports a non-Apple platform, so Mod is Control.
    await user.keyboard("{Control>}{Enter}{/Control}");
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("Should keep the current link when the typed address is refused", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Harness initial="[spec](https://compozy.com)" onValueChange={onValueChange} />);
    const box = await screen.findByRole("textbox", { name: "Note" });
    await user.click(box);
    await user.keyboard("{Control>}a{/Control}");
    await user.click(screen.getByRole("button", { name: "Link" }));
    const address = await screen.findByRole("textbox", { name: "Link address" });
    await user.clear(address);
    await user.type(address, "ftp://example.com");
    expect(address).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled();
    await user.keyboard("{Enter}");
    expect(onValueChange).not.toHaveBeenCalled();
    expect(box.querySelector("a")).toHaveAttribute("href", "https://compozy.com");
  });

  it("Should take an outside reset of the value", async () => {
    function Resettable() {
      const [value, setValue] = React.useState("draft");
      return (
        <>
          <MarkdownEditor aria-label="Note" onValueChange={setValue} value={value} />
          <button onClick={() => setValue("")} type="button">
            Reset
          </button>
        </>
      );
    }
    const user = userEvent.setup();
    render(<Resettable />);
    const box = await screen.findByRole("textbox", { name: "Note" });
    expect(box).toHaveTextContent("draft");
    await user.click(screen.getByRole("button", { name: "Reset" }));
    expect(box).toHaveTextContent("");
  });
});
