import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LLMCopyButton } from "@/components/docs/page-actions/llm-copy-button";

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>(res => {
    resolve = res;
  });
  return { promise, resolve };
}

describe("LLMCopyButton", () => {
  beforeEach(() => {
    const clipboard = {
      writeText: vi.fn().mockResolvedValue(undefined),
    };
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: clipboard,
    });
    vi.stubGlobal("fetch", vi.fn());
  });

  it("disables the button while copying cached markdown", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValue({
      ok: true,
      text: vi.fn().mockResolvedValue("cached markdown"),
    } as unknown as Response);

    render(<LLMCopyButton markdownUrl="/runtime/cache-hit.md" />);

    const button = screen.getByRole("button", { name: "Copy as Markdown" });
    fireEvent.click(button);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith("cached markdown");

    const pendingCopy = deferred<void>();
    vi.mocked(navigator.clipboard.writeText).mockImplementationOnce(() => pendingCopy.promise);

    fireEvent.click(button);
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(true));
    pendingCopy.resolve(undefined);
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(navigator.clipboard.writeText).toHaveBeenLastCalledWith("cached markdown");
  });

  it("does not cache non-OK responses and leaves the action retryable", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock
      .mockResolvedValueOnce({
        ok: false,
        status: 500,
        statusText: "Internal Server Error",
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        text: vi.fn().mockResolvedValue("fresh markdown"),
      } as unknown as Response);

    render(<LLMCopyButton markdownUrl="/runtime/retry.md" />);

    const button = screen.getByRole("button", { name: "Copy as Markdown" });
    fireEvent.click(button);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(button.textContent).toContain("Retry copy"));
    expect(navigator.clipboard.writeText).not.toHaveBeenCalled();

    fireEvent.click(button);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith("fresh markdown");
  });
});
