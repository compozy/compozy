// Suite: OS About dialog
// Invariant: only the open dialog reads full daemon status; pending and resolved rows stay truthful.
// Boundary IN: OsAboutDialog over real status query hooks.
// Boundary OUT: the status HTTP adapter.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { statusFixture } from "@/systems/status/mocks/fixtures";
import { fetchStatus } from "@/systems/status/adapters/daemon-api";
import { OsAboutDialog } from "../os-about-dialog";

vi.mock("@/systems/status/adapters/daemon-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/status/adapters/daemon-api")>()),
  fetchStatus: vi.fn(),
}));

const clients: QueryClient[] = [];

function renderDialog(open = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  return render(<OsAboutDialog open={open} onOpenChange={vi.fn()} />, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  });
}

describe("OsAboutDialog", () => {
  beforeEach(() => {
    vi.mocked(fetchStatus).mockReset();
    vi.mocked(fetchStatus).mockReturnValue(new Promise(() => undefined));
  });

  afterEach(() => {
    for (const client of clients.splice(0)) client.clear();
    vi.useRealTimers();
  });

  it("Should hold the identity rows in one busy region while the daemon status is read", () => {
    renderDialog();
    expect(screen.getByRole("status")).toHaveTextContent("Reading CompozyOS status");
    expect(screen.getByText("Version")).toBeInTheDocument();
    expect(screen.queryByTestId("os-about-row-version")).not.toBeInTheDocument();
  });

  it("Should swap the placeholder for the identity rows once the status answers", async () => {
    let resolve: ((value: typeof statusFixture) => void) | undefined;
    vi.mocked(fetchStatus).mockReturnValue(
      new Promise(done => {
        resolve = done;
      })
    );
    renderDialog();
    expect(screen.getByRole("status")).toHaveTextContent("Reading CompozyOS status");
    await act(async () => {
      resolve?.({ ...statusFixture, daemon: { ...statusFixture.daemon, version: "0.4.0" } });
    });
    await waitFor(() => expect(screen.queryByRole("status")).not.toBeInTheDocument());
    expect(screen.getByTestId("os-about-row-version")).toHaveTextContent("0.4.0");
  });

  it("Should read and poll full status only while About is open", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchStatus).mockResolvedValue(statusFixture);
    const { rerender, unmount } = renderDialog(false);
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    expect(fetchStatus).not.toHaveBeenCalled();
    await act(async () => {
      rerender(<OsAboutDialog open onOpenChange={vi.fn()} />);
    });
    expect(fetchStatus).toHaveBeenCalledTimes(1);
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(fetchStatus).toHaveBeenCalledTimes(2);
    await act(async () => {
      rerender(<OsAboutDialog open={false} onOpenChange={vi.fn()} />);
    });
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    expect(fetchStatus).toHaveBeenCalledTimes(2);
    unmount();
  });
});
