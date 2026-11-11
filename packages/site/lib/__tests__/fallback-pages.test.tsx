import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ErrorPage from "@/app/error";
import NotFound from "@/app/not-found";

vi.mock("next/navigation", () => ({
  usePathname: () => "/missing",
}));

vi.mock("@/components/site/github-stars-api", () => ({
  loadGithubStars: vi.fn().mockResolvedValue(null),
}));

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
  it("routes missing pages back to home and the docs", () => {
    render(<NotFound />);

    expect(screen.getByRole("heading", { level: 1, name: "This page is not here." })).toBeDefined();
    expect(screen.getByRole("link", { name: "Back to home" }).getAttribute("href")).toBe("/");
    expect(screen.getByRole("link", { name: "Read the docs" }).getAttribute("href")).toBe("/docs");
  });

  it("lets operators retry recoverable render failures without exposing raw errors", () => {
    const retry = vi.fn();
    const error = new Error("postgres://user:password@example.invalid") as Error & {
      digest?: string;
    };
    error.digest = "digest-123";

    render(<ErrorPage error={error} retry={retry} />);

    expect(screen.getByText("Render failure")).toBeDefined();
    expect(
      screen.getByRole("heading", { name: "The site hit a recoverable boundary." })
    ).toBeDefined();
    expect(screen.getByText(/Retry the boundary/)).toBeDefined();
    expect(screen.getByText("Digest digest-123")).toBeDefined();
    expect(screen.queryByText(/postgres:\/\/user:password/)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Retry boundary" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("uses a stable fallback detail when the runtime does not provide a digest", () => {
    render(<ErrorPage error={new Error("render failed")} retry={vi.fn()} />);

    expect(screen.getByText("Runtime boundary failure")).toBeDefined();
    expect(screen.queryByText("render failed")).toBeNull();
  });
});
