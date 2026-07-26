import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ErrorPage from "@/app/error";
import NotFound from "@/app/not-found";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    className,
  }: {
    href: string;
    children: React.ReactNode;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

describe("fallback pages", () => {
  it("routes missing pages back into the docs catalog", () => {
    render(<NotFound />);

    expect(screen.getByText("Not found")).toBeDefined();
    expect(
      screen.getByRole("heading", { name: "This route is not in the runtime." })
    ).toBeDefined();
    expect(screen.getByText(/not part of the published AGH site/)).toBeDefined();
    expect(screen.getByRole("link", { name: "Runtime docs" }).getAttribute("href")).toBe(
      "/runtime/"
    );
    expect(screen.getByRole("link", { name: "Network protocol" }).getAttribute("href")).toBe(
      "/protocol/"
    );
  });

  it("lets operators retry recoverable render failures without exposing raw errors", () => {
    const reset = vi.fn();
    const error = new Error("postgres://user:password@example.invalid") as Error & {
      digest?: string;
    };
    error.digest = "digest-123";

    render(<ErrorPage error={error} reset={reset} />);

    expect(screen.getByText("Render failure")).toBeDefined();
    expect(
      screen.getByRole("heading", { name: "The site hit a recoverable boundary." })
    ).toBeDefined();
    expect(screen.getByText(/Retry the boundary/)).toBeDefined();
    expect(screen.getByText("Digest digest-123")).toBeDefined();
    expect(screen.queryByText(/postgres:\/\/user:password/)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Retry boundary" }));
    expect(reset).toHaveBeenCalledTimes(1);
  });

  it("uses a stable fallback detail when the runtime does not provide a digest", () => {
    const reset = vi.fn();

    render(<ErrorPage error={new Error("render failed")} reset={reset} />);

    expect(screen.getByText("Runtime boundary failure")).toBeDefined();
    expect(screen.queryByText("render failed")).toBeNull();
  });
});
