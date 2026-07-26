// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Composer } from "../composer";

describe("Composer", () => {
  it("Should disable the Send button until the textarea has non-whitespace content", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(
      <Composer
        onSubmit={onSubmit}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="thread"
      />
    );
    const send = screen.getByTestId("network-composer-send-thread");
    expect(send).toBeDisabled();
    await user.type(screen.getByTestId("network-composer-textarea-thread"), "Hello");
    expect(send).not.toBeDisabled();
  });

  it("Should reset the textarea on submit", async () => {
    const onSubmit = vi.fn().mockImplementation(({ reset }: { reset: () => void }) => reset());
    const user = userEvent.setup();
    render(
      <Composer
        onSubmit={onSubmit}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="thread"
      />
    );
    const textarea = screen.getByTestId("network-composer-textarea-thread") as HTMLTextAreaElement;
    await user.type(textarea, "Hello world");
    await user.click(screen.getByTestId("network-composer-send-thread"));
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit.mock.calls[0]?.[0]?.text).toBe("Hello world");
    expect(textarea.value).toBe("");
  });

  it("Should surface mention chips and include mentions in submit args", async () => {
    const onSubmit = vi.fn().mockImplementation(({ reset }: { reset: () => void }) => reset());
    const user = userEvent.setup();
    render(
      <Composer
        onSubmit={onSubmit}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="thread"
      />
    );

    await user.type(
      screen.getByTestId("network-composer-textarea-thread"),
      "@peer.alpha @peer.beta coordinate status"
    );

    expect(screen.getByTestId("network-composer-mention-thread-peer.alpha")).toHaveTextContent(
      "peer.alpha"
    );
    expect(screen.getByTestId("network-composer-mention-thread-peer.beta")).toHaveTextContent(
      "peer.beta"
    );
    expect(screen.getByRole("group", { name: "Mention recipients" })).toBeInTheDocument();

    await user.click(screen.getByTestId("network-composer-send-thread"));
    expect(onSubmit.mock.calls[0]?.[0]?.mentions).toEqual(["peer.alpha", "peer.beta"]);
  });

  it("Should open the slash popover when the user types `/`", async () => {
    const user = userEvent.setup();
    render(
      <Composer
        onSubmit={vi.fn()}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="thread"
      />
    );
    await user.type(screen.getByTestId("network-composer-textarea-thread"), "/");
    expect(screen.getByTestId("network-composer-slash-popover")).toBeInTheDocument();
    expect(screen.getByTestId("network-composer-slash-option-run")).toBeInTheDocument();
    expect(screen.getByTestId("network-composer-slash-option-mention")).toBeInTheDocument();
    const attach = screen.getByTestId("network-composer-slash-option-attach");
    expect(attach).toHaveAttribute("data-disabled", "true");
    expect(attach).toHaveAttribute("title", "Post-MVP");
  });

  it("Should respect disabled state and not invoke onSubmit", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(
      <Composer
        disabled
        disabledReason="Network is off."
        onSubmit={onSubmit}
        placeholder="Reply..."
        sendLabel="Send to #ops"
        testIdSuffix="thread"
      />
    );
    const textarea = screen.getByTestId("network-composer-textarea-thread") as HTMLTextAreaElement;
    expect(textarea).toBeDisabled();
    expect(textarea.placeholder).toBe("Network is off.");
    await user.click(screen.getByTestId("network-composer-send-thread"));
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
